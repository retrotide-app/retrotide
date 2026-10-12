package invite

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/notification"
	"retrotide/internal/orgauthz"
	"retrotide/internal/token"

	"github.com/google/uuid"
)

type (
	inviteLetter struct {
		organisation *model.Organisation
		inviterID    uuid.UUID
		team         *model.Team
		address      string
		role         model.OrgRole
		link         string
	}
)

func (s *service) List(ctx context.Context, orgID uuid.UUID) (*dto.OrganisationInviteListResponse, error) {
	if _, err := s.liveOrganisation(ctx, orgID); err != nil {
		return nil, err
	}

	rows, err := s.orgs.ListInvites(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list the invites of organisation %s: %w", orgID, err)
	}

	now := time.Now()
	listed := make([]dto.OrganisationInviteResponse, 0, len(rows))
	for _, row := range rows {
		listed = append(listed, dto.NewOrganisationInviteResponse(row, now))
	}

	return &dto.OrganisationInviteListResponse{Invites: listed}, nil
}

func (s *service) Create(ctx context.Context, creation spec.InviteCreation) (*dto.CreatedInviteResponse, error) {
	pending, err := draftInvite(creation)
	if err != nil {
		return nil, err
	}

	organisation, err := s.liveOrganisation(ctx, pending.OrgID)
	if err != nil {
		return nil, err
	}

	if err := s.requireAdminGrant(ctx, pending.OrgID, creation.ActorID, pending.Role, ErrAdminInviteForbidden); err != nil {
		return nil, err
	}

	team, err := s.inviteTeam(ctx, pending.OrgID, pending.TeamID)
	if err != nil {
		return nil, err
	}

	if pending.Email != nil {
		if err := s.requireInvitable(ctx, pending.OrgID, *pending.Email); err != nil {
			return nil, err
		}

		if err := s.requireNoPending(ctx, pending.OrgID, *pending.Email); err != nil {
			return nil, err
		}
	}

	current, err := s.readSite(ctx)
	if err != nil {
		return nil, err
	}

	drawn, err := token.New()
	if err != nil {
		return nil, err
	}

	link := current.tokenLink(invitePage, drawn)

	var letter email.Message
	if pending.Email != nil {
		letter, err = s.composeLetter(ctx, current, inviteLetter{organisation: organisation, inviterID: creation.ActorID, team: team, address: *pending.Email, role: pending.Role, link: link})
		if err != nil {
			return nil, err
		}
	}

	pending.TokenHash = token.Hash(drawn)
	pending.ExpiresAt = time.Now().Add(inviteTTL)

	created, err := s.orgs.CreateInvite(ctx, pending)
	if errors.Is(err, dao.ErrNotFound) && pending.TeamID != nil {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create the invite: %w", err)
	}

	response := &dto.CreatedInviteResponse{OrganisationInviteResponse: dto.NewOrganisationInviteResponse(*created, time.Now())}

	auditErr := s.record(ctx, inviteEntry(creation.ActorID, creation.IP, audit.ActionMemberInvited, *created))

	if created.Email == nil {
		response.URL = link

		return response, auditErr
	}

	if err := s.email.Send(ctx, letter); err != nil {
		return response, fmt.Errorf("%w: %w", ErrInviteNotSent, errors.Join(fmt.Errorf("send the invite email: %w", err), auditErr))
	}

	return response, auditErr
}

