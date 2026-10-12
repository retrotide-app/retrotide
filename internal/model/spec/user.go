package spec

import (
	"time"

	"github.com/google/uuid"
)

type (
	NewUser struct {
		Email         string
		PasswordHash  *string
		DisplayName   string
		EmailVerified bool
	}

	UserEmailFilter struct {
		Email         string
		ExcludeUserID uuid.UUID
	}

	UserPasswordHashUpdate struct {
		UserID       uuid.UUID
		PasswordHash string
	}

	UserDisplayNameUpdate struct {
		UserID      uuid.UUID
		DisplayName string
	}

	UserVerifiedEmailUpdate struct {
		UserID uuid.UUID
		Email  string
	}

	UserLastSeenIPUpdate struct {
		UserID uuid.UUID
		IP     string
	}

	UserListing struct {
		Search string
		Limit  int
		Offset int
	}

	UserBan struct {
		UserID   uuid.UUID
		BannedBy uuid.UUID
		Reason   string
	}

	UserLock struct {
		UserID   uuid.UUID
		LockedBy uuid.UUID
		Reason   string
	}

	NewRegistration struct {
		User                  NewUser
		FirstAccount          bool
		VerificationHash      string
		VerificationExpiresAt time.Time
	}

	PasswordReset struct {
		TokenHash    string
		PasswordHash string
	}

	PasswordChange struct {
		UserID       uuid.UUID
		PasswordHash string
		KeepToken    string
	}

	NewAccount struct {
		Email       string
		Password    string
		DisplayName string
	}

	UserCredentials struct {
		Email    string
		Password string
	}

	PasswordSignIn struct {
		UserCredentials

		Remember           bool
		TrustedDeviceToken string
	}

	UserPasswordConfirmation struct {
		UserID   uuid.UUID
		Password string
	}

	AccountRegistration struct {
		Email       string
		Password    string
		DisplayName string
	}

	PasswordResetCompletion struct {
		Token       string
		NewPassword string
	}

	PasswordUpdate struct {
		UserID          uuid.UUID
		CurrentPassword string
		NewPassword     string
		KeepToken       string
	}

	EmailChange struct {
		UserID   uuid.UUID
		Email    string
		Password string
	}
)
