package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	Invitation struct {
		ID        uuid.UUID
		Email     string
		TokenHash string
		ExpiresAt time.Time
		CreatedAt time.Time
		UpdatedAt time.Time
	}
)

func (i Invitation) Expired(now time.Time) bool {
	return !now.Before(i.ExpiresAt)
}
