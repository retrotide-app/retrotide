package dto

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	UserResponse struct {
		ID                 uuid.UUID `json:"id"`
		Email              string    `json:"email"`
		DisplayName        string    `json:"display_name"`
		EmailVerified      bool      `json:"email_verified"`
		HasPassword        bool      `json:"has_password"`
		EmailNotifications bool      `json:"email_notifications"`
		CreatedAt          string    `json:"created_at"`
	}
)

func NewUserResponse(u *model.User) UserResponse {
	return UserResponse{
		ID:                 u.ID,
		Email:              u.Email,
		DisplayName:        u.DisplayName,
		EmailVerified:      u.EmailVerified,
		HasPassword:        u.HasPassword(),
		EmailNotifications: u.EmailNotifications,
		CreatedAt:          u.CreatedAt.UTC().Format(time.RFC3339),
	}
}
