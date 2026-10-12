package spec

import (
	"time"

	"github.com/google/uuid"
)

type (
	NewSession struct {
		Token     string
		UserID    uuid.UUID
		ExpiresAt time.Time
	}

	SessionGrant struct {
		UserID   uuid.UUID
		Remember bool
	}

	SessionDeletionExcept struct {
		UserID    uuid.UUID
		KeepToken string
	}
)
