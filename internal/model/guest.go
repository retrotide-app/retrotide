package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	GuestDevice struct {
		ID         uuid.UUID
		CreatedAt  time.Time
		LastSeenAt time.Time
		ExpiresAt  time.Time
	}

	GuestCredential struct {
		Token    string
		DeviceID uuid.UUID
	}

	GuestBoard struct {
		Board       Board
		Participant BoardParticipant
	}
)

const (
	GuestDeviceUnseenLimit = 90 * 24 * time.Hour
)
