package dto

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	InvitationTokenRequest struct {
		Token string `json:"token"`
	}

	InvitationResponse struct {
		Email     string `json:"email"`
		ExpiresAt string `json:"expires_at"`
	}

	AcceptInvitationRequest struct {
		Token       string `json:"token"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}

	InvitationAcceptedResponse struct {
		Email string `json:"email"`
	}

	AdminInviteRequest struct {
		Email string `json:"email"`
	}

	AdminInvitationResponse struct {
		ID        uuid.UUID `json:"id"`
		Email     string    `json:"email"`
		Expired   bool      `json:"expired"`
		ExpiresAt string    `json:"expires_at"`
		SentAt    string    `json:"sent_at"`
		CreatedAt string    `json:"created_at"`
	}

	AdminInvitationListResponse struct {
		Invitations []AdminInvitationResponse `json:"invitations"`
		Total       int                       `json:"total"`
		Limit       int                       `json:"limit"`
		Offset      int                       `json:"offset"`
	}
)

func NewInvitationResponse(invitation model.Invitation) InvitationResponse {
	return InvitationResponse{Email: invitation.Email, ExpiresAt: invitation.ExpiresAt.UTC().Format(time.RFC3339)}
}

func NewAdminInvitationResponse(invitation model.Invitation, now time.Time) AdminInvitationResponse {
	return AdminInvitationResponse{
		ID:        invitation.ID,
		Email:     invitation.Email,
		Expired:   invitation.Expired(now),
		ExpiresAt: invitation.ExpiresAt.UTC().Format(time.RFC3339),
		SentAt:    invitation.UpdatedAt.UTC().Format(time.RFC3339),
		CreatedAt: invitation.CreatedAt.UTC().Format(time.RFC3339),
	}
}
