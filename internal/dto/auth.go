package dto

import (
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	RegisterRequest struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		DisplayName    string `json:"display_name"`
		TurnstileToken string `json:"turnstile_token"`
	}

	LoginRequest struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		Remember       bool   `json:"remember"`
		TurnstileToken string `json:"turnstile_token"`
	}

	RegistrationResponse struct {
		Email                string `json:"email"`
		VerificationRequired bool   `json:"verification_required"`
	}

	ForgotPasswordRequest struct {
		Email          string `json:"email"`
		TurnstileToken string `json:"turnstile_token"`
	}

	ResendVerificationRequest struct {
		Email          string `json:"email"`
		TurnstileToken string `json:"turnstile_token"`
	}

	ResetPasswordRequest struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}

	VerifyEmailRequest struct {
		Token string `json:"token"`
	}

	SetEmailRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	ChangePasswordRequest struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	DeleteAccountRequest struct {
		Password string `json:"password"`
	}

	UpdateDisplayNameRequest struct {
		DisplayName string `json:"display_name"`
	}

	SessionResponse struct {
		Authenticated bool          `json:"authenticated"`
		User          *UserResponse `json:"user,omitempty"`
		Permissions   []string      `json:"permissions,omitempty"`
		SiteRole      role.Role     `json:"site_role,omitempty"`
		Guest         *GuestView    `json:"guest,omitempty"`
	}

	GuestView struct {
		ID uuid.UUID `json:"id"`
	}
)
