package team

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

func (s *service) ListMembers(ctx context.Context, teamID uuid.UUID) (*dto.TeamMemberListResponse, error) {
	found, err := s.header(ctx, teamID)
	if err != nil {
		return nil, err
	}

	members, err := s.teams.ListMembers(ctx, spec.TeamKey{OrgID: found.OrgID, TeamID: found.ID})
	if err != nil {
		return nil, fmt.Errorf("list the members of team %s: %w", teamID, err)
	}

	listed := make([]dto.TeamMemberResponse, 0, len(members))
	for _, member := range members {
		listed = append(listed, dto.NewTeamMemberResponse(member))
	}

	return &dto.TeamMemberListResponse{Members: listed}, nil
}

func (s *service) AddMember(ctx context.Context, change MemberRole) error {
	if !validRole(change.Role) {
		return ErrInvalidRole
	}

	found, err := s.activeHeader(ctx, change.TeamID)
	if err != nil {
		return err
	}

	standing, err := s.hierarchy.ActOnTeamMember(ctx, spec.TeamMemberActionCheck{OrgID: found.OrgID, TeamID: found.ID, ActorID: change.ActorID, TargetID: change.UserID})
	if errors.Is(err, hierarchy.ErrNotMember) {
		return ErrMemberNotFound
	}
	if err != nil {
		return fmt.Errorf("add user %s to team %s: %w", change.UserID, found.ID, err)
	}

	if standing.Target.Team != "" {
		return ErrAlreadyTeamMember
	}

	if err := mayBecome(*standing, change); err != nil {
		return err
	}

	added, err := s.teams.AddMember(ctx, spec.NewTeamMember{OrgID: found.OrgID, TeamID: found.ID, UserID: change.UserID, Role: change.Role})
	if errors.Is(err, dao.ErrConflict) {
		return ErrAlreadyTeamMember
	}
	if errors.Is(err, dao.ErrNotFound) {
		return ErrMemberNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("add user %s to team %s: %w", change.UserID, found.ID, err)
	}

	return settle(err, s.record(ctx, memberEntry(change.Actor, *found, audit.ActionTeamMemberAdded, change.UserID, map[string]string{"role": string(added.Role)})))
}

func (s *service) SetMemberRole(ctx context.Context, change MemberRole) error {
	if !validRole(change.Role) {
		return ErrInvalidRole
	}

	found, err := s.activeHeader(ctx, change.TeamID)
	if err != nil {
		return err
	}

	standing, err := s.actOnTeamMember(ctx, *found, change.MemberCommand)
	if err != nil {
		return err
	}

	if err := mayBecome(*standing, change); err != nil {
		return err
	}

	changed, err := s.teams.SetMemberRole(ctx, spec.TeamMemberRoleChange{OrgID: found.OrgID, TeamID: found.ID, UserID: change.UserID, Role: change.Role})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrTeamMemberNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("change the role of user %s in team %s: %w", change.UserID, found.ID, err)
	}

	details := map[string]string{"role": string(changed.Role), "previous_role": string(standing.Target.Team)}

	return settle(err, s.record(ctx, memberEntry(change.Actor, *found, audit.ActionTeamMemberRoleChanged, change.UserID, details)))
}

func (s *service) RemoveMember(ctx context.Context, command MemberCommand) error {
	found, err := s.activeHeader(ctx, command.TeamID)
	if err != nil {
		return err
	}

	if _, err := s.actOnTeamMember(ctx, *found, command); err != nil {
		return err
	}

	removed, err := s.teams.RemoveMember(ctx, spec.TeamMemberKey{OrgID: found.OrgID, TeamID: found.ID, UserID: command.UserID})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrTeamMemberNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("remove user %s from team %s: %w", command.UserID, found.ID, err)
	}

	return settle(err, s.record(ctx, memberEntry(command.Actor, *found, audit.ActionTeamMemberRemoved, command.UserID, map[string]string{"role": string(removed.Role)})))
}

func (s *service) actOnTeamMember(ctx context.Context, team model.Team, command MemberCommand) (*hierarchy.Standing, error) {
	standing, err := s.hierarchy.ActOnTeamMember(ctx, spec.TeamMemberActionCheck{OrgID: team.OrgID, TeamID: team.ID, ActorID: command.ActorID, TargetID: command.UserID})
	if errors.Is(err, hierarchy.ErrNotMember) {
		return nil, ErrTeamMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("act on user %s in team %s: %w", command.UserID, team.ID, err)
	}

	if standing.Target.Team == "" {
		return nil, ErrTeamMemberNotFound
	}

	return standing, nil
}

func mayBecome(standing hierarchy.Standing, change MemberRole) error {
	after := standing.Target
	after.Team = change.Role

	if !standing.MayChangeTo(after) {
		return fmt.Errorf("%w: user %s cannot make user %s a team %s", hierarchy.ErrOutranked, change.ActorID, change.UserID, change.Role)
	}

	return nil
}
