package dto

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	AdminOrganisationResponse struct {
		ID               uuid.UUID      `json:"id"`
		Name             string         `json:"name"`
		Slug             string         `json:"slug"`
		Plan             model.PlanCode `json:"plan"`
		OwnerID          *uuid.UUID     `json:"owner_id,omitempty"`
		OwnerDisplayName string         `json:"owner_display_name,omitempty"`
		OwnerEmail       string         `json:"owner_email,omitempty"`
		MemberCount      int            `json:"member_count"`
		TeamCount        int            `json:"team_count"`
		BoardCount       int            `json:"board_count"`
		CreatedAt        string         `json:"created_at"`
		DeletedAt        string         `json:"deleted_at,omitempty"`
	}

	AdminOrganisationListResponse struct {
		Organisations []AdminOrganisationResponse `json:"organisations"`
		Total         int                         `json:"total"`
		Limit         int                         `json:"limit"`
		Offset        int                         `json:"offset"`
	}

	AdminOrganisationTeamResponse struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		MemberCount int       `json:"member_count"`
		BoardCount  int       `json:"board_count"`
		CreatedAt   string    `json:"created_at"`
		ArchivedAt  string    `json:"archived_at,omitempty"`
	}

	AdminOrganisationDetailResponse struct {
		AdminOrganisationResponse

		Teams   []AdminOrganisationTeamResponse `json:"teams"`
		Members []OrganisationMemberResponse    `json:"members"`
	}
)

func NewAdminOrganisationResponse(o model.StaffOrganisation, plan model.PlanCode) AdminOrganisationResponse {
	return AdminOrganisationResponse{
		ID:               o.ID,
		Name:             o.Name,
		Slug:             o.Slug,
		Plan:             plan,
		OwnerID:          o.OwnerID,
		OwnerDisplayName: optionalString(o.OwnerDisplayName),
		OwnerEmail:       optionalString(o.OwnerEmail),
		MemberCount:      o.MemberCount,
		TeamCount:        o.TeamCount,
		BoardCount:       o.BoardCount,
		CreatedAt:        o.CreatedAt.UTC().Format(time.RFC3339),
		DeletedAt:        optionalTime(o.DeletedAt),
	}
}

func NewAdminOrganisationTeamResponse(t model.TeamWithCounts) AdminOrganisationTeamResponse {
	return AdminOrganisationTeamResponse{
		ID:          t.ID,
		Name:        t.Name,
		MemberCount: t.MemberCount,
		BoardCount:  t.BoardCount,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
		ArchivedAt:  optionalTime(t.ArchivedAt),
	}
}
