package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"retrotide/internal/auth"
	"retrotide/internal/config"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/cookie"
	"retrotide/internal/dto"
	"retrotide/internal/identity"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	identityHarness struct {
		*testutil.Harness
		identity *identity.MockService
		users    *user.MockService
		auth     *auth.MockService
	}
)

const (
	flowToken          = "5d2b8f4e1a7c9e3b6d0f2a4c8e1b3d5f7a9c0e2b4d6f8a1c3e5b7d9f0a2c4e6b"
	googleAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth?client_id=client&state=state"
	googleStartPath    = "/api/v1/auth/google/start"
	googleCallbackPath = "/api/v1/auth/google/callback"
	flowCookieName     = "__Host-rt_oidc"
	flowTTLSeconds     = 600
	clientHeaderName   = "X-RetroTide-Client"
	clientHeaderValue  = "web"
	callbackState      = "q8Zt3vW0yXbN5mKc2hLs9dPf7gRj4aUe1iOw6nTy0Bx"
	callbackCode       = "4/0AbCdEfGhIjKlMnOpQrStUvWxYz"
	callbackURL        = googleCallbackPath + "?state=" + callbackState + "&code=" + callbackCode
	callbackIP         = "198.51.100.7"
	failedLocation     = "/login?error=failed&provider=google"
	unknownProvider    = "nowhere"
	nextFlowToken      = "0e2c4a6f8d1b3e5c7a9f0d2b4e6c8a1f3d5b7e9c0a2f4d6b8e1c3a5f7d9b0e2c"
	exampleAuthURL     = "https://id.example.test/authorize?client_id=retrotide&state=next"

	exampleProvider model.IdentityProvider = "example"
)

func newIdentityHarness(t *testing.T, baseURL string) *identityHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(baseURL)

	return mountIdentityRoutes(t, h)
}

func mountIdentityRoutes(t *testing.T, h *testutil.Harness) *identityHarness {
	t.Helper()

	h.SettingsService.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(false, nil).Maybe()
	h.Engine.Use(middleware.RequireClientHeader())

	ih := &identityHarness{Harness: h, identity: identity.NewMockService(t), users: user.NewMockService(t), auth: auth.NewMockService(t)}

	s := &Service{
		IdentityService:  ih.identity,
		UserService:      ih.users,
		AuthService:      ih.auth,
		AuthzService:     h.Authz,
		Sessions:         h.Sessions,
		FlowCookies:      cookie.NewBuilder(h.SettingsService, identity.FlowCookieName, cookie.Fixed(identity.FlowTTL)),
		TwoFactorCookies: cookie.NewBuilder(h.SettingsService, twofactor.ChallengeCookieName, cookie.Fixed(twofactor.ChallengeTTL)),
		TrustedDevices:   cookie.NewBuilder(h.SettingsService, twofactor.TrustedDeviceCookieName, cookie.Fixed(twofactor.TrustedDeviceTTL)),
		Guards:           h.Guards,
	}

	api := h.Engine.Group("/api/v1")
	RegisterRoutes(api, s.getAllIdentityRoutes())
	RegisterRoutes(api, s.getAllAuthRoutes())

	return ih
}

func newRecordingIdentityHarness(t *testing.T) (*identityHarness, *[]*gin.Error) {
	t.Helper()

	recorded := &[]*gin.Error{}

	base := testutil.NewHarness(t)
	base.UseBaseURL(testutil.TestBaseURL)
	base.Engine.Use(func(c *gin.Context) {
		c.Next()

		*recorded = slices.Clone(c.Errors)
	})

	return mountIdentityRoutes(t, base), recorded
}

func flowCookie() *http.Cookie {
	return &http.Cookie{Name: flowCookieName, Value: flowToken}
}

func signInCallback() spec.IdentityCallback {
	return spec.IdentityCallback{Provider: model.ProviderGoogle, FlowToken: flowToken, State: callbackState, Code: callbackCode, IP: callbackIP}
}

func assertFlowCookie(t *testing.T, rec *httptest.ResponseRecorder, name string, secure bool) {
	t.Helper()

	cookie := testutil.ResponseCookie(t, rec, name)
	assert.Equal(t, flowToken, cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.Empty(t, cookie.Domain)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, secure, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, flowTTLSeconds, cookie.MaxAge)
}

