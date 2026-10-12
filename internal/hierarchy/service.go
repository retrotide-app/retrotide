package hierarchy

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/authz"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		ActOnUser(ctx context.Context, check spec.UserActionCheck) (*Standing, error)
		ActOnMember(ctx context.Context, check spec.MemberActionCheck) (*Standing, error)
		ActOnTeamMember(ctx context.Context, check spec.TeamMemberActionCheck) (*Standing, error)
	}

	Deps struct {
		Authz         authz.Service
		Organisations repository.OrganisationRepository
		Teams         repository.TeamRepository
	}

	service struct {
		authz         authz.Service
		organisations repository.OrganisationRepository
		teams         repository.TeamRepository
	}
)

func NewService(deps Deps) Service {
	return &service{authz: deps.Authz, organisations: deps.Organisations, teams: deps.Teams}
}

func (s *service) ActOnUser(ctx context.Context, check spec.UserActionCheck) (*Standing, error) {
	actor, err := s.siteHolder(ctx, check.ActorID)
	if err != nil {
		return nil, err
	}

	target, err := s.siteHolder(ctx, check.TargetID)
	if err != nil {
		return nil, err
	}

	return permitted(Standing{Actor: actor, Target: target}, check.ActorID, check.TargetID)
}

func (s *service) ActOnMember(ctx context.Context, check spec.MemberActionCheck) (*Standing, error) {
	actor, err := s.memberHolder(ctx, spec.OrganisationMemberKey{OrgID: check.OrgID, UserID: check.ActorID})
	if err != nil {
		return nil, err
	}

	target, err := s.memberHolder(ctx, spec.OrganisationMemberKey{OrgID: check.OrgID, UserID: check.TargetID})
	if err != nil {
		return nil, err
	}

	return permitted(Standing{Actor: actor, Target: target}, check.ActorID, check.TargetID)
}

func (s *service) ActOnTeamMember(ctx context.Context, check spec.TeamMemberActionCheck) (*Standing, error) {
	actor, err := s.teamHolder(ctx, spec.TeamMemberKey{OrgID: check.OrgID, TeamID: check.TeamID, UserID: check.ActorID})
	if err != nil {
		return nil, err
	}

	target, err := s.teamHolder(ctx, spec.TeamMemberKey{OrgID: check.OrgID, TeamID: check.TeamID, UserID: check.TargetID})
	if err != nil {
		return nil, err
	}

	return permitted(Standing{Actor: actor, Target: target}, check.ActorID, check.TargetID)
}

func (s *service) siteHolder(ctx context.Context, userID uuid.UUID) (Holder, error) {
	site, err := s.authz.GetRole(ctx, userID)
	if err != nil {
		return Holder{}, err
	}

	return Holder{Site: site}, nil
}

func (s *service) memberHolder(ctx context.Context, key spec.OrganisationMemberKey) (Holder, error) {
	member, err := s.organisations.GetMember(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return Holder{}, fmt.Errorf("%w: user %s in organisation %s: %w", ErrNotMember, key.UserID, key.OrgID, err)
	}
	if err != nil {
		return Holder{}, fmt.Errorf("read the membership of user %s in organisation %s: %w", key.UserID, key.OrgID, err)
	}

	holder, err := s.siteHolder(ctx, key.UserID)
	if err != nil {
		return Holder{}, err
	}

	holder.Org = member.Role

	return holder, nil
}

func (s *service) teamHolder(ctx context.Context, key spec.TeamMemberKey) (Holder, error) {
	holder, err := s.memberHolder(ctx, spec.OrganisationMemberKey{OrgID: key.OrgID, UserID: key.UserID})
	if err != nil {
		return Holder{}, err
	}

	team, err := s.teamRole(ctx, key)
	if err != nil {
		return Holder{}, err
	}

	holder.Team = team

	return holder, nil
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

func permitted(standing Standing, actorID uuid.UUID, targetID uuid.UUID) (*Standing, error) {
	if !standing.Permits() {
		return nil, fmt.Errorf("%w: user %s acting on user %s", ErrOutranked, actorID, targetID)
	}

	return &standing, nil
}
