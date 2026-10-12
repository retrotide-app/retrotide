package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	NotificationType string

	Notification struct {
		ID            int64
		UserID        uuid.UUID
		OrgID         *uuid.UUID
		Type          NotificationType
		ReferenceType string
		ReferenceID   *uuid.UUID
		ActorID       *uuid.UUID
		Message       string
		Read          bool
		CreatedAt     time.Time
	}
)

const (
	NotificationOrgInvite          NotificationType = "org_invite"
	NotificationActionItemAssigned NotificationType = "action_item_assigned"
	NotificationPaymentFailed      NotificationType = "payment_failed"
	NotificationTrialEnding        NotificationType = "trial_ending"
	NotificationPaymentRefunded    NotificationType = "payment_refunded"
	NotificationPaymentDisputed    NotificationType = "payment_disputed"
	NotificationOrgDormant         NotificationType = "org_dormant"
)
