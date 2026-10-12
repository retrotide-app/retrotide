package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	EmailVerificationToken struct {
		TokenHash    string
		UserID       uuid.UUID
		PendingEmail *string
		ExpiresAt    time.Time
		UsedAt       *time.Time
		CreatedAt    time.Time
	}
)

func (t *EmailVerificationToken) ChangesEmail() bool {
	return t.PendingEmail != nil
}
