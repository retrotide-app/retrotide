package admin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/entitlements"
	"retrotide/internal/hierarchy"
	"retrotide/internal/identity"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/user"

	"github.com/google/uuid"
)

type (
	Service interface {
		ListUsers(ctx context.Context, listing spec.UserListing) (*dto.AdminUserListResponse, error)
		GetUser(ctx context.Context, userID uuid.UUID) (*dto.AdminUserDetailResponse, error)
		GetUserAuditLog(ctx context.Context, listing spec.AuditLogSubjectListing) (*dto.AuditLogListResponse, error)
		SetUserRole(ctx context.Context, change spec.StaffRoleChange) error
		RemoveUserRole(ctx context.Context, change spec.StaffRoleChange) error
		BanUser(ctx context.Context, restriction spec.StaffUserRestriction) error
		UnbanUser(ctx context.Context, action spec.StaffUserAction) error
		LockUser(ctx context.Context, restriction spec.StaffUserRestriction) error
		UnlockUser(ctx context.Context, action spec.StaffUserAction) error
		ForceLogout(ctx context.Context, action spec.StaffUserAction) error
		DeleteUser(ctx context.Context, action spec.StaffUserAction) error
		ResetUserPassword(ctx context.Context, action spec.StaffUserAction) (*dto.AdminPasswordResetResponse, error)
		SetUserEmail(ctx context.Context, change spec.StaffEmailChange) error
		VerifyUserEmail(ctx context.Context, action spec.StaffUserAction) error
		UnverifyUserEmail(ctx context.Context, action spec.StaffUserAction) error
		SetUserDisplayName(ctx context.Context, change spec.StaffDisplayNameChange) error

		ListInvitations(ctx context.Context, listing spec.InvitationListing) (*dto.AdminInvitationListResponse, error)
		InviteUser(ctx context.Context, invitation spec.StaffInvitation) (*dto.AdminInvitationResponse, error)
		ResendInvitation(ctx context.Context, action spec.StaffInvitationAction) error
		CancelInvitation(ctx context.Context, action spec.StaffInvitationAction) error

		GetSettings(ctx context.Context) (*dto.AdminSettingsResponse, error)
		UpdateSettings(ctx context.Context, update spec.StaffSettingsUpdate) error
		SendTestEmail(ctx context.Context, action spec.StaffAction) error

		GetPermissions(ctx context.Context) (*dto.AdminPermissionsResponse, error)
		UpdateRolePermissions(ctx context.Context, update spec.StaffRolePermissionsUpdate) error

		GetAuditLog(ctx context.Context, listing spec.AuditLogListing) (*dto.AuditLogListResponse, error)

		GetStats(ctx context.Context, viewerID uuid.UUID) (*dto.AdminStatsResponse, error)

		ListOrganisations(ctx context.Context, listing spec.OrganisationListing) (*dto.AdminOrganisationListResponse, error)
		GetOrganisation(ctx context.Context, orgID uuid.UUID) (*dto.AdminOrganisationDetailResponse, error)
	}

	Deps struct {
		Users           repository.UserRepository
		Roles           repository.RoleRepository
		Permissions     repository.PermissionRepository
		AuditLog        repository.AuditLogRepository
		Verifications   repository.EmailVerificationRepository
		Invitations     repository.InvitationRepository
		Organisations   repository.OrganisationRepository
		Teams           repository.TeamRepository
		Stats           repository.StatsRepository
		Authz           authz.Service
		Hierarchy       hierarchy.Service
		Accounts        user.Service
		Settings        settings.Service
		Sessions        *session.Manager
		Email           email.Service
		Entitlements    entitlements.Service
		SignInProviders []identity.Provider
	}

	service struct {
		users           repository.UserRepository
		roles           repository.RoleRepository
		permissions     repository.PermissionRepository
		auditLog        repository.AuditLogRepository
		verifications   repository.EmailVerificationRepository
		invitations     repository.InvitationRepository
		organisations   repository.OrganisationRepository
		teams           repository.TeamRepository
		stats           repository.StatsRepository
		authz           authz.Service
		hierarchy       hierarchy.Service
		accounts        user.Service
		settings        settings.Service
		sessions        *session.Manager
		email           email.Service
		entitlements    entitlements.Service
		signInProviders []identity.Provider
		now             func() time.Time
	}

	site struct {
		name    string
		baseURL string
	}
)

const (
	defaultUserPageSize         = 20
	defaultOrganisationPageSize = 20
	defaultAuditPageSize        = 50
	maxPageSize                 = 100
)

func NewService(deps Deps) Service {
	return &service{
		users:           deps.Users,
		roles:           deps.Roles,
		permissions:     deps.Permissions,
		auditLog:        deps.AuditLog,
		verifications:   deps.Verifications,
		invitations:     deps.Invitations,
		organisations:   deps.Organisations,
		teams:           deps.Teams,
		stats:           deps.Stats,
		authz:           deps.Authz,
		hierarchy:       deps.Hierarchy,
		accounts:        deps.Accounts,
		settings:        deps.Settings,
		sessions:        deps.Sessions,
		email:           deps.Email,
		entitlements:    deps.Entitlements,
		signInProviders: deps.SignInProviders,
		now:             time.Now,
	}
}

func (s *service) authorise(ctx context.Context, actorID uuid.UUID, targetID uuid.UUID) (*hierarchy.Standing, error) {
	standing, err := s.hierarchy.ActOnUser(ctx, spec.UserActionCheck{ActorID: actorID, TargetID: targetID})
	if errors.Is(err, hierarchy.ErrOutranked) {
		return nil, fmt.Errorf("%w: %w", ErrProtectedUser, err)
	}
	if err != nil {
		return nil, err
	}

	return standing, nil
}

func (s *service) lookUp(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	found, err := s.users.GetByID(ctx, userID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("look up user %s: %w", userID, err)
	}

	return found, nil
}

func (s *service) record(ctx context.Context, entry audit.NewEntry) error {
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return nil
}

func (s *service) recordStaffChange(ctx context.Context, writeErr error, entry audit.NewEntry) error {
	auditErr := s.record(ctx, entry)

	if writeErr != nil {
		return fmt.Errorf("%w: %w", ErrStaffCacheStale, errors.Join(writeErr, auditErr))
	}

	return auditErr
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

func (s site) pageLink(page string) string {
	return s.baseURL + "/" + page
}

func (s site) tokenLink(page string, token string) string {
	return s.baseURL + "/" + page + "#" + token
}

func staffEntry(actor spec.StaffAction, action audit.Action, targetType audit.TargetType, targetID string, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		ActorID:    actor.ActorID,
		ActorKind:  audit.ActorStaff,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
		IP:         actor.IP,
	}
}

func userEntry(actor spec.StaffAction, action audit.Action, userID uuid.UUID, details map[string]string) audit.NewEntry {
	entry := staffEntry(actor, action, audit.TargetUser, userID.String(), details)
	entry.SubjectID = userID

	return entry
}

func clampPage(limit int, offset int, fallback int) (int, int) {
	if limit <= 0 {
		limit = fallback
	}

	return min(limit, maxPageSize), min(max(offset, 0), math.MaxInt32)
}
