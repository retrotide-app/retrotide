package microsoft

import (
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
		settings  settings.Service
		authority string
		client    *http.Client

		mu         sync.Mutex
		discovered map[string]*oidc.Provider
		generation uint64
	}

	configuration struct {
		oauth    *oauth2.Config
		provider *oidc.Provider
		tenancy  tenancy
	}
)

const (
	displayName        = "Microsoft"
	microsoftAuthority = "https://login.microsoftonline.com"
	personalTenantID   = "9188040d-6c67-4c5b-b112-36a304b66dad"
	callbackPath       = "/api/v1/auth/" + string(model.ProviderMicrosoft) + "/callback"
	clientTimeout      = 10 * time.Second
	idTokenField       = "id_token"
	issuerTemplate     = "{tenantid}"
	versionPath        = "/v2.0"
)

var (
	scopes = []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile}

	credentials = []*config.SiteSettingDef{config.SettingMicrosoftClientID, config.SettingMicrosoftClientSecret}

	discoverySettings = []*config.SiteSettingDef{
		config.SettingMicrosoftClientID,
		config.SettingMicrosoftClientSecret,
		config.SettingMicrosoftTenantID,
		config.SettingMicrosoftPersonalAccounts,
	}

	_ identity.Provider             = (*Adapter)(nil)
	_ settings.Listener             = (*Adapter)(nil)
	_ identity.ConfigurationChecker = (*Adapter)(nil)
)

func New(settingsSvc settings.Service) *Adapter {
	return &Adapter{
		settings:  settingsSvc,
		authority: microsoftAuthority,
		client:    &http.Client{Timeout: clientTimeout},
	}
}

func (a *Adapter) ID() model.IdentityProvider {
	return model.ProviderMicrosoft
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

	cfg, err := a.configure(ctx)
	if err != nil {
		return "", err
	}

	return cfg.oauth.AuthCodeURL(authorisation.State, oauth2.S256ChallengeOption(authorisation.Verifier), oidc.Nonce(authorisation.Nonce), oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

func (a *Adapter) Complete(ctx context.Context, flow model.OIDCFlow, code string) (*model.ProviderIdentity, error) {
	ctx = oidc.ClientContext(ctx, a.client)

	cfg, err := a.configure(ctx)
	if err != nil {
		return nil, err
	}

	exchanged, err := cfg.oauth.Exchange(ctx, code, oauth2.VerifierOption(flow.CodeVerifier))
	if err != nil {
		return nil, fmt.Errorf("exchange the microsoft authorisation code: %w", err)
	}

	rawIDToken, ok := exchanged.Extra(idTokenField).(string)
	if !ok || rawIDToken == "" {
		return nil, identity.ErrNoIDToken
	}

	idToken, err := cfg.provider.Verifier(&oidc.Config{ClientID: cfg.oauth.ClientID, SkipIssuerCheck: true}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify the microsoft id token: %w", err)
	}

	if flow.Nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		return nil, identity.ErrNonceMismatch
	}

	var claims microsoftClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("read the microsoft id token claims: %w", err)
	}

	if err := cfg.tenancy.admit(a.authority, claims); err != nil {
		return nil, err
	}

	email, fromEmailClaim := claims.address()

	return &model.ProviderIdentity{
		Provider:           model.ProviderMicrosoft,
		Subject:            claims.subject(),
		Email:              email,
		EmailVerified:      email != "",
		EmailAuthoritative: fromEmailClaim && bool(claims.DomainOwnerVerify),
		Name:               claims.Name,
	}, nil
}

func (a *Adapter) OnSettingChanged(key config.SiteSettingKey, _ string) {
	changed := slices.ContainsFunc(discoverySettings, func(def *config.SiteSettingDef) bool {
		return def.Key == key
	})
	if !changed {
		return
	}

	a.mu.Lock()
	a.discovered = nil
	a.generation++
	a.mu.Unlock()

	logger.Ctx(context.Background()).Info().Str("key", string(key)).Msg("microsoft sign-in settings changed, so microsoft is discovered again on its next use")
}

func (a *Adapter) configure(ctx context.Context) (*configuration, error) {
	clientID, err := a.settings.Get(ctx, config.SettingMicrosoftClientID)
	if err != nil {
		return nil, err
	}

	clientSecret, err := a.settings.Get(ctx, config.SettingMicrosoftClientSecret)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return nil, identity.ErrProviderNotConfigured
	}

	baseURL, err := a.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return nil, err
	}

	tenantID, err := a.settings.Get(ctx, config.SettingMicrosoftTenantID)
	if err != nil {
		return nil, err
	}

	personal, err := a.settings.GetBool(ctx, config.SettingMicrosoftPersonalAccounts)
	if err != nil {
		return nil, err
	}

	admitted := tenancy{tenantID: strings.ToLower(strings.TrimSpace(tenantID)), personal: personal}

	provider, err := a.provider(ctx, admitted)
	if err != nil {
		return nil, err
	}

	oauth := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  strings.TrimRight(baseURL, "/") + callbackPath,
		Scopes:       scopes,
	}

	return &configuration{oauth: oauth, provider: provider, tenancy: admitted}, nil
}

func (a *Adapter) provider(ctx context.Context, admitted tenancy) (*oidc.Provider, error) {
	segment, pinned := admitted.segment()

	a.mu.Lock()
	cached, generation := a.discovered[segment], a.generation
	a.mu.Unlock()

	if cached != nil {
		return cached, nil
	}

	discoveryURL := a.authority + "/" + segment + versionPath
	discoveryCtx := ctx
	if !pinned {
		discoveryCtx = oidc.InsecureIssuerURLContext(ctx, a.authority+"/"+issuerTemplate+versionPath)
	}

	discovered, err := oidc.NewProvider(discoveryCtx, discoveryURL)
	if err != nil {
		return nil, fmt.Errorf("%w: discover %s: %w", identity.ErrProviderUnavailable, discoveryURL, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.generation != generation {
		return discovered, nil
	}

	if existing := a.discovered[segment]; existing != nil {
		return existing, nil
	}

	if a.discovered == nil {
		a.discovered = make(map[string]*oidc.Provider)
	}

	a.discovered[segment] = discovered

	return discovered, nil
}
