package dto

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NotifyParams struct {
		RecipientID   uuid.UUID
		OrgID         uuid.UUID
		Type          model.NotificationType
		ReferenceType string
		ReferenceID   uuid.UUID
		ActorID       uuid.UUID
		Message       string
		Link          string
	}

	NotificationResponse struct {
		ID            int64                  `json:"id"`
		Type          model.NotificationType `json:"type"`
		OrgID         *uuid.UUID             `json:"org_id,omitempty"`
		ReferenceType string                 `json:"reference_type,omitempty"`
		ReferenceID   *uuid.UUID             `json:"reference_id,omitempty"`
		Message       string                 `json:"message,omitempty"`
		Read          bool                   `json:"read"`
		CreatedAt     string                 `json:"created_at"`
	}

	NotificationListResponse struct {
		Notifications []NotificationResponse `json:"notifications"`
		Total         int                    `json:"total"`
		Limit         int                    `json:"limit"`
		Offset        int                    `json:"offset"`
	}

	UnreadCountResponse struct {
		Count int `json:"count"`
	}
)

func NewNotificationResponse(n model.Notification) NotificationResponse {
	return NotificationResponse{
		ID:            n.ID,
		Type:          n.Type,
		OrgID:         n.OrgID,
		ReferenceType: n.ReferenceType,
		ReferenceID:   n.ReferenceID,
		Message:       n.Message,
		Read:          n.Read,
		CreatedAt:     n.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}
