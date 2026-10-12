package dto

type (
	TwoFactorStatusResponse struct {
		Enabled bool `json:"enabled"`
	}

	TwoFactorEnrolmentRequest struct {
		CurrentPassword string `json:"current_password"`
	}

	TwoFactorEnrolmentResponse struct {
		Secret string `json:"secret"`
		URI    string `json:"otpauth_uri"`
	}

	TwoFactorCodeRequest struct {
		Code string `json:"code"`
	}

	TwoFactorVerifyRequest struct {
		Code        string `json:"code"`
		TrustDevice bool   `json:"trust_device"`
	}

	TwoFactorRecoveryCodesResponse struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}

	TwoFactorChallengeResponse struct {
		TwoFactorRequired bool `json:"two_factor_required"`
	}

	TwoFactorVerifiedResponse struct {
		User     UserResponse `json:"user"`
		Location string       `json:"location,omitempty"`
	}
)
