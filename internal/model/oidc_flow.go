package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	OIDCFlowPurpose string

	OIDCFlow struct {
		TokenHash        string
		Provider         IdentityProvider
		Purpose          OIDCFlowPurpose
		State            string
		Nonce            string
		CodeVerifier     string
		UserID           *uuid.UUID
		SessionTokenHash *string
		ReturnTo         *string
		ThenLink         *IdentityProvider
		CreatedAt        time.Time
		ExpiresAt        time.Time
	}
)

const (
	OIDCFlowSignIn         OIDCFlowPurpose = "sign_in"
	OIDCFlowLink           OIDCFlowPurpose = "link"
	OIDCFlowReauthenticate OIDCFlowPurpose = "reauthenticate"
)