func assertFlowCookieCleared(t *testing.T, rec *httptest.ResponseRecorder, name string, secure bool) {
	t.Helper()

	cookie := testutil.ResponseCookie(t, rec, name)
	assert.Empty(t, cookie.Value)
	assert.Equal(t, -1, cookie.MaxAge)
	assert.Equal(t, "/", cookie.Path)
	assert.Equal(t, secure, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, location, rec.Header().Get("Location"))
	assert.NotContains(t, rec.Header().Get("Content-Type"), "json")
}

func sessionCookieSet(rec *httptest.ResponseRecorder, name string) bool {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return true
		}
	}

	return false
}

func TestIdentityController_SignInStartSetsTheHostPrefixedFlowCookie(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Start(mock.Anything, spec.IdentityFlowStart{Provider: model.ProviderGoogle, Purpose: model.OIDCFlowSignIn, ReturnTo: "/invite"}).Return(&identity.Started{AuthURL: googleAuthURL, FlowToken: flowToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in", ReturnTo: "/invite"}).WithHeader(clientHeaderName, clientHeaderValue).Record()

	// then the browser that started the flow is the only one that can finish it
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"url":"`+googleAuthURL+`"}`, rec.Body.String())
	assertFlowCookie(t, rec, flowCookieName, true)
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
}

func TestIdentityController_StartUsesTheBareFlowCookieNameOverHTTP(t *testing.T) {
	// given local development over http
	h := newIdentityHarness(t, testutil.TestInsecureBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(&identity.Started{AuthURL: googleAuthURL, FlowToken: flowToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assertFlowCookie(t, rec, "rt_oidc", false)
}

func TestIdentityController_StartNeedsTheClientHeader(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).Record()

	// then the strict identity mock sees no call, so no flow is written
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"missing client header"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestIdentityController_BothRoutesAnswerNotFoundForAProviderThatIsOffOrUnknown(t *testing.T) {
	tests := []struct {
		name     string
		provider model.IdentityProvider
		request  func(h *identityHarness) *testutil.Request
	}{
		{name: "the start of a switched-off provider", provider: model.ProviderGoogle, request: func(h *identityHarness) *testutil.Request {
			return h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue)
		}},
		{name: "the callback of a switched-off provider", provider: model.ProviderGoogle, request: func(h *identityHarness) *testutil.Request {
			return h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie())
		}},
		{name: "the start of an unknown provider", provider: unknownProvider, request: func(h *identityHarness) *testutil.Request {
			return h.NewRequest(http.MethodPost, "/api/v1/auth/"+unknownProvider+"/start").WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue)
		}},
		{name: "the callback of an unknown provider", provider: unknownProvider, request: func(h *identityHarness) *testutil.Request {
			return h.NewRequest(http.MethodGet, "/api/v1/auth/"+unknownProvider+"/callback?state="+callbackState+"&code="+callbackCode).WithCookie(flowCookie())
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the identity service knows no such provider, or the provider's own settings are empty
			h := newIdentityHarness(t, testutil.TestBaseURL)
			h.identity.EXPECT().Enabled(mock.Anything, tc.provider).Return(false, nil)

			// when
			rec := tc.request(h).Record()

			// then the route looks like one that does not exist, and neither Start nor Finish runs
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestIdentityController_APrivateSiteAnswersNotFoundOnEveryProviderRoute(t *testing.T) {
	tests := []struct {
		name    string
		request func(h *testutil.Harness) *testutil.Request
	}{
		{name: "a sign-in start", request: func(h *testutil.Harness) *testutil.Request {
			return h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue)
		}},
		{name: "a link start from account settings", request: func(h *testutil.Harness) *testutil.Request {
			return h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: authPassword}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(h.SignIn(uuid.New()))
		}},
		{name: "the callback", request: func(h *testutil.Harness) *testutil.Request {
			return h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie())
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the real identity service over a private site and a provider whose own switch is never asked
			h := testutil.NewHarness(t)
			h.UseBaseURL(testutil.TestBaseURL)
			h.SettingsService.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
			h.Engine.Use(middleware.RequireClientHeader())

			provider := identity.NewMockProvider(t)
			provider.EXPECT().ID().Return(model.ProviderGoogle)

			identitySvc, err := identity.NewService(identity.Deps{Providers: []identity.Provider{provider}, Settings: h.SettingsService})
			require.NoError(t, err)

			s := &Service{IdentityService: identitySvc, Sessions: h.Sessions, FlowCookies: cookie.NewBuilder(h.SettingsService, identity.FlowCookieName, cookie.Fixed(identity.FlowTTL)), Guards: h.Guards}
			RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllIdentityRoutes())

			// when
			rec := tc.request(h).Record()

			// then a private site signs in by password only, so neither route exists and no flow cookie is set
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestIdentityController_TheRouteNamesTheProvider(t *testing.T) {
	// given a second provider behind the same generic routes
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, exampleProvider).Return(true, nil).Twice()
	h.identity.EXPECT().Start(mock.Anything, spec.IdentityFlowStart{Provider: exampleProvider, Purpose: model.OIDCFlowSignIn}).Return(&identity.Started{AuthURL: googleAuthURL, FlowToken: flowToken}, nil)
	arrived := signInCallback()
	arrived.Provider = exampleProvider
	h.identity.EXPECT().Finish(mock.Anything, arrived).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/login?error=expired&provider=example"}, nil)

	// when
	started := h.NewRequest(http.MethodPost, "/api/v1/auth/example/start").WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue).Record()
	finished := h.NewRequest(http.MethodGet, "/api/v1/auth/example/callback?state="+callbackState+"&code="+callbackCode).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()

	// then both halves of the flow reach the service under that provider's id, and share the one flow cookie
	require.Equal(t, http.StatusOK, started.Code)
	assertFlowCookie(t, started, flowCookieName, true)
	assertRedirect(t, finished, testutil.TestBaseURL+"/login?error=expired&provider=example")
	assertFlowCookieCleared(t, finished, flowCookieName, true)
}

func TestIdentityController_StartReportsASwitchThatCannotBeRead(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(false, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue).Record()

	// then a blip is never read as the feature being off
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to start the sign-in"}`, rec.Body.String())
}

