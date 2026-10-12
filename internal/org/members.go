package org

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
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

func (s *service) ListMembers(ctx context.Context, orgID uuid.UUID) (*dto.OrganisationMemberListResponse, error) {
	if _, err := s.live(ctx, orgID); err != nil {
		return nil, err
	}

	members, err := s.organisations.ListMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list the members of organisation %s: %w", orgID, err)
	}

	listed := make([]dto.OrganisationMemberResponse, 0, len(members))
	for _, member := range members {
		listed = append(listed, dto.NewOrganisationMemberResponse(member))
	}

	return &dto.OrganisationMemberListResponse{Members: listed}, nil
}

func (s *service) ChangeMemberRole(ctx context.Context, change RoleChange) error {
	if change.Role != model.OrgRoleAdmin && change.Role != model.OrgRoleMember {
		return ErrInvalidRole
	}

	standing, err := s.actOnMember(ctx, change.MemberCommand)
	if err != nil {
		return err
	}

	previous := standing.Target.Org
	if previous == model.OrgRoleAdmin || change.Role == model.OrgRoleAdmin {
		if err := requireManageAdmins(*standing); err != nil {
			return err
		}
	}

	after := standing.Target
	after.Org = change.Role
	if !standing.MayChangeTo(after) {
		return fmt.Errorf("%w: user %s cannot make user %s an organisation %s", hierarchy.ErrOutranked, change.ActorID, change.UserID, change.Role)
	}

	changed, err := s.organisations.SetMemberRole(ctx, spec.OrganisationMemberRoleChange{OrgID: change.OrgID, UserID: change.UserID, Role: change.Role})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrMemberNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("change the role of user %s in organisation %s: %w", change.UserID, change.OrgID, err)
	}

	details := map[string]string{"role": string(changed.Role), "previous_role": string(previous)}

	return settle(err, s.record(ctx, memberEntry(change.Actor, change.OrgID, audit.ActionMemberRoleChanged, change.UserID, details)))
}

func (s *service) RemoveMember(ctx context.Context, command MemberCommand) error {
	standing, err := s.actOnMember(ctx, command)
	if err != nil {
		return err
	}

	if standing.Target.Org == model.OrgRoleAdmin {
		if err := requireManageAdmins(*standing); err != nil {
			return err
		}
	}

	removed, err := s.organisations.RemoveMember(ctx, spec.OrganisationMemberKey{OrgID: command.OrgID, UserID: command.UserID})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrMemberNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("remove user %s from organisation %s: %w", command.UserID, command.OrgID, err)
	}

	return settle(err, s.record(ctx, memberEntry(command.Actor, command.OrgID, audit.ActionMemberRemoved, command.UserID, map[string]string{"role": string(removed.Role)})))
}

func (s *service) Leave(ctx context.Context, command Command) error {
	if _, err := s.live(ctx, command.OrgID); err != nil {
		return err
	}

	key := spec.OrganisationMemberKey{OrgID: command.OrgID, UserID: command.ActorID}

	member, err := s.membership(ctx, key)
	if err != nil {
		return err
	}

	if member.Role == model.OrgRoleOwner {
		return ErrOwnerCannotLeave
	}

	removed, err := s.organisations.RemoveMember(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrMemberNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("user %s leaving organisation %s: %w", command.ActorID, command.OrgID, err)
	}

	details := map[string]string{"role": string(removed.Role), "left": "true"}

	return settle(err, s.record(ctx, memberEntry(command.Actor, command.OrgID, audit.ActionMemberRemoved, command.ActorID, details)))
}

func (s *service) TransferOwnership(ctx context.Context, command MemberCommand) error {
	if _, err := s.actOnMember(ctx, command); err != nil {
		return err
	}

	allowed, err := s.maxOwnedPerOwner(ctx)
	if err != nil {
		return err
	}

	err = s.organisations.TransferOwnership(ctx, spec.OwnershipTransfer{OrgID: command.OrgID, FromUserID: command.ActorID, ToUserID: command.UserID, MaxOwned: allowed})
	if errors.Is(err, dao.ErrQuotaExceeded) {
		return ErrNewOwnerLimitReached
	}
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrOwnershipNotTransferred, err)
	}
	if err != nil && !errors.Is(err, repository.ErrStaleTenantCache) {
		return fmt.Errorf("transfer organisation %s to user %s: %w", command.OrgID, command.UserID, err)
	}

	return settle(err, s.record(ctx, memberEntry(command.Actor, command.OrgID, audit.ActionOrgOwnershipTransferred, command.UserID, nil)))
}

func (s *service) actOnMember(ctx context.Context, command MemberCommand) (*hierarchy.Standing, error) {
	if _, err := s.live(ctx, command.OrgID); err != nil {
		return nil, err
	}

	standing, err := s.hierarchy.ActOnMember(ctx, spec.MemberActionCheck{OrgID: command.OrgID, ActorID: command.ActorID, TargetID: command.UserID})
	if errors.Is(err, hierarchy.ErrNotMember) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("act on user %s in organisation %s: %w", command.UserID, command.OrgID, err)
	}

	if standing.Target.Org == model.OrgRoleOwner {
		return nil, ErrOwnerProtected
	}

	return standing, nil
}

func requireManageAdmins(standing hierarchy.Standing) error {
	if !orgauthz.OrgRoleGrants(standing.Actor.Org, orgauthz.PermOrgManageAdmins) {
		return ErrAdminsNeedOwner
	}

	return nil
}
