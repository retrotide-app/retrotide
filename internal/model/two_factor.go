package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	UserTOTP struct {
		UserID       uuid.UUID
		Secret       string
		ConfirmedAt  *time.Time
		LastUsedStep int64
		CreatedAt    time.Time
	}

	TwoFactorChallenge struct {
		TokenHash string
		UserID    uuid.UUID
		ReturnTo  *string
		Remember  bool
		Attempts  int
		CreatedAt time.Time
		ExpiresAt time.Time
	}
)

func (t *UserTOTP) Enabled() bool {
	return t.ConfirmedAt != nil
}
