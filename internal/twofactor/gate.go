package twofactor

import (
	"context"
	"time"

	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	Gate interface {
		Enabled(ctx context.Context, userID uuid.UUID) (bool, error)
		Challenge(ctx context.Context, pending spec.TwoFactorPending) (string, error)
	}
)

const (
	ChallengeCookieName     = "rt_2fa"
	ChallengeTTL            = 5 * time.Minute
	MaxChallengeAttempts    = 5
	PagePath                = "/two-factor"
	TrustedDeviceCookieName = "rt_trusted_device"
	TrustedDeviceTTL        = 30 * 24 * time.Hour
)