func TestIdentityController_StartRefusesABodyThatIsNotJSON(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithRawBody("{", "application/json").WithHeader(clientHeaderName, clientHeaderValue).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid request body"}`, rec.Body.String())
}

func TestIdentityController_StartMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{name: "the settings emptied meanwhile", err: fmt.Errorf("begin the google sign-in: %w", identity.ErrProviderNotConfigured), status: http.StatusNotFound, want: `{"error":"the identity provider is not configured"}`},
		{name: "a failed discovery", err: fmt.Errorf("begin the google sign-in: %w", errDiscoveryDown), status: http.StatusServiceUnavailable, want: `{"error":"the identity provider could not be discovered"}`},
		{name: "an unknown purpose", err: fmt.Errorf("%w: %q", identity.ErrUnknownPurpose, "reauthenticate"), status: http.StatusBadRequest, want: `{"error":"the purpose must be sign_in or link"}`},
		{name: "a flow that cannot be stored", err: errServiceDown, status: http.StatusInternalServerError, want: `{"error":"failed to start the sign-in"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newIdentityHarness(t, testutil.TestBaseURL)
			h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
			h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue).Record()

			// then the start is a fetch, so it answers JSON, and no flow cookie is handed out
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
			assert.NotContains(t, rec.Body.String(), "database down")
		})
	}
}

func TestIdentityController_AFailedDiscoveryAtTheStartRecordsItsCauseOnce(t *testing.T) {
	// given
	h, recorded := newRecordingIdentityHarness(t)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("begin the google sign-in: %w", errDiscoveryDown))

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue).Record()

	// then ErrorResponder logs the cause, and the adapter leaves the logging to it
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"error":"the identity provider could not be discovered"}`, rec.Body.String())
	require.Len(t, *recorded, 1)
	assert.ErrorIs(t, (*recorded)[0].Err, errServiceDown)
}

func TestIdentityController_AnExpectedStartRefusalRecordsNoCause(t *testing.T) {
	// given
	h, recorded := newRecordingIdentityHarness(t)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("begin the google sign-in: %w", identity.ErrProviderNotConfigured))

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue).Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, *recorded)
}

