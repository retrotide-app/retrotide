package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) ListOrganisations(ctx context.Context, listing spec.OrganisationListing) (*dto.AdminOrganisationListResponse, error) {
	limit, offset := clampPage(listing.Limit, listing.Offset, defaultOrganisationPageSize)
	page := spec.OrganisationListing{Search: strings.TrimSpace(listing.Search), Limit: limit, Offset: offset}

	rows, err := s.organisations.ListForStaff(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("list organisations: %w", err)
	}

	total, err := s.organisations.CountForStaff(ctx, page.Search)
	if err != nil {
		return nil, fmt.Errorf("count organisations: %w", err)
	}

	listed := make([]dto.AdminOrganisationResponse, 0, len(rows))
	for _, row := range rows {
		plan, err := s.planOf(ctx, row.ID)
		if err != nil {
			return nil, err
		}

		listed = append(listed, dto.NewAdminOrganisationResponse(row, plan))
	}

	return &dto.AdminOrganisationListResponse{Organisations: listed, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *service) GetOrganisation(ctx context.Context, orgID uuid.UUID) (*dto.AdminOrganisationDetailResponse, error) {
	found, err := s.organisations.GetForStaff(ctx, orgID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrOrganisationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("look up organisation %s: %w", orgID, err)
	}

	plan, err := s.planOf(ctx, orgID)
	if err != nil {
		return nil, err
	}

	teams, err := s.teams.ListWithCounts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list the teams of organisation %s: %w", orgID, err)
	}

	members, err := s.organisations.ListMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list the members of organisation %s: %w", orgID, err)
	}

	listedTeams := make([]dto.AdminOrganisationTeamResponse, 0, len(teams))
	for _, team := range teams {
		listedTeams = append(listedTeams, dto.NewAdminOrganisationTeamResponse(team))
	}

	listedMembers := make([]dto.OrganisationMemberResponse, 0, len(members))
	for _, member := range members {
		listedMembers = append(listedMembers, dto.NewOrganisationMemberResponse(member))
	}

	return &dto.AdminOrganisationDetailResponse{
		AdminOrganisationResponse: dto.NewAdminOrganisationResponse(*found, plan),
		Teams:                     listedTeams,
		Members:                   listedMembers,
	}, nil
}

func (s *service) planOf(ctx context.Context, orgID uuid.UUID) (model.PlanCode, error) {
	plan, err := s.entitlements.For(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("read the plan of organisation %s: %w", orgID, err)
	}

	return plan.Code, nil
}
