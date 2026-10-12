package microsoft

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/identity"
	"retrotide/internal/model"
	"retrotide/internal/settings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	fakeMicrosoft struct {
		t      *testing.T
		server *httptest.Server
		keys   *oidctest.Server
		signer *rsa.PrivateKey

		discoveries   atomic.Int32
		discoveryDown atomic.Bool

		mu         sync.Mutex
		discovered []string
		grants     map[string]url.Values
		plan       tokenPlan
	}

	tokenPlan struct {
		claims      func(claims map[string]any)
		signer      *rsa.PrivateKey
		keyID       string
		omitIDToken bool
	}

	testKeyPair struct {
		issuer   *rsa.PrivateKey
		stranger *rsa.PrivateKey
	}

	discoveryDocument struct {
		Issuer        string   `json:"issuer"`
		Auth          string   `json:"authorization_endpoint"`
		Token         string   `json:"token_endpoint"`
		JWKs          string   `json:"jwks_uri"`
		ResponseTypes []string `json:"response_types_supported"`
		SubjectTypes  []string `json:"subject_types_supported"`
		Algs          []string `json:"id_token_signing_alg_values_supported"`
	}
)

const (
	testClientID      = "6731de76-14a6-49ae-97bc-6eba6914391e"
	testClientSecret  = "test-client-secret"
	testBaseURL       = "https://retrotide.example/"
	testRedirectURL   = "https://retrotide.example/api/v1/auth/microsoft/callback"
	testKeyID         = "fake-entra-key"
	testTenantID      = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	testOtherTenantID = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	testObjectID      = "3d2e4f60-7a8b-4c9d-8e0f-1a2b3c4d5e6f"
	testPairwiseSub   = "AAAAAAAAAAAAAAAAAAAAAIkzqFVrSaSaFHy782bbtaQ"
	testEmail         = "ada@contoso.example"
	testName          = "Ada Lovelace"
	testState         = "state-drawn-by-the-identity-service-0123456789"
	testNonce         = "nonce-drawn-by-the-identity-service-0123456789"
	testVerifier      = "4f1c9a7e2b4d6f8091a3c5e7f9b1d3e5a7c9e1f3b5d7f9a1c3e5b7d9f1a3c5e7"
)

var (
	testKeys = sync.OnceValues(generateTestKeys)
)

func generateTestKeys() (testKeyPair, error) {
	issuer, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return testKeyPair{}, fmt.Errorf("generate the fake entra signing key: %w", err)
	}

	stranger, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return testKeyPair{}, fmt.Errorf("generate the stranger's signing key: %w", err)
	}

	return testKeyPair{issuer: issuer, stranger: stranger}, nil
}

func newFakeMicrosoft(t *testing.T) *fakeMicrosoft {
	t.Helper()

	keys, err := testKeys()
	require.NoError(t, err)

	fake := &fakeMicrosoft{
		t:      t,
		signer: keys.issuer,
		keys:   &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: keys.issuer.Public(), KeyID: testKeyID, Algorithm: oidc.RS256}}},
		grants: make(map[string]url.Values),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{segment}/v2.0/.well-known/openid-configuration", fake.discover)
	mux.Handle("GET /keys", fake.keys)
	mux.HandleFunc("GET /{segment}/oauth2/v2.0/authorize", fake.authorise)
	mux.HandleFunc("POST /{segment}/oauth2/v2.0/token", fake.exchange)

	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)

	return fake
}

func newTestAdapter(t *testing.T, fake *fakeMicrosoft, values map[config.SiteSettingKey]string) *Adapter {
	t.Helper()

	read := func(def *config.SiteSettingDef) (string, error) {
		value, ok := values[def.Key]
		if !ok {
			return "", fmt.Errorf("%w: %s", errUnexpectedSetting, def.Key)
		}

		return value, nil
	}

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, def *config.SiteSettingDef) (string, error) {
		return read(def)
	}).Maybe()
	settingsSvc.EXPECT().GetBool(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, def *config.SiteSettingDef) (bool, error) {
		value, err := read(def)

		return value == "true", err
	}).Maybe()

	return &Adapter{settings: settingsSvc, authority: fake.server.URL, client: fake.server.Client()}
}

func configuredSettings() map[config.SiteSettingKey]string {
	return map[config.SiteSettingKey]string{
		config.SettingMicrosoftClientID.Key:         testClientID,
		config.SettingMicrosoftClientSecret.Key:     testClientSecret,
		config.SettingMicrosoftTenantID.Key:         "",
		config.SettingMicrosoftPersonalAccounts.Key: "false",
		config.SettingBaseURL.Key:                   testBaseURL,
	}
}

func testAuthorisation() identity.Authorisation {
	return identity.Authorisation{State: testState, Nonce: testNonce, Verifier: testVerifier}
}

