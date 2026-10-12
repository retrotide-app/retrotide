package dto

import (
	"time"

	"retrotide/internal/model"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	CreateWorkspaceRequest struct {
		Name     string `json:"name"`
		TeamName string `json:"team_name"`
	}

	UpdateOrganisationRequest struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		AllowGuests bool   `json:"allow_guests"`
	}

	OrganisationResponse struct {
		ID              uuid.UUID     `json:"id"`
		Slug            string        `json:"slug"`
		Name            string        `json:"name"`
		AllowGuests     bool          `json:"allow_guests"`
		Role            model.OrgRole `json:"role,omitempty"`
		CreatedAt       string        `json:"created_at"`
		DeletedAt       string        `json:"deleted_at,omitempty"`
		RestorableUntil string        `json:"restorable_until,omitempty"`
	}

	OrganisationListResponse struct {
		Organisations []OrganisationResponse `json:"organisations"`
	}

	WorkspaceResponse struct {
		Organisation OrganisationResponse `json:"organisation"`
		Team         TeamResponse         `json:"team"`
	}

	OrganisationMemberResponse struct {
		UserID      uuid.UUID     `json:"user_id"`
		Email       string        `json:"email"`
		DisplayName string        `json:"display_name"`
		Role        model.OrgRole `json:"role"`
		SiteRole    role.Role     `json:"site_role,omitempty"`
		JoinedAt    string        `json:"joined_at"`
	}

	OrganisationMemberListResponse struct {
		Members []OrganisationMemberResponse `json:"members"`
	}

	ChangeMemberRoleRequest struct {
		Role string `json:"role"`
	}

	TransferOwnershipRequest struct {
		UserID uuid.UUID `json:"user_id"`
	}

	CreateInviteRequest struct {
		Email   string     `json:"email"`
		Role    string     `json:"role"`
		TeamID  *uuid.UUID `json:"team_id,omitempty"`
		MaxUses *int       `json:"max_uses,omitempty"`
	}

	OrganisationInviteResponse struct {
		ID        uuid.UUID     `json:"id"`
		Email     string        `json:"email,omitempty"`
		Role      model.OrgRole `json:"role"`
		TeamID    *uuid.UUID    `json:"team_id,omitempty"`
		MaxUses   *int          `json:"max_uses,omitempty"`
		UseCount  int           `json:"use_count"`
		Expired   bool          `json:"expired"`
		ExpiresAt string        `json:"expires_at"`
		CreatedAt string        `json:"created_at"`
	}

	OrganisationInviteListResponse struct {
		Invites []OrganisationInviteResponse `json:"invites"`
	}

	InviteTokenRequest struct {
		Token string `json:"token"`
	}

	InviteDetailsResponse struct {
		OrgName     string        `json:"org_name"`
		OrgSlug     string        `json:"org_slug"`
		TeamName    string        `json:"team_name,omitempty"`
		InviterName string        `json:"inviter_name,omitempty"`
		Role        model.OrgRole `json:"role"`
		Email       string        `json:"email,omitempty"`
		ExpiresAt   string        `json:"expires_at"`
	}

	EntitlementsResponse struct {
		Plan   model.PlanCode      `json:"plan"`
		Limits map[model.Limit]int `json:"limits"`
	}
)

func NewOrganisationResponse(o model.Organisation, viewerRole model.OrgRole) OrganisationResponse {
	return OrganisationResponse{
		ID:              o.ID,
		Slug:            o.Slug,
		Name:            o.Name,
		AllowGuests:     o.AllowGuests,
		Role:            viewerRole,
		CreatedAt:       o.CreatedAt.UTC().Format(time.RFC3339),
		DeletedAt:       optionalTime(o.DeletedAt),
		RestorableUntil: optionalTime(o.RestorableUntil()),
	}
}

func NewOrganisationMemberResponse(m model.OrganisationMemberProfile) OrganisationMemberResponse {
	return OrganisationMemberResponse{
		UserID:      m.UserID,
		Email:       m.Email,
		DisplayName: m.DisplayName,
		Role:        m.Role,
		SiteRole:    m.SiteRole,
		JoinedAt:    m.JoinedAt.UTC().Format(time.RFC3339),
	}
}

func NewOrganisationInviteResponse(i model.OrganisationInvite, now time.Time) OrganisationInviteResponse {
	return OrganisationInviteResponse{
		ID:        i.ID,
		Email:     optionalString(i.Email),
		Role:      i.Role,
		TeamID:    i.TeamID,
		MaxUses:   i.MaxUses,
		UseCount:  i.UseCount,
		Expired:   i.Expired(now),
		ExpiresAt: i.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt: i.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func NewInviteDetailsResponse(d model.InviteDetails) InviteDetailsResponse {
	return InviteDetailsResponse{
		OrgName:     d.OrgName,
		OrgSlug:     d.OrgSlug,
		TeamName:    optionalString(d.TeamName),
		InviterName: d.InviterName,
		Role:        d.Role,
		Email:       optionalString(d.Email),
		ExpiresAt:   d.ExpiresAt.UTC().Format(time.RFC3339),
	}
}
