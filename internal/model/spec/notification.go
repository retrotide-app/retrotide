package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewNotification struct {
		UserID        uuid.UUID
		OrgID         uuid.UUID
		Type          model.NotificationType
		ReferenceType string
		ReferenceID   uuid.UUID
		ActorID       uuid.UUID
		Message       string
	}

	NotificationListing struct {
		UserID uuid.UUID
		Limit  int
		Offset int
	}

	NotificationLookup struct {
		UserID uuid.UUID
		ID     int64
	}
)