func (s *service) Resend(ctx context.Context, action spec.InviteAction) (*dto.OrganisationInviteResponse, error) {
	organisation, err := s.liveOrganisation(ctx, action.OrgID)
	if err != nil {
		return nil, err
	}

	key := spec.OrganisationInviteKey{OrgID: action.OrgID, InviteID: action.InviteID}

	previous, err := s.orgs.GetInvite(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("look up invite %s: %w", action.InviteID, err)
	}

	if previous.RevokedAt != nil || usedUp(*previous) {
		return nil, ErrInviteNotFound
	}

	if previous.Email == nil {
		return nil, ErrNotResendable
	}

	if err := s.requireAdminGrant(ctx, action.OrgID, action.ActorID, previous.Role, ErrAdminInviteForbidden); err != nil {
		return nil, err
	}

	team, err := s.inviteTeam(ctx, action.OrgID, previous.TeamID)
	if err != nil {
		return nil, err
	}

	if err := s.requireInvitable(ctx, action.OrgID, *previous.Email); err != nil {
		return nil, err
	}

	current, err := s.readSite(ctx)
	if err != nil {
		return nil, err
	}

	drawn, err := token.New()
	if err != nil {
		return nil, err
	}

	letter, err := s.composeLetter(ctx, current, inviteLetter{organisation: organisation, inviterID: action.ActorID, team: team, address: *previous.Email, role: previous.Role, link: current.tokenLink(invitePage, drawn)})
	if err != nil {
		return nil, err
	}

	_, err = s.orgs.RevokeInvite(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("revoke invite %s before resending it: %w", action.InviteID, err)
	}

	replacement := spec.NewOrganisationInvite{
		OrgID:     action.OrgID,
		TeamID:    previous.TeamID,
		TokenHash: token.Hash(drawn),
		Email:     previous.Email,
		Role:      previous.Role,
		MaxUses:   new(emailInviteUses),
		InvitedBy: action.ActorID,
		ExpiresAt: time.Now().Add(inviteTTL),
	}

	renewed, err := s.orgs.CreateInvite(ctx, replacement)
	if err != nil {
		return nil, fmt.Errorf("%w: replace invite %s: %w", ErrInviteNotReissued, action.InviteID, err)
	}

	response := new(dto.NewOrganisationInviteResponse(*renewed, time.Now()))

	entry := inviteEntry(action.ActorID, action.IP, audit.ActionMemberInvited, *renewed)
	entry.Details["replaces"] = previous.ID.String()

	auditErr := s.record(ctx, entry)

	if err := s.email.Send(ctx, letter); err != nil {
		return response, fmt.Errorf("%w: %w", ErrInviteNotSent, errors.Join(fmt.Errorf("send the invite email: %w", err), auditErr))
	}

	return response, auditErr
}

func (s *service) Revoke(ctx context.Context, action spec.InviteAction) error {
	if _, err := s.liveOrganisation(ctx, action.OrgID); err != nil {
		return err
	}

	key := spec.OrganisationInviteKey{OrgID: action.OrgID, InviteID: action.InviteID}

	pending, err := s.orgs.GetInvite(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrInviteNotFound
	}
	if err != nil {
		return fmt.Errorf("look up invite %s: %w", action.InviteID, err)
	}

	if pending.RevokedAt != nil {
		return ErrInviteNotFound
	}

	if err := s.requireAdminGrant(ctx, action.OrgID, action.ActorID, pending.Role, ErrAdminRevokeForbidden); err != nil {
		return err
	}

	revoked, err := s.orgs.RevokeInvite(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrInviteNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke invite %s: %w", action.InviteID, err)
	}

	return s.record(ctx, inviteEntry(action.ActorID, action.IP, audit.ActionInviteRevoked, *revoked))
}

func (s *service) DeleteExpired(ctx context.Context) (int64, error) {
	deleted, err := s.orgs.DeleteExpiredInvites(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired invites: %w", err)
	}

	return deleted, nil
}

func (s *service) requireAdminGrant(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, role model.OrgRole, refusal error) error {
	if role != model.OrgRoleAdmin {
		return nil
	}

	decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: orgID, UserID: actorID, Permission: orgauthz.PermOrgManageAdmins})
	if err != nil {
		return fmt.Errorf("check whether user %s may manage admins: %w", actorID, err)
	}

	if !decision.Allowed {
		return refusal
	}

	return nil
}

