package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	Session struct {
		UserID    uuid.UUID
		CreatedAt time.Time
		ExpiresAt time.Time
	}
)
