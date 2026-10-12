package google

import (
	"cmp"
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/identity"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/settings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type (
	Adapter struct {
		settings settings.Service
		issuer   string
		client   *http.Client

		mu         sync.Mutex
		discovered *oidc.Provider
		generation uint64
	}

	googleClaims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		HostedDomain  string `json:"hd"`
		Name          string `json:"name"`
	}
)

const (
	displayName   = "Google"
	googleIssuer  = "https://accounts.google.com"
	callbackPath  = "/api/v1/auth/" + string(model.ProviderGoogle) + "/callback"
	clientTimeout = 10 * time.Second
	idTokenField  = "id_token"
)

var (
	scopes = []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile}

	gmailDomains = []string{"gmail.com", "googlemail.com"}

	credentials = []*config.SiteSettingDef{config.SettingGoogleClientID, config.SettingGoogleClientSecret}

	_ identity.Provider             = (*Adapter)(nil)
	_ settings.Listener             = (*Adapter)(nil)
	_ identity.ConfigurationChecker = (*Adapter)(nil)
)

func New(settingsSvc settings.Service) *Adapter {
	return &Adapter{
		settings: settingsSvc,
		issuer:   googleIssuer,
		client:   &http.Client{Timeout: clientTimeout},
	}
}

func (a *Adapter) ID() model.IdentityProvider {
	return model.ProviderGoogle
}

func (a *Adapter) Name() string {
	return displayName
}

func (a *Adapter) Enabled(ctx context.Context) (bool, error) {
	for _, def := range credentials {
		value, err := a.settings.Get(ctx, def)
		if err != nil {
			return false, fmt.Errorf("read %s: %w", def.Key, err)
		}

		if strings.TrimSpace(value) == "" {
			return false, nil
		}
	}

	return true, nil
}

func (a *Adapter) ConfiguredWith(values map[config.SiteSettingKey]string) bool {
	for _, def := range credentials {
		if strings.TrimSpace(values[def.Key]) == "" {
			return false
		}
	}

	return true
}

func (a *Adapter) Begin(ctx context.Context, authorisation identity.Authorisation) (string, error) {
	ctx = oidc.ClientContext(ctx, a.client)

	cfg, _, err := a.configure(ctx)
	if err != nil {
		return "", err
	}

	return cfg.AuthCodeURL(authorisation.State, oauth2.S256ChallengeOption(authorisation.Verifier), oidc.Nonce(authorisation.Nonce), oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

func (a *Adapter) Complete(ctx context.Context, flow model.OIDCFlow, code string) (*model.ProviderIdentity, error) {
	ctx = oidc.ClientContext(ctx, a.client)

	cfg, provider, err := a.configure(ctx)
	if err != nil {
		return nil, err
	}

	exchanged, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(flow.CodeVerifier))
	if err != nil {
		return nil, fmt.Errorf("exchange the google authorisation code: %w", err)
	}

	rawIDToken, ok := exchanged.Extra(idTokenField).(string)
	if !ok || rawIDToken == "" {
		return nil, identity.ErrNoIDToken
	}

	idToken, err := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify the google id token: %w", err)
	}

	if flow.Nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		return nil, identity.ErrNonceMismatch
	}

	var claims googleClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("read the google id token claims: %w", err)
	}

	gmail, aliases := gmailAliases(claims.Email)

	return &model.ProviderIdentity{
		Provider:           model.ProviderGoogle,
		Subject:            idToken.Subject,
		Email:              claims.Email,
		EmailAliases:       aliases,
		EmailVerified:      claims.EmailVerified,
		EmailAuthoritative: gmail || claims.HostedDomain != "",
		Name:               claims.Name,
	}, nil
}

func gmailAliases(email string) (bool, []string) {
	local, domain, found := strings.CutLast(email, "@")
	if !found || !slices.ContainsFunc(gmailDomains, func(gmailDomain string) bool {
		return strings.EqualFold(gmailDomain, domain)
	}) {
		return false, nil
	}

	aliases := make([]string, 0, len(gmailDomains)-1)
	for _, gmailDomain := range gmailDomains {
		if !strings.EqualFold(gmailDomain, domain) {
			aliases = append(aliases, local+"@"+gmailDomain)
		}
	}

	return true, aliases
}

func (a *Adapter) OnSettingChanged(key config.SiteSettingKey, _ string) {
	changed := slices.ContainsFunc(credentials, func(def *config.SiteSettingDef) bool {
		return def.Key == key
	})
	if !changed {
		return
	}

	a.mu.Lock()
	a.discovered = nil
	a.generation++
	a.mu.Unlock()

	logger.Ctx(context.Background()).Info().Str("key", string(key)).Msg("google sign-in settings changed, so google is discovered again on its next use")
}

func (a *Adapter) configure(ctx context.Context) (*oauth2.Config, *oidc.Provider, error) {
	clientID, err := a.settings.Get(ctx, config.SettingGoogleClientID)
	if err != nil {
		return nil, nil, err
	}

	clientSecret, err := a.settings.Get(ctx, config.SettingGoogleClientSecret)
	if err != nil {
		return nil, nil, err
	}

	if clientID == "" || clientSecret == "" {
		return nil, nil, identity.ErrProviderNotConfigured
	}

	baseURL, err := a.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return nil, nil, err
	}

	provider, err := a.provider(ctx)
	if err != nil {
		return nil, nil, err
	}

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  strings.TrimRight(baseURL, "/") + callbackPath,
		Scopes:       scopes,
	}

	return cfg, provider, nil
}

func (a *Adapter) provider(ctx context.Context) (*oidc.Provider, error) {
	a.mu.Lock()
	cached, generation := a.discovered, a.generation
	a.mu.Unlock()

	if cached != nil {
		return cached, nil
	}

	discovered, err := oidc.NewProvider(ctx, a.issuer)
	if err != nil {
		return nil, fmt.Errorf("%w: discover %s: %w", identity.ErrProviderUnavailable, a.issuer, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.generation == generation {
		a.discovered = cmp.Or(a.discovered, discovered)
	}

	return cmp.Or(a.discovered, discovered), nil
}
