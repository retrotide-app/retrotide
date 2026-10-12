package orgauthz

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		CanInOrg(ctx context.Context, check spec.OrgPermissionCheck) (Decision, error)
		CanInTeam(ctx context.Context, check spec.TeamPermissionCheck) (Decision, error)
	}

	Decision struct {
		Member  bool
		Allowed bool
	}

	service struct {
		organisations repository.OrganisationRepository
		teams         repository.TeamRepository
	}
)

func NewService(organisations repository.OrganisationRepository, teams repository.TeamRepository) Service {
	return &service{organisations: organisations, teams: teams}
}

func (s *service) CanInOrg(ctx context.Context, check spec.OrgPermissionCheck) (Decision, error) {
	if err := requireScope(check.Permission, ScopeOrg); err != nil {
		return Decision{}, err
	}

	if check.UserID == uuid.Nil {
		return Decision{}, nil
	}

	org, err := s.orgRole(ctx, spec.OrganisationMemberKey{OrgID: check.OrgID, UserID: check.UserID})
	if err != nil {
		return Decision{}, err
	}

	if org == "" {
		return Decision{}, nil
	}

	return Decision{Member: true, Allowed: OrgRoleGrants(org, check.Permission)}, nil
}

func (s *service) CanInTeam(ctx context.Context, check spec.TeamPermissionCheck) (Decision, error) {
	if err := requireScope(check.Permission, ScopeTeam); err != nil {
		return Decision{}, err
	}

	if check.UserID == uuid.Nil {
		return Decision{}, nil
	}

	header, err := s.teams.GetHeader(ctx, check.TeamID)
	if errors.Is(err, dao.ErrNotFound) {
		return Decision{}, nil
	}
	if err != nil {
		return Decision{}, fmt.Errorf("read the header of team %s: %w", check.TeamID, err)
	}

	org, err := s.orgRole(ctx, spec.OrganisationMemberKey{OrgID: header.OrgID, UserID: check.UserID})
	if err != nil {
		return Decision{}, err
	}

	if org == "" {
		return Decision{}, nil
	}

	team := EffectiveTeamRole(org, "")
	if team == "" {
		team, err = s.teamRole(ctx, spec.TeamMemberKey{OrgID: header.OrgID, TeamID: header.ID, UserID: check.UserID})
		if err != nil {
			return Decision{}, err
		}
	}

	return Decision{Member: true, Allowed: TeamRoleGrants(team, check.Permission)}, nil
}

func (s *service) orgRole(ctx context.Context, key spec.OrganisationMemberKey) (model.OrgRole, error) {
	member, err := s.organisations.GetMember(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read the membership of user %s in organisation %s: %w", key.UserID, key.OrgID, err)
	}

	return member.Role, nil
}

func (s *service) teamRole(ctx context.Context, key spec.TeamMemberKey) (model.TeamRole, error) {
	member, err := s.teams.GetMember(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read the membership of user %s in team %s: %w", key.UserID, key.TeamID, err)
	}

	return member.Role, nil
}

func requireScope(perm Permission, scope PermissionScope) error {
	def, known := LookupPermission(perm)
	if !known {
		return fmt.Errorf("%w: %q", ErrUnknownPermission, perm)
	}

	if def.Scope != scope {
		return fmt.Errorf("%w: %q is a %s permission, asked in the %s scope", ErrOutOfScope, perm, def.Scope, scope)
	}

	return nil
}
