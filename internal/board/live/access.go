package live

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"retrotide/internal/board/policy"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
)

type (
	membership struct {
		orgMember bool
		role      model.BoardRole
	}
)

const (
	teamViewPermission = orgauthz.PermTeamView
	teamLeadPermission = orgauthz.PermTeamManageBoards
	orgViewPermission  = orgauthz.PermOrgView
	orgLeadPermission  = orgauthz.PermOrgManageBoards
)

func (s *service) Resolve(ctx context.Context, query spec.BoardAccessQuery) (*model.BoardAccess, error) {
	header, err := s.boards.GetHeader(ctx, query.BoardID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrNoAccess, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the header of board %s: %w", query.BoardID, err)
	}

	return s.resolve(ctx, query.Principal, *header)
}

func (s *service) resolve(ctx context.Context, principal model.Principal, header model.BoardHeader) (*model.BoardAccess, error) {
	if header.Gone() {
		return nil, fmt.Errorf("%w: board %s or its organisation is deleted", ErrNoAccess, header.ID)
	}

	member, err := s.membership(ctx, principal, header)
	if err != nil {
		return nil, err
	}

	participant, err := s.participant(ctx, principal, header)
	if err != nil {
		return nil, err
	}

	role, err := effectiveRole(header, principal, member, participant)
	if err != nil {
		return nil, err
	}

	access := &model.BoardAccess{
		Principal:    principal,
		Header:       header,
		Role:         role,
		OrgMember:    member.orgMember,
		Capabilities: narrow(policy.Capabilities(role, member.orgMember), header),
	}

	if participant != nil {
		access.ParticipantID = new(participant.ID)
	}

	return access, nil
}

func (s *service) MembershipRole(ctx context.Context, header model.BoardHeader, principal model.Principal) (model.BoardRole, error) {
	member, err := s.membership(ctx, principal, header)
	if err != nil {
		return "", err
	}

	return member.role, nil
}

func (s *service) membership(ctx context.Context, principal model.Principal, header model.BoardHeader) (membership, error) {
	if principal.Kind != model.PrincipalUser {
		return membership{}, nil
	}

	viewPermission, leadPermission := teamViewPermission, teamLeadPermission
	if header.TeamID == nil {
		viewPermission, leadPermission = orgViewPermission, orgLeadPermission
	}

	view, err := s.can(ctx, principal, header, viewPermission)
	if err != nil {
		return membership{}, err
	}

	if !view.Allowed {
		return membership{orgMember: view.Member}, nil
	}

	lead, err := s.can(ctx, principal, header, leadPermission)
	if err != nil {
		return membership{}, err
	}

	if lead.Allowed {
		return membership{orgMember: view.Member, role: model.BoardRoleFacilitator}, nil
	}

	return membership{orgMember: view.Member, role: model.BoardRoleParticipant}, nil
}

func (s *service) can(ctx context.Context, principal model.Principal, header model.BoardHeader, permission orgauthz.Permission) (orgauthz.Decision, error) {
	if header.TeamID == nil {
		decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: header.OrgID, UserID: principal.ID, Permission: permission})
		if err != nil {
			return orgauthz.Decision{}, fmt.Errorf("decide %s for user %s in organisation %s: %w", permission, principal.ID, header.OrgID, err)
		}

		return decision, nil
	}

	decision, err := s.orgAuthz.CanInTeam(ctx, spec.TeamPermissionCheck{TeamID: *header.TeamID, UserID: principal.ID, Permission: permission})
	if err != nil {
		return orgauthz.Decision{}, fmt.Errorf("decide %s for user %s on team %s: %w", permission, principal.ID, *header.TeamID, err)
	}

	return decision, nil
}

func (s *service) participant(ctx context.Context, principal model.Principal, header model.BoardHeader) (*model.BoardParticipant, error) {
	var participant *model.BoardParticipant
	var err error

	switch principal.Kind {
	case model.PrincipalUser:
		participant, err = s.boards.GetParticipantForUser(ctx, spec.BoardParticipantUser{BoardID: header.ID, UserID: principal.ID})
	case model.PrincipalGuest:
		participant, err = s.boards.GetParticipantForGuest(ctx, spec.BoardParticipantGuest{BoardID: header.ID, GuestDeviceID: principal.ID})
	default:
		return nil, nil
	}

	if errors.Is(err, dao.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the participant of %s %s on board %s: %w", principal.Kind, principal.ID, header.ID, err)
	}

	return participant, nil
}

func effectiveRole(header model.BoardHeader, principal model.Principal, member membership, participant *model.BoardParticipant) (model.BoardRole, error) {
	if participant != nil && participant.Removed() {
		return "", fmt.Errorf("%w: participant %s of board %s", ErrRemoved, participant.ID, header.ID)
	}

	if member.role == model.BoardRoleFacilitator {
		return model.BoardRoleFacilitator, nil
	}

	if member.role == model.BoardRoleParticipant {
		if participant != nil && participant.Role == model.BoardRoleFacilitator {
			return model.BoardRoleFacilitator, nil
		}

		return model.BoardRoleParticipant, nil
	}

	if participant == nil || !participant.JoinedThroughLink() {
		return "", fmt.Errorf("%w: board %s has no path in for this principal", ErrNoAccess, header.ID)
	}

	return linkRole(header, principal, *participant)
}

func linkRole(header model.BoardHeader, principal model.Principal, participant model.BoardParticipant) (model.BoardRole, error) {
	if err := admitsThroughLink(header, principal); err != nil {
		return "", err
	}

	if participant.AccessEpoch == header.AccessEpoch {
		return model.BoardRoleParticipant, nil
	}

	if header.PasswordHash != nil {
		return "", fmt.Errorf("%w: board %s is at access epoch %d and participant %s at %d", ErrPasswordRequired, header.ID, header.AccessEpoch, participant.ID, participant.AccessEpoch)
	}

	return "", fmt.Errorf("%w: board %s is at access epoch %d and participant %s at %d", ErrNoAccess, header.ID, header.AccessEpoch, participant.ID, participant.AccessEpoch)
}

func admitsThroughLink(header model.BoardHeader, principal model.Principal) error {
	if header.LinkAccess == model.LinkAccessOff {
		return fmt.Errorf("%w: board %s", ErrLinkDisabled, header.ID)
	}

	if !header.Admits(principal.Kind) {
		return fmt.Errorf("%w: board %s admits accounts only", ErrAccountRequired, header.ID)
	}

	if principal.Kind != model.PrincipalUser && !header.OrgAllowsGuests {
		return fmt.Errorf("%w: organisation %s", ErrGuestsNotAllowed, header.OrgID)
	}

	return nil
}

func narrow(capabilities []model.BoardCapability, header model.BoardHeader) []model.BoardCapability {
	readOnly := header.Archived() || header.Closed() || header.Locked()
	noFacilitating := header.Archived() || header.Closed() && header.Anonymous()

	return slices.DeleteFunc(slices.Clone(capabilities), func(capability model.BoardCapability) bool {
		if capability == model.BoardCapabilityParticipate {
			return readOnly
		}

		if capability == model.BoardCapabilityFacilitate {
			return noFacilitating
		}

		return false
	})
}