func TestIdentityController_LinkStartBindsTheFlowToTheSignedInSession(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
	h.identity.EXPECT().Start(mock.Anything, spec.IdentityFlowStart{Provider: model.ProviderGoogle, Purpose: model.OIDCFlowLink, UserID: userID, SessionToken: testutil.TestSessionToken, CurrentPassword: authPassword, ReturnTo: "/account"}).Return(&identity.Started{AuthURL: googleAuthURL, FlowToken: flowToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: authPassword, ReturnTo: "/account"}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assertFlowCookie(t, rec, flowCookieName, true)
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
}

func TestIdentityController_LinkStartNeedsASignedInUser(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: authPassword}).WithHeader(clientHeaderName, clientHeaderValue).Record()

	// then
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"linking a sign-in method needs a signed-in account"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestIdentityController_LinkStartNeedsTheCurrentPassword(t *testing.T) {
	// given an account with a password, which a reauthentication provider cannot stand in for
	h := newIdentityHarness(t, testutil.TestBaseURL)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", ReauthenticateWith: string(exampleProvider)}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).Record()

	// then the service is never asked, which the strict mock would report
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"current password is required"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestIdentityController_APasswordlessLinkStartNeedsAReauthenticationProvider(t *testing.T) {
	// given an account that signs in only through a provider
	h := newIdentityHarness(t, testutil.TestBaseURL)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	account := signedInAccount(userID)
	account.HasPassword = false
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(account, nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: authPassword}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).Record()

	// then a password it does not have cannot confirm the link
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"confirm the link by signing in with a sign-in method your account already has","code":"reauthentication_required"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestIdentityController_APasswordlessLinkStartHandsTheReauthenticationProviderToTheService(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	account := signedInAccount(userID)
	account.HasPassword = false
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(account, nil)
	h.identity.EXPECT().Start(mock.Anything, spec.IdentityFlowStart{Provider: model.ProviderGoogle, Purpose: model.OIDCFlowLink, UserID: userID, SessionToken: testutil.TestSessionToken, ReturnTo: "/account", ReauthenticateWith: exampleProvider}).Return(&identity.Started{AuthURL: exampleAuthURL, FlowToken: flowToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", ReturnTo: "/account", ReauthenticateWith: string(exampleProvider)}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).Record()

	// then the browser goes to the provider the account already holds
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"url":"`+exampleAuthURL+`"}`, rec.Body.String())
	assertFlowCookie(t, rec, flowCookieName, true)
}

func TestIdentityController_APasswordlessLinkStartChargesTheAccountsCredentialBucket(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	account := signedInAccount(userID)
	account.HasPassword = false
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(account, nil)
	h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(nil, identity.ErrReauthenticationRequired).Times(middleware.CredentialAttemptsPerMinute)

	for i := range middleware.CredentialAttemptsPerMinute {
		rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", ReauthenticateWith: string(exampleProvider)}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).WithRemoteAddr(fmt.Sprintf("198.51.100.%d:4444", i+1)).Record()
		require.Equal(t, http.StatusBadRequest, rec.Code, "attempt %d", i+1)
	}

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", ReauthenticateWith: string(exampleProvider)}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).WithRemoteAddr("203.0.113.9:4444").Record()

	// then
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestIdentityController_LinkStartMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{name: "a wrong current password", err: user.ErrIncorrectPassword, status: http.StatusForbidden, want: `{"error":"the current password is incorrect","code":"incorrect_password"}`},
		{name: "an unverified email address", err: identity.ErrAccountUnverified, status: http.StatusForbidden, want: `{"error":"verify your email address before linking a sign-in method","code":"email_unverified"}`},
		{name: "a session the service no longer accepts", err: identity.ErrSignInRequired, status: http.StatusUnauthorized, want: `{"error":"linking a sign-in method needs a signed-in account"}`},
		{name: "a locked account", err: identity.ErrAccountLocked, status: http.StatusForbidden, want: `{"error":"your account is locked"}`},
		{name: "a reauthentication provider the account cannot use", err: fmt.Errorf("%w: the account holds no example identity", identity.ErrReauthenticationRequired), status: http.StatusBadRequest, want: `{"error":"confirm the link by signing in with a sign-in method your account already has","code":"reauthentication_required"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newIdentityHarness(t, testutil.TestBaseURL)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
			h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
			h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: "wrong"}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestIdentityController_LinkStartReportsAnAccountThatCannotBeRead(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(nil, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: authPassword}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to start the link"}`, rec.Body.String())
}

func TestIdentityController_AWrongLinkPasswordChargesTheAccountsCredentialBucket(t *testing.T) {
	// given one signed-in account guessing its password from many addresses
	h := newIdentityHarness(t, testutil.TestBaseURL)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
	h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(nil, user.ErrIncorrectPassword).Times(middleware.CredentialAttemptsPerMinute)

	for i := range middleware.CredentialAttemptsPerMinute {
		rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: "wrong"}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).WithRemoteAddr(fmt.Sprintf("198.51.100.%d:4444", i+1)).Record()
		require.Equal(t, http.StatusForbidden, rec.Code, "attempt %d", i+1)
		require.JSONEq(t, `{"error":"the current password is incorrect","code":"incorrect_password"}`, rec.Body.String())
	}

	// when
	linkRec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "link", CurrentPassword: "wrong"}).WithHeader(clientHeaderName, clientHeaderValue).WithCookie(cookie).WithRemoteAddr("203.0.113.9:4444").Record()
	loginRec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: "ALICE@example.com", Password: "wrong"}).WithHeader(clientHeaderName, clientHeaderValue).WithRemoteAddr("203.0.113.10:4444").Record()

	// then the password is never compared again, and password login for that address draws on the same bucket
	assert.Equal(t, http.StatusTooManyRequests, linkRec.Code)
	assert.NotEmpty(t, linkRec.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusTooManyRequests, loginRec.Code)
}

