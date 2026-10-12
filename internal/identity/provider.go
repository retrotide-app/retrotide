package identity

import (
	"context"

	"retrotide/internal/config"
	"retrotide/internal/model"
)

type (
	Provider interface {
		ID() model.IdentityProvider
		Name() string
		Enabled(ctx context.Context) (bool, error)
		Begin(ctx context.Context, authorisation Authorisation) (string, error)
		Complete(ctx context.Context, flow model.OIDCFlow, code string) (*model.ProviderIdentity, error)
	}

	ConfigurationChecker interface {
		ConfiguredWith(values map[config.SiteSettingKey]string) bool
	}

	Authorisation struct {
		State    string
		Nonce    string
		Verifier string
	}
)
