package invite

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
)

type (
	Service interface {
		List(ctx context.Context, orgID uuid.UUID) (*dto.OrganisationInviteListResponse, error)
		Create(ctx context.Context, creation spec.InviteCreation) (*dto.CreatedInviteResponse, error)
		Resend(ctx context.Context, action spec.InviteAction) (*dto.OrganisationInviteResponse, error)
		Revoke(ctx context.Context, action spec.InviteAction) error
		Lookup(ctx context.Context, token string) (*dto.InviteDetailsResponse, error)
		Accept(ctx context.Context, join spec.InviteJoin) (*dto.JoinedOrganisationResponse, error)
		Register(ctx context.Context, registration spec.InviteRegistration) (*dto.InviteRegistrationResponse, error)
		DeleteExpired(ctx context.Context) (int64, error)
	}

	Deps struct {
		Organisations repository.OrganisationRepository
		Teams         repository.TeamRepository
		Users         repository.UserRepository
		AuditLog      repository.AuditLogRepository
		Accounts      user.Service
		OrgAuthz      orgauthz.Service
		Entitlements  entitlements.Service
		Settings      settings.Service
		Email         email.Service
	}

	service struct {
		orgs         repository.OrganisationRepository
		teams        repository.TeamRepository
		users        repository.UserRepository
		auditLog     repository.AuditLogRepository
		accounts     user.Service
		orgAuthz     orgauthz.Service
		entitlements entitlements.Service
		settings     settings.Service
		email        email.Service
	}

	site struct {
		name    string
		baseURL string
	}

	memberQuota struct {
		plan    *entitlements.Plan
		allowed int
	}
)

const (
	inviteTTL       = 7 * 24 * time.Hour
	verificationTTL = 24 * time.Hour
	emailInviteUses = 1
	maxEmailLength  = 254

	invitePage      = "invite"
	verifyEmailPage = "verify-email"
)

func NewService(deps Deps) Service {
	return &service{
		orgs:         deps.Organisations,
		teams:        deps.Teams,
		users:        deps.Users,
		auditLog:     deps.AuditLog,
		accounts:     deps.Accounts,
		orgAuthz:     deps.OrgAuthz,
		entitlements: deps.Entitlements,
		settings:     deps.Settings,
		email:        deps.Email,
	}
}

func (s *service) liveOrganisation(ctx context.Context, orgID uuid.UUID) (*model.Organisation, error) {
	found, err := s.orgs.GetByID(ctx, orgID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("look up organisation %s: %w", orgID, err)
	}

	if found.Deleted() {
		return nil, ErrOrganisationNotFound
	}

	return found, nil
}

func (s *service) liveInvite(ctx context.Context, presented string) (*model.InviteDetails, error) {
	if presented == "" {
		return nil, ErrInvalidInvite
	}

	details, err := s.orgs.GetLiveInviteByTokenHash(ctx, token.Hash(presented))
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrInvalidInvite
	}
	if err != nil {
		return nil, fmt.Errorf("look up the invite: %w", err)
	}

	return details, nil
}

func (s *service) memberQuota(ctx context.Context, orgID uuid.UUID) (*memberQuota, error) {
	members, err := s.orgs.CountMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("count the members of organisation %s: %w", orgID, err)
	}

	if err := s.entitlements.CheckWithin(ctx, spec.LimitCheck{OrgID: orgID, Limit: model.LimitMembers, Current: members}); err != nil {
		return nil, fmt.Errorf("check the member allowance of organisation %s: %w", orgID, err)
	}

	plan, err := s.entitlements.For(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("read the plan of organisation %s: %w", orgID, err)
	}

	allowed, known := plan.Allowance(model.LimitMembers)
	if !known {
		return nil, fmt.Errorf("%w: %q on the %s plan", entitlements.ErrUnknownLimit, model.LimitMembers, plan.Code)
	}

	return &memberQuota{plan: plan, allowed: allowed}, nil
}

func (q *memberQuota) refuseJoin(err error) error {
	if errors.Is(err, dao.ErrQuotaExceeded) {
		return &entitlements.DeniedError{Plan: q.plan.Code, Limit: model.LimitMembers, Allowed: q.allowed, Current: q.allowed}
	}

	if errors.Is(err, dao.ErrConflict) {
		return ErrAlreadyMember
	}

	if errors.Is(err, dao.ErrNotFound) {
		return ErrInvalidInvite
	}

	return fmt.Errorf("accept the invite: %w", err)
}

func (s *service) recordJoin(ctx context.Context, accepted model.AcceptedInvite, ip string) error {
	details := map[string]string{"role": string(accepted.Member.Role)}
	if accepted.TeamMember != nil {
		details["team_id"] = accepted.TeamMember.TeamID.String()
	}

	entry := audit.NewEntry{
		OrgID:      accepted.Member.OrgID,
		ActorID:    accepted.Member.UserID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionMemberJoined,
		TargetType: audit.TargetInvite,
		TargetID:   accepted.Invite.ID.String(),
		SubjectID:  accepted.Member.UserID,
		Details:    details,
		IP:         ip,
	}

	return s.record(ctx, entry)
}

func (s *service) record(ctx context.Context, entry audit.NewEntry) error {
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return nil
}

func (s *service) readSite(ctx context.Context) (site, error) {
	baseURL, err := s.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return site{}, fmt.Errorf("read the base url: %w", err)
	}

	name, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return site{}, fmt.Errorf("read the site name: %w", err)
	}

	return site{name: name, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s site) tokenLink(page string, drawn string) string {
	return s.baseURL + "/" + page + "#" + drawn
}

func staleCache(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w: %w", ErrCacheNotCleared, err)
}

func normaliseEmail(raw string) (string, error) {
	address := strings.TrimSpace(raw)
	if address == "" || len(address) > maxEmailLength {
		return "", ErrInvalidEmail
	}

	parsed, err := mail.ParseAddress(address)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidEmail, err)
	}

	if parsed.Address != address {
		return "", ErrInvalidEmail
	}

	return address, nil
}
