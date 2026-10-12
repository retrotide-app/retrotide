package org

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) ListForUser(ctx context.Context, userID uuid.UUID) (*dto.OrganisationListResponse, error) {
	rows, err := s.organisations.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the organisations of user %s: %w", userID, err)
	}

	listed := make([]dto.OrganisationResponse, 0, len(rows))
	for _, row := range rows {
		listed = append(listed, dto.NewOrganisationResponse(row.Organisation, row.Role))
	}

	return &dto.OrganisationListResponse{Organisations: listed}, nil
}

func (s *service) Get(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.OrganisationResponse, error) {
	member, err := s.membership(ctx, viewer)
	if err != nil {
		return nil, err
	}

	found, err := s.live(ctx, viewer.OrgID)
	if err != nil {
		return nil, err
	}

	return new(dto.NewOrganisationResponse(*found, member.Role)), nil
}

func (s *service) Update(ctx context.Context, request UpdateRequest) (*dto.OrganisationResponse, error) {
	name, err := cleanName(request.Name)
	if err != nil {
		return nil, err
	}

	slug, err := normaliseSlug(request.Slug)
	if err != nil {
		return nil, err
	}

	member, err := s.membership(ctx, spec.OrganisationMemberKey{OrgID: request.OrgID, UserID: request.ActorID})
	if err != nil {
		return nil, err
	}

	current, err := s.live(ctx, request.OrgID)
	if err != nil {
		return nil, err
	}

	updated, err := s.organisations.Update(ctx, spec.OrganisationUpdate{OrgID: request.OrgID, Slug: slug, Name: name, AllowGuests: request.AllowGuests})
	if errors.Is(err, dao.ErrConflict) {
		return nil, ErrSlugTaken
	}
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update organisation %s: %w", request.OrgID, err)
	}

	response := new(dto.NewOrganisationResponse(*updated, member.Role))

	return response, s.record(ctx, orgEntry(request.Actor, updated.ID, audit.ActionOrgUpdated, changes(*current, *updated)))
}

func (s *service) Delete(ctx context.Context, command Command) error {
	deleted, err := s.organisations.SoftDelete(ctx, command.OrgID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrOrganisationNotFound
	}
	if err != nil {
		return fmt.Errorf("delete organisation %s: %w", command.OrgID, err)
	}

	return s.record(ctx, orgEntry(command.Actor, deleted.ID, audit.ActionOrgDeleted, map[string]string{"name": deleted.Name, "slug": deleted.Slug}))
}

func (s *service) Restore(ctx context.Context, command Command) (*dto.OrganisationResponse, error) {
	member, err := s.membership(ctx, spec.OrganisationMemberKey{OrgID: command.OrgID, UserID: command.ActorID})
	if err != nil {
		return nil, err
	}

	if member.Role != model.OrgRoleOwner {
		return nil, ErrNotOwner
	}

	found, err := s.organisations.GetByID(ctx, command.OrgID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read organisation %s: %w", command.OrgID, err)
	}

	if !found.Deleted() {
		return nil, ErrOrganisationNotDeleted
	}

	cutoff := s.now().Add(-model.OrganisationRestoreWindow)
	if !found.DeletedAt.After(cutoff) {
		return nil, ErrNotRestorable
	}

	allowed, err := s.ownedAllowance(ctx, command.ActorID)
	if err != nil {
		return nil, err
	}

	restored, err := s.organisations.Restore(ctx, spec.OrganisationRestore{OrgID: command.OrgID, OwnerID: command.ActorID, DeletedAfter: cutoff, MaxOwned: allowed})
	if errors.Is(err, dao.ErrQuotaExceeded) {
		return nil, ErrOwnedLimitReached
	}
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrNotRestorable
	}
	if err != nil {
		return nil, fmt.Errorf("restore organisation %s: %w", command.OrgID, err)
	}

	response := new(dto.NewOrganisationResponse(*restored, member.Role))

	return response, s.record(ctx, orgEntry(command.Actor, restored.ID, audit.ActionOrgRestored, map[string]string{"name": restored.Name, "slug": restored.Slug}))
}

func (s *service) membership(ctx context.Context, key spec.OrganisationMemberKey) (*model.OrganisationMember, error) {
	member, err := s.organisations.GetMember(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read the membership of user %s in organisation %s: %w", key.UserID, key.OrgID, err)
	}

	return member, nil
}

func (s *service) live(ctx context.Context, orgID uuid.UUID) (*model.Organisation, error) {
	found, err := s.organisations.GetByID(ctx, orgID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read organisation %s: %w", orgID, err)
	}

	if found.Deleted() {
		return nil, ErrOrganisationNotFound
	}

	return found, nil
}

func changes(before model.Organisation, after model.Organisation) map[string]string {
	changed := make(map[string]string)

	if before.Name != after.Name {
		changed["name"] = after.Name
		changed["previous_name"] = before.Name
	}

	if before.Slug != after.Slug {
		changed["slug"] = after.Slug
		changed["previous_slug"] = before.Slug
	}

	if before.AllowGuests != after.AllowGuests {
		changed["allow_guests"] = strconv.FormatBool(after.AllowGuests)
	}

	return changed
}
