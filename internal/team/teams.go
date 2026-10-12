package team

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	allowance struct {
		plan model.PlanCode
		max  int
	}
)

func (s *service) Create(ctx context.Context, request CreateRequest) (*dto.TeamResponse, error) {
	name, err := CleanName(request.Name)
	if err != nil {
		return nil, err
	}

	if err := s.requireLiveOrganisation(ctx, request.OrgID); err != nil {
		return nil, err
	}

	creator, err := s.organisations.GetMember(ctx, spec.OrganisationMemberKey{OrgID: request.OrgID, UserID: request.ActorID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read the membership of user %s in organisation %s: %w", request.ActorID, request.OrgID, err)
	}

	allowed, err := s.teamAllowance(ctx, request.OrgID)
	if err != nil {
		return nil, err
	}

	current, err := s.teams.CountActive(ctx, request.OrgID)
	if err != nil {
		return nil, fmt.Errorf("count the active teams of organisation %s: %w", request.OrgID, err)
	}

	if err := s.entitlements.CheckWithin(ctx, spec.LimitCheck{OrgID: request.OrgID, Limit: model.LimitTeams, Current: current}); err != nil {
		return nil, fmt.Errorf("check the team allowance of organisation %s: %w", request.OrgID, err)
	}

	created, err := s.teams.Create(ctx, spec.NewTeam{OrgID: request.OrgID, Name: name, MaxTeams: allowed.max})
	if errors.Is(err, dao.ErrQuotaExceeded) {
		return nil, &entitlements.DeniedError{Plan: allowed.plan, Limit: model.LimitTeams, Allowed: allowed.max, Current: allowed.max}
	}
	if errors.Is(err, dao.ErrConflict) {
		return nil, ErrNameTaken
	}
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create team %q in organisation %s: %w", name, request.OrgID, err)
	}

	response := new(dto.NewTeamResponse(*created, orgauthz.EffectiveTeamRole(creator.Role, "")))

	return response, s.record(ctx, teamEntry(request.Actor, *created, audit.ActionTeamCreated, map[string]string{"name": created.Name}))
}

func (s *service) List(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.TeamListResponse, error) {
	if err := s.requireLiveOrganisation(ctx, viewer.OrgID); err != nil {
		return nil, err
	}

	member, err := s.organisations.GetMember(ctx, viewer)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read the membership of user %s in organisation %s: %w", viewer.UserID, viewer.OrgID, err)
	}

	teams, err := s.teams.ListForOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the teams of organisation %s: %w", viewer.OrgID, err)
	}

	joined, err := s.teams.ListForMember(ctx, viewer)
	if err != nil {
		return nil, fmt.Errorf("list the teams of user %s in organisation %s: %w", viewer.UserID, viewer.OrgID, err)
	}

	roles := make(map[uuid.UUID]model.TeamRole, len(joined))
	for _, membership := range joined {
		roles[membership.ID] = membership.Role
	}

	listed := make([]dto.TeamResponse, 0, len(teams))
	for _, team := range teams {
		effective := orgauthz.EffectiveTeamRole(member.Role, roles[team.ID])
		if !orgauthz.TeamRoleGrants(effective, orgauthz.PermTeamView) {
			continue
		}

		listed = append(listed, dto.NewTeamResponse(team, effective))
	}

	return &dto.TeamListResponse{Teams: listed}, nil
}

func (s *service) Get(ctx context.Context, viewer Viewer) (*dto.TeamResponse, error) {
	found, err := s.header(ctx, viewer.TeamID)
	if err != nil {
		return nil, err
	}

	role, err := s.viewerRole(ctx, *found, viewer.UserID)
	if err != nil {
		return nil, err
	}

	return new(dto.NewTeamResponse(*found, role)), nil
}

func (s *service) Rename(ctx context.Context, request RenameRequest) (*dto.TeamResponse, error) {
	name, err := CleanName(request.Name)
	if err != nil {
		return nil, err
	}

	found, err := s.activeHeader(ctx, request.TeamID)
	if err != nil {
		return nil, err
	}

	role, err := s.viewerRole(ctx, *found, request.ActorID)
	if err != nil {
		return nil, err
	}

	renamed, err := s.teams.Rename(ctx, spec.TeamRename{OrgID: found.OrgID, TeamID: found.ID, Name: name})
	if errors.Is(err, dao.ErrConflict) {
		return nil, ErrNameTaken
	}
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrTeamNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return nil, fmt.Errorf("rename team %s: %w", found.ID, err)
	}

	return new(dto.NewTeamResponse(*renamed, role)), settle(err, nil)
}

func (s *service) Archive(ctx context.Context, command OrgTeamCommand) error {
	if err := s.requireLiveOrganisation(ctx, command.OrgID); err != nil {
		return err
	}

	key := spec.TeamKey{OrgID: command.OrgID, TeamID: command.TeamID}

	found, err := s.teams.GetByID(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrTeamNotFound
	}
	if err != nil {
		return fmt.Errorf("read team %s: %w", command.TeamID, err)
	}

	if found.Archived() {
		return ErrTeamArchived
	}

	archived, err := s.teams.Archive(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrTeamNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("archive team %s: %w", command.TeamID, err)
	}

	return settle(err, s.record(ctx, teamEntry(command.Actor, *archived, audit.ActionTeamArchived, map[string]string{"name": archived.Name})))
}

func (s *service) Delete(ctx context.Context, command OrgTeamCommand) error {
	if err := s.requireLiveOrganisation(ctx, command.OrgID); err != nil {
		return err
	}

	deleted, err := s.teams.Delete(ctx, spec.TeamKey{OrgID: command.OrgID, TeamID: command.TeamID})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrTeamNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("delete team %s: %w", command.TeamID, err)
	}

	return settle(err, s.record(ctx, teamEntry(command.Actor, *deleted, audit.ActionTeamDeleted, map[string]string{"name": deleted.Name})))
}

func (s *service) teamAllowance(ctx context.Context, orgID uuid.UUID) (allowance, error) {
	plan, err := s.entitlements.For(ctx, orgID)
	if err != nil {
		return allowance{}, fmt.Errorf("read the plan of organisation %s: %w", orgID, err)
	}

	allowed, known := plan.Allowance(model.LimitTeams)
	if !known {
		return allowance{}, fmt.Errorf("%w: %q on the %s plan", entitlements.ErrUnknownLimit, model.LimitTeams, plan.Code)
	}

	return allowance{plan: plan.Code, max: allowed}, nil
}
