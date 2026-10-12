package dto

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	CreatedInviteResponse struct {
		OrganisationInviteResponse

		URL string `json:"url,omitempty"`
	}

	InviteRegistrationRequest struct {
		Token       string `json:"token"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}

	JoinedOrganisationResponse struct {
		OrgID   uuid.UUID     `json:"org_id"`
		OrgSlug string        `json:"org_slug"`
		OrgName string        `json:"org_name"`
		Role    model.OrgRole `json:"role"`
		TeamID  *uuid.UUID    `json:"team_id,omitempty"`
	}

	InviteRegistrationResponse struct {
		Email                string                     `json:"email"`
		VerificationRequired bool                       `json:"verification_required"`
		Joined               JoinedOrganisationResponse `json:"joined"`
	}
)

func NewJoinedOrganisationResponse(details model.InviteDetails, accepted model.AcceptedInvite) JoinedOrganisationResponse {
	joined := JoinedOrganisationResponse{
		OrgID:   accepted.Member.OrgID,
		OrgSlug: details.OrgSlug,
		OrgName: details.OrgName,
		Role:    accepted.Member.Role,
	}

	if accepted.TeamMember != nil {
		joined.TeamID = new(accepted.TeamMember.TeamID)
	}

	return joined
}