func TestIdentityController_StartDrawsOnTheSameCredentialBucketAsLogin(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Start(mock.Anything, mock.Anything).Return(&identity.Started{AuthURL: googleAuthURL, FlowToken: flowToken}, nil)
	h.auth.EXPECT().Login(mock.Anything, mock.Anything).Return(nil, user.ErrInvalidCredentials)

	for i := range middleware.CredentialAttemptsPerMinute {
		request := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"})
		if i%2 == 1 {
			request = h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: fmt.Sprintf("user%d@example.com", i), Password: "x"})
		}

		rec := request.WithHeader(clientHeaderName, clientHeaderValue).WithRemoteAddr(publicPeer).Record()
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "attempt %d", i+1)
	}

	// when
	rec := h.NewRequest(http.MethodPost, googleStartPath).WithJSONBody(dto.IdentityStartRequest{Purpose: "sign_in"}).WithHeader(clientHeaderName, clientHeaderValue).WithRemoteAddr(publicPeer).Record()

	// then each start spends one token, because each writes a flow row
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestIdentityController_ACallbackThatSignsInSetsTheSessionCookie(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		flowCookie    string
		sessionCookie string
		secure        bool
	}{
		{name: "over https", baseURL: testutil.TestBaseURL, flowCookie: flowCookieName, sessionCookie: testutil.SessionCookieName, secure: true},
		{name: "over http", baseURL: testutil.TestInsecureBaseURL, flowCookie: "rt_oidc", sessionCookie: "rt_session", secure: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a browser that still presents another session
			h := newIdentityHarness(t, tc.baseURL)
			h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
			presented := signInCallback()
			presented.SessionToken = testutil.TestSessionToken
			h.identity.EXPECT().Finish(mock.Anything, presented).Return(&identity.Outcome{Location: tc.baseURL + "/", SessionToken: issuedToken}, nil)

			// when
			rec := h.NewRequest(http.MethodGet, callbackURL).
				WithCookie(&http.Cookie{Name: tc.flowCookie, Value: flowToken}).
				WithCookie(&http.Cookie{Name: tc.sessionCookie, Value: testutil.TestSessionToken}).
				WithRemoteAddr(publicPeer).
				Record()

			// then the service replaced the presented session, and the browser gets the new one
			assertRedirect(t, rec, tc.baseURL+"/")
			assertSessionCookie(t, rec, tc.sessionCookie, tc.secure)
			assertFlowCookieCleared(t, rec, tc.flowCookie, tc.secure)
		})
	}
}

