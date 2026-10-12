package spec

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewOIDCFlow struct {
		TokenHash        string
		Provider         model.IdentityProvider
		Purpose          model.OIDCFlowPurpose
		State            string
		Nonce            string
		CodeVerifier     string
		UserID           uuid.UUID
		SessionTokenHash string
		ReturnTo         string
		ThenLink         model.IdentityProvider
		ExpiresAt        time.Time
	}

	OIDCFlowKey struct {
		TokenHash string
		Provider  model.IdentityProvider
	}

	IdentityFlowStart struct {
		Provider           model.IdentityProvider
		Purpose            model.OIDCFlowPurpose
		UserID             uuid.UUID
		SessionToken       string
		CurrentPassword    string
		ReturnTo           string
		ReauthenticateWith model.IdentityProvider
	}

	IdentityCallback struct {
		Provider           model.IdentityProvider
		FlowToken          string
		State              string
		Code               string
		ProviderError      string
		SessionToken       string
		TrustedDeviceToken string
		IP                 string
	}
)
