package model

import (
	"time"

	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	UserWithRole struct {
		User

		Role role.Role
	}

	User struct {
		ID                 uuid.UUID
		Email              string
		PasswordHash       *string
		DisplayName        string
		EmailVerified      bool
		EmailNotifications bool
		BannedAt           *time.Time
		BannedBy           *uuid.UUID
		BanReason          string
		LockedAt           *time.Time
		LockedBy           *uuid.UUID
		LockReason         string
		LastSeenIP         *string
		LastSeenIPAt       *time.Time
		CreatedAt          time.Time
		UpdatedAt          time.Time
	}
)

func (u *User) HasPassword() bool {
	return u.PasswordHash != nil
}