func TestIdentityController_ALinkCallbackKeepsItsSession(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	presented := signInCallback()
	presented.SessionToken = testutil.TestSessionToken
	h.identity.EXPECT().Finish(mock.Anything, presented).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/account"}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithCookie(&http.Cookie{Name: testutil.SessionCookieName, Value: testutil.TestSessionToken}).WithRemoteAddr(publicPeer).Record()

	// then the session that started the link, which the service checked, is left as it is
	assertRedirect(t, rec, testutil.TestBaseURL+"/account")
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
	assertFlowCookieCleared(t, rec, flowCookieName, true)
}

func TestIdentityController_AReauthenticationCallbackHandsOnWithANewFlowCookie(t *testing.T) {
	// given the service confirmed the account and started the link at the other provider
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	presented := signInCallback()
	presented.SessionToken = testutil.TestSessionToken
	h.identity.EXPECT().Finish(mock.Anything, presented).Return(&identity.Outcome{Location: exampleAuthURL, FlowToken: nextFlowToken}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithCookie(&http.Cookie{Name: testutil.SessionCookieName, Value: testutil.TestSessionToken}).WithRemoteAddr(publicPeer).Record()

	// then the browser goes straight on to the other provider, carrying only the new flow
	assertRedirect(t, rec, exampleAuthURL)
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
	assert.False(t, sessionCookieSet(rec, loginChallengeCookie))

	next := testutil.ResponseCookie(t, rec, flowCookieName)
	assert.Equal(t, nextFlowToken, next.Value)
	assert.Equal(t, flowTTLSeconds, next.MaxAge)
	assert.True(t, next.HttpOnly)
	assert.True(t, next.Secure)
	assert.Equal(t, http.SameSiteLaxMode, next.SameSite)
}

func TestIdentityController_ACallbackForAnAccountWithTwoFactorSetsOnlyTheChallengeCookie(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Finish(mock.Anything, signInCallback()).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/two-factor", ChallengeToken: loginChallengeToken}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()

	// then no session exists until the code is checked
	assertRedirect(t, rec, testutil.TestBaseURL+"/two-factor")
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
	assertFlowCookieCleared(t, rec, flowCookieName, true)

	challenge := testutil.ResponseCookie(t, rec, loginChallengeCookie)
	assert.Equal(t, loginChallengeToken, challenge.Value)
	assert.Equal(t, loginChallengeTTL, challenge.MaxAge)
	assert.True(t, challenge.HttpOnly)
	assert.True(t, challenge.Secure)
	assert.Equal(t, http.SameSiteLaxMode, challenge.SameSite)
}

func TestIdentityController_ACallbackHandsOnTheTrustedDevice(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	presented := signInCallback()
	presented.TrustedDeviceToken = trustedDeviceToken
	h.identity.EXPECT().Finish(mock.Anything, presented).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/", SessionToken: issuedToken}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).
		WithCookie(flowCookie()).
		WithCookie(&http.Cookie{Name: trustedDeviceCookie, Value: trustedDeviceToken}).
		WithRemoteAddr(publicPeer).
		Record()

	// then
	assertRedirect(t, rec, testutil.TestBaseURL+"/")
	assertSessionCookie(t, rec, testutil.SessionCookieName, true)
}

func TestIdentityController_ACommittedCallbackWhoseAftermathFailedStillLands(t *testing.T) {
	// given an automatic link whose notice could not be sent
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Finish(mock.Anything, signInCallback()).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/", SessionToken: issuedToken}, fmt.Errorf("%w: %w", identity.ErrNoticeNotSent, errServiceDown))

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()

	// then the cause is recorded, and the user is signed in all the same
	assertRedirect(t, rec, testutil.TestBaseURL+"/")
	assertSessionCookie(t, rec, testutil.SessionCookieName, true)
	assertFlowCookieCleared(t, rec, flowCookieName, true)
}

func TestIdentityController_EveryCallbackRefusalIsARedirect(t *testing.T) {
	cancelled := spec.IdentityCallback{Provider: model.ProviderGoogle, FlowToken: flowToken, State: callbackState, ProviderError: "access_denied", IP: callbackIP}

	tests := []struct {
		name     string
		url      string
		callback spec.IdentityCallback
		outcome  identity.Outcome
		err      error
	}{
		{name: "a cancelled sign-in", url: googleCallbackPath + "?error=access_denied&state=" + callbackState, callback: cancelled, outcome: identity.Outcome{Location: testutil.TestBaseURL + "/login?error=cancelled&provider=google"}},
		{name: "a used or expired flow", url: callbackURL, callback: signInCallback(), outcome: identity.Outcome{Location: testutil.TestBaseURL + "/login?error=expired&provider=google"}},
		{name: "a link whose session changed", url: callbackURL, callback: signInCallback(), outcome: identity.Outcome{Location: testutil.TestBaseURL + "/account?error=expired&provider=google"}},
		{name: "an address that needs linking from account settings", url: callbackURL, callback: signInCallback(), outcome: identity.Outcome{Location: testutil.TestBaseURL + "/login?error=link_required&provider=google"}},
		{name: "a failed discovery", url: callbackURL, callback: signInCallback(), outcome: identity.Outcome{Location: testutil.TestBaseURL + "/login?error=failed&provider=google"}, err: fmt.Errorf("complete the google sign-in: %w", errDiscoveryDown)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newIdentityHarness(t, testutil.TestBaseURL)
			h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
			h.identity.EXPECT().Finish(mock.Anything, tc.callback).Return(&tc.outcome, tc.err)

			// when
			rec := h.NewRequest(http.MethodGet, tc.url).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()

			// then a page navigation never lands on raw JSON
			assertRedirect(t, rec, tc.outcome.Location)
			assertFlowCookieCleared(t, rec, flowCookieName, true)
			assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
			assert.NotContains(t, rec.Body.String(), "database down")
		})
	}
}

