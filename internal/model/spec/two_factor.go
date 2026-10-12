package spec

import (
	"time"

	"github.com/google/uuid"
)

type (
	TwoFactorEnrolment struct {
		UserID uuid.UUID
		Secret string
	}

	TwoFactorStepUse struct {
		UserID uuid.UUID
		Step   int64
	}

	TwoFactorRecoveryCodeUse struct {
		UserID   uuid.UUID
		CodeHash string
	}

	TwoFactorActivation struct {
		UserID     uuid.UUID
		Step       int64
		CodeHashes []string
		KeepToken  string
	}

	NewTwoFactorChallenge struct {
		TokenHash string
		UserID    uuid.UUID
		ReturnTo  string
		Remember  bool
		ExpiresAt time.Time
	}

	NewTwoFactorTrustedDevice struct {
		TokenHash string
		UserID    uuid.UUID
		ExpiresAt time.Time
	}

	TwoFactorTrustedDeviceCheck struct {
		TokenHash string
		UserID    uuid.UUID
	}

	TwoFactorChallengeAttempt struct {
		TokenHash   string
		MaxAttempts int
	}

	TwoFactorPending struct {
		UserID             uuid.UUID
		ReturnTo           string
		Remember           bool
		TrustedDeviceToken string
	}

	TwoFactorEnrolmentStart struct {
		UserID          uuid.UUID
		CurrentPassword string
		IP              string
	}

	TwoFactorCodeEntry struct {
		UserID uuid.UUID
		Code   string
		IP     string
	}

	TwoFactorConfirmation struct {
		TwoFactorCodeEntry

		KeepToken string
	}

	TwoFactorVerification struct {
		ChallengeToken string
		Code           string
		TrustDevice    bool
	}
)
