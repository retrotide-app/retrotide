package dto

import (
	"time"

	"retrotide/internal/model"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	CreateTeamRequest struct {
		Name string `json:"name"`
	}

	RenameTeamRequest struct {
		Name string `json:"name"`
	}

	TeamResponse struct {
		ID         uuid.UUID      `json:"id"`
		OrgID      uuid.UUID      `json:"org_id"`
		Name       string         `json:"name"`
		Role       model.TeamRole `json:"role,omitempty"`
		CreatedAt  string         `json:"created_at"`
		ArchivedAt string         `json:"archived_at,omitempty"`
	}

	TeamListResponse struct {
		Teams []TeamResponse `json:"teams"`
	}

	AddTeamMemberRequest struct {
		UserID uuid.UUID `json:"user_id"`
		Role   string    `json:"role"`
	}

	ChangeTeamMemberRoleRequest struct {
		Role string `json:"role"`
	}

	TeamMemberResponse struct {
		UserID      uuid.UUID      `json:"user_id"`
		Email       string         `json:"email"`
		DisplayName string         `json:"display_name"`
		Role        model.TeamRole `json:"role"`
		OrgRole     model.OrgRole  `json:"org_role"`
		SiteRole    role.Role      `json:"site_role,omitempty"`
		JoinedAt    string         `json:"joined_at"`
	}

	TeamMemberListResponse struct {
		Members []TeamMemberResponse `json:"members"`
	}
)

func NewTeamResponse(t model.Team, viewerRole model.TeamRole) TeamResponse {
	return TeamResponse{
		ID:         t.ID,
		OrgID:      t.OrgID,
		Name:       t.Name,
		Role:       viewerRole,
		CreatedAt:  t.CreatedAt.UTC().Format(time.RFC3339),
		ArchivedAt: optionalTime(t.ArchivedAt),
	}
}

func NewTeamMemberResponse(m model.TeamMemberProfile) TeamMemberResponse {
	return TeamMemberResponse{
		UserID:      m.UserID,
		Email:       m.Email,
		DisplayName: m.DisplayName,
		Role:        m.Role,
		OrgRole:     m.OrgRole,
		SiteRole:    m.SiteRole,
		JoinedAt:    m.JoinedAt.UTC().Format(time.RFC3339),
	}
}
