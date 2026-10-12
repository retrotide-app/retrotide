package dto

import (
	"time"

	"retrotide/internal/model"
)

type (
	IdentityStartRequest struct {
		Purpose            string `json:"purpose"`
		CurrentPassword    string `json:"current_password"`
		ReturnTo           string `json:"return_to"`
		ReauthenticateWith string `json:"reauthenticate_with"`
	}

	IdentityStartResponse struct {
		URL string `json:"url"`
	}

	IdentityResponse struct {
		Provider     string `json:"provider"`
		ProviderName string `json:"provider_name"`
		Email        string `json:"email"`
		CreatedAt    string `json:"created_at"`
	}

	IdentityListResponse struct {
		Identities []IdentityResponse `json:"identities"`
	}

	IdentityProviderResponse struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
)

func NewIdentityResponse(identity model.UserIdentity, providerName string) IdentityResponse {
	return IdentityResponse{
		Provider:     string(identity.Provider),
		ProviderName: providerName,
		Email:        identity.Email,
		CreatedAt:    identity.CreatedAt.UTC().Format(time.RFC3339),
	}
}