func (s *service) inviteTeam(ctx context.Context, orgID uuid.UUID, teamID *uuid.UUID) (*model.Team, error) {
	if teamID == nil {
		return nil, nil
	}

	team, err := s.teams.GetByID(ctx, spec.TeamKey{OrgID: orgID, TeamID: *teamID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("look up team %s: %w", *teamID, err)
	}

	if team.Archived() {
		return nil, ErrTeamArchived
	}

	return team, nil
}

func (s *service) requireEmail(ctx context.Context) error {
	enabled, err := s.email.Enabled(ctx)
	if err != nil {
		return fmt.Errorf("check the email provider: %w", err)
	}

	if !enabled {
		return ErrEmailUnavailable
	}

	return nil
}

func (s *service) requireInvitable(ctx context.Context, orgID uuid.UUID, address string) error {
	if err := s.requireEmail(ctx); err != nil {
		return err
	}

	account, err := s.users.GetByEmail(ctx, address)
	if errors.Is(err, dao.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("look up the account by email: %w", err)
	}

	return s.requireNotJoined(ctx, orgID, account.ID)
}

func (s *service) requireNoPending(ctx context.Context, orgID uuid.UUID, address string) error {
	invites, err := s.orgs.ListInvites(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list the invites of organisation %s: %w", orgID, err)
	}

	now := time.Now()
	for _, invite := range invites {
		if invite.Email != nil && strings.EqualFold(*invite.Email, address) && !invite.Expired(now) {
			return ErrInvitePending
		}
	}

	return nil
}

func (s *service) composeLetter(ctx context.Context, current site, letter inviteLetter) (email.Message, error) {
	inviter, err := s.users.GetByID(ctx, letter.inviterID)
	if err != nil {
		return email.Message{}, fmt.Errorf("look up the inviter %s: %w", letter.inviterID, err)
	}

	data := notification.OrgInviteEmailData{
		To:           letter.address,
		SiteName:     current.name,
		Organisation: letter.organisation.Name,
		InviterName:  inviter.DisplayName,
		Admin:        letter.role == model.OrgRoleAdmin,
		LinkURL:      letter.link,
	}
	if letter.team != nil {
		data.Team = letter.team.Name
	}

	return notification.OrgInviteEmail(data)
}

func draftInvite(creation spec.InviteCreation) (spec.NewOrganisationInvite, error) {
	role, err := inviteRole(creation.Role)
	if err != nil {
		return spec.NewOrganisationInvite{}, err
	}

	pending := spec.NewOrganisationInvite{OrgID: creation.OrgID, TeamID: creation.TeamID, Role: role, MaxUses: creation.MaxUses, InvitedBy: creation.ActorID}

	if creation.Email == "" {
		if creation.MaxUses != nil && *creation.MaxUses < 1 {
			return spec.NewOrganisationInvite{}, ErrInvalidMaxUses
		}

		return pending, nil
	}

	address, err := normaliseEmail(creation.Email)
	if err != nil {
		return spec.NewOrganisationInvite{}, err
	}

	if creation.MaxUses != nil && *creation.MaxUses != emailInviteUses {
		return spec.NewOrganisationInvite{}, ErrInvalidMaxUses
	}

	pending.Email = new(address)
	pending.MaxUses = new(emailInviteUses)

	return pending, nil
}

func inviteRole(requested model.OrgRole) (model.OrgRole, error) {
	switch requested {
	case "", model.OrgRoleMember:
		return model.OrgRoleMember, nil
	case model.OrgRoleAdmin:
		return model.OrgRoleAdmin, nil
	}

	return "", ErrInvalidRole
}

func inviteEntry(actorID uuid.UUID, ip string, action audit.Action, invite model.OrganisationInvite) audit.NewEntry {
	details := map[string]string{"role": string(invite.Role)}
	if invite.Email != nil {
		details["email"] = *invite.Email
	}

	if invite.TeamID != nil {
		details["team_id"] = invite.TeamID.String()
	}

	if invite.MaxUses != nil {
		details["max_uses"] = strconv.Itoa(*invite.MaxUses)
	}

	return audit.NewEntry{
		OrgID:      invite.OrgID,
		ActorID:    actorID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetInvite,
		TargetID:   invite.ID.String(),
		Details:    details,
		IP:         ip,
	}
}

func usedUp(invite model.OrganisationInvite) bool {
	return invite.MaxUses != nil && invite.UseCount >= *invite.MaxUses
}
