package spec

import (
	"time"

	"github.com/google/uuid"
)

type (
	NewInvitation struct {
		Email     string
		TokenHash string
		ExpiresAt time.Time
	}

	InvitationRenewal struct {
		ID        uuid.UUID
		TokenHash string
		ExpiresAt time.Time
	}

	InvitationListing struct {
		Limit  int
		Offset int
	}

	InvitationRedemption struct {
		TokenHash string
		User      NewUser
	}

	InvitationAcceptance struct {
		Token       string
		DisplayName string
		Password    string
		IP          string
	}

	StaffInvitation struct {
		StaffAction

		Email string
	}

	StaffInvitationAction struct {
		StaffAction

		InvitationID uuid.UUID
	}
)