func startFlow(t *testing.T, fake *fakeMicrosoft, adapter *Adapter) (model.OIDCFlow, string) {
	t.Helper()

	authURL, err := adapter.Begin(t.Context(), testAuthorisation())
	require.NoError(t, err)

	code := fake.visit(t, authURL)

	flow := model.OIDCFlow{Provider: model.ProviderMicrosoft, Purpose: model.OIDCFlowSignIn, State: testState, Nonce: testNonce, CodeVerifier: testVerifier}

	return flow, code
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (f *fakeMicrosoft) issuerFor(tenantID string) string {
	return f.server.URL + "/" + tenantID + "/v2.0"
}

func (f *fakeMicrosoft) issue(plan tokenPlan) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.plan = plan
}

func (f *fakeMicrosoft) discoveredSegments() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.discovered...)
}

func (f *fakeMicrosoft) visit(t *testing.T, authURL string) string {
	t.Helper()

	browser := &http.Client{
		Transport: f.server.Client().Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, authURL, nil)
	require.NoError(t, err)

	resp, err := browser.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	callback, err := resp.Location()
	require.NoError(t, err)
	require.Equal(t, testState, callback.Query().Get("state"))

	return callback.Query().Get("code")
}

func (f *fakeMicrosoft) discover(w http.ResponseWriter, r *http.Request) {
	f.discoveries.Add(1)

	segment := r.PathValue("segment")

	f.mu.Lock()
	f.discovered = append(f.discovered, segment)
	f.mu.Unlock()

	if f.discoveryDown.Load() {
		http.Error(w, "discovery is down", http.StatusServiceUnavailable)
		return
	}

	issuer := f.issuerFor(segment)
	if segment == segmentCommon || segment == segmentOrganizations {
		issuer = f.issuerFor(issuerTemplate)
	}

	f.answer(w, http.StatusOK, discoveryDocument{
		Issuer:        issuer,
		Auth:          f.server.URL + "/" + segment + "/oauth2/v2.0/authorize",
		Token:         f.server.URL + "/" + segment + "/oauth2/v2.0/token",
		JWKs:          f.server.URL + "/keys",
		ResponseTypes: []string{"code", "id_token", "code id_token"},
		SubjectTypes:  []string{"pairwise"},
		Algs:          []string{oidc.RS256},
	})
}

func (f *fakeMicrosoft) authorise(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	callback, err := url.Parse(query.Get("redirect_uri"))
	if err != nil {
		http.Error(w, "the redirect uri does not parse", http.StatusBadRequest)
		return
	}

	code := rand.Text()

	f.mu.Lock()
	f.grants[code] = query
	f.mu.Unlock()

	callback.RawQuery = url.Values{"code": {code}, "state": {query.Get("state")}}.Encode()

	http.Redirect(w, r, callback.String(), http.StatusSeeOther)
}

func (f *fakeMicrosoft) exchange(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" {
		f.answer(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	clientID, ok := authenticatedClient(r)
	if !ok {
		f.answer(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}

	code := r.PostForm.Get("code")

	f.mu.Lock()
	grant, issued := f.grants[code]
	delete(f.grants, code)
	plan := f.plan
	f.mu.Unlock()

	redeemable := issued && grant.Get("client_id") == clientID && grant.Get("redirect_uri") == r.PostForm.Get("redirect_uri")
	if !redeemable || grant.Get("code_challenge_method") != "S256" || grant.Get("code_challenge") != s256(r.PostForm.Get("code_verifier")) {
		f.answer(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}

	now := time.Now()
	claims := map[string]any{
		"iss":                f.issuerFor(testTenantID),
		"aud":                clientID,
		"sub":                testPairwiseSub,
		"iat":                now.Unix(),
		"exp":                now.Add(time.Hour).Unix(),
		"tid":                testTenantID,
		"oid":                testObjectID,
		"email":              testEmail,
		"preferred_username": testEmail,
		"name":               testName,
		"xms_edov":           true,
	}

	if nonce := grant.Get("nonce"); nonce != "" {
		claims["nonce"] = nonce
	}

	if plan.claims != nil {
		plan.claims(claims)
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		f.t.Errorf("encode the fake id token claims: %v", err)
		f.answer(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}

	answer := map[string]any{"access_token": rand.Text(), "token_type": "Bearer", "expires_in": 3599, "scope": grant.Get("scope")}
	if !plan.omitIDToken {
		answer["id_token"] = oidctest.SignIDToken(cmp.Or(plan.signer, f.signer), cmp.Or(plan.keyID, testKeyID), oidc.RS256, string(payload))
	}

	f.answer(w, http.StatusOK, answer)
}

func (f *fakeMicrosoft) answer(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.MarshalWrite(w, body); err != nil {
		f.t.Errorf("write the fake entra answer: %v", err)
	}
}

func authenticatedClient(r *http.Request) (string, bool) {
	clientID, clientSecret := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")

	if escapedID, escapedSecret, ok := r.BasicAuth(); ok {
		var idErr, secretErr error
		clientID, idErr = url.QueryUnescape(escapedID)
		clientSecret, secretErr = url.QueryUnescape(escapedSecret)

		if idErr != nil || secretErr != nil {
			return "", false
		}
	}

	return clientID, clientID == testClientID && clientSecret == testClientSecret
}