func TestIdentityController_ACallbackWithoutAFlowCookieIsLeftToTheService(t *testing.T) {
	// given a browser that never started a flow, the login-CSRF case
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	arrived := signInCallback()
	arrived.FlowToken = ""
	h.identity.EXPECT().Finish(mock.Anything, arrived).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/login?error=expired&provider=google"}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithRemoteAddr(publicPeer).Record()

	// then
	assertRedirect(t, rec, testutil.TestBaseURL+"/login?error=expired&provider=google")
	assertFlowCookieCleared(t, rec, flowCookieName, true)
}

func TestIdentityController_ACallbackWhoseSwitchCannotBeReadRedirectsWithAFailure(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(false, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()

	// then the cause is recorded, the flow is left alone, and the browser still lands on a page that can name the provider
	assertRedirect(t, rec, failedLocation)
	assert.NotContains(t, rec.Body.String(), "database down")
}

func TestIdentityController_ACallbackWhoseSessionCookieCannotBeBuiltRedirectsWithAFailure(t *testing.T) {
	// given a session duration that cannot be read after the sign-in succeeded
	base := testutil.NewHarness(t)
	base.SettingsService.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testutil.TestBaseURL, nil).Maybe()
	base.SettingsService.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(0, errServiceDown)
	h := mountIdentityRoutes(t, base)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Finish(mock.Anything, signInCallback()).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/", SessionToken: issuedToken}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()

	// then the user is not sent on as though signed in
	assertRedirect(t, rec, failedLocation)
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
	assertFlowCookieCleared(t, rec, flowCookieName, true)
}

func TestIdentityController_TheCallbackDrawsOnNoLimiter(t *testing.T) {
	// given
	h := newIdentityHarness(t, testutil.TestBaseURL)
	h.identity.EXPECT().Enabled(mock.Anything, model.ProviderGoogle).Return(true, nil)
	h.identity.EXPECT().Finish(mock.Anything, signInCallback()).Return(&identity.Outcome{Location: testutil.TestBaseURL + "/login?error=expired&provider=google"}, nil)

	for range 2 * middleware.CredentialAttemptsPerMinute {
		rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()
		require.Equal(t, http.StatusSeeOther, rec.Code)
	}

	// when
	rec := h.NewRequest(http.MethodGet, callbackURL).WithCookie(flowCookie()).WithRemoteAddr(publicPeer).Record()

	// then a limiter's JSON 429 never strands the user on a callback
	assertRedirect(t, rec, testutil.TestBaseURL+"/login?error=expired&provider=google")
	assert.Empty(t, rec.Header().Get("X-RateLimit-Limit"))
}
