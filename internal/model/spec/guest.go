package spec

import (
	"time"

	"github.com/google/uuid"
)

type (
	NewGuestDevice struct {
		Token     string
		ExpiresAt time.Time
	}

	GuestClaim struct {
		UserID   uuid.UUID
		DeviceID uuid.UUID
		BoardIDs []uuid.UUID
	}
)
