package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/auth"
	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/cookie"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/identity"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org"
	"retrotide/internal/role"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	authHarness struct {
		*testutil.Harness
		auth     *auth.MockService
		users    *user.MockService
		identity *identity.MockService
	}
)

const (
	loginChallengeCookie = "__Host-rt_2fa"
	loginChallengeToken  = "6b8d0f2a4c6e8b0d2f4a6c8e0b2d4f6a8c0e2b4d6f8a0c2e4b6d8f0a2c4e6b8d"
	loginChallengeTTL    = 300

	trustedDeviceCookie = "__Host-rt_trusted_device"
	trustedDeviceToken  = "2e4a6c8e0b2d4f6a8c0e2b4d6f8a0c2e4b6d8f0a2c4e6b8d6b8d0f2a4c6e8b0d"
	trustedDeviceTTL    = 30 * 24 * 60 * 60

	issuedToken     = "8c6e4a2f0d9b7c5e3a1f8d6b4c2e0a9f7d5b3c1e8a6f4d2b0c9e7a5f3d1b8c6e"
	authEmail       = "alice@example.com"
	authPassword    = "correct horse battery"
	authNewPassword = "a brand new passphrase"
	publicPeer      = "198.51.100.7:4444"
)

func newAuthHarness(t *testing.T, baseURL string, turnstile bool) *authHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(baseURL)

	return mountAuthRoutes(t, h, turnstile)
}

func mountAuthRoutes(t *testing.T, h *testutil.Harness, turnstile bool) *authHarness {
	t.Helper()

	h.SettingsService.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(turnstile, nil).Maybe()

	a := &authHarness{Harness: h, auth: auth.NewMockService(t), users: user.NewMockService(t), identity: identity.NewMockService(t)}

	s := &Service{
		AuthService:      a.auth,
		UserService:      a.users,
		IdentityService:  a.identity,
		AuthzService:     h.Authz,
		Sessions:         h.Sessions,
		TwoFactorCookies: cookie.NewBuilder(h.SettingsService, twofactor.ChallengeCookieName, cookie.Fixed(twofactor.ChallengeTTL)),
		TrustedDevices:   cookie.NewBuilder(h.SettingsService, twofactor.TrustedDeviceCookieName, cookie.Fixed(twofactor.TrustedDeviceTTL)),
		Guards:           h.Guards,
	}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllAuthRoutes())

	return a
}

func signedInAccount(userID uuid.UUID) *dto.UserResponse {
	return &dto.UserResponse{ID: userID, Email: authEmail, DisplayName: "Alice", HasPassword: true, CreatedAt: "2026-09-25T12:00:00Z"}
}

func assertSessionCookie(t *testing.T, rec *httptest.ResponseRecorder, name string, secure bool) {
	t.Helper()

	cookie := testutil.ResponseCookie(t, rec, name)
	assert.Equal(t, issuedToken, cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.Empty(t, cookie.Domain)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, secure, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, testutil.TestSessionDays*24*60*60, cookie.MaxAge)
}

func assertClearedCookie(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	cookie := testutil.ResponseCookie(t, rec, testutil.SessionCookieName)
	assert.Empty(t, cookie.Value)
	assert.Equal(t, -1, cookie.MaxAge)
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

func registerBody() dto.RegisterRequest {
	return dto.RegisterRequest{Email: authEmail, Password: authPassword, DisplayName: "Alice"}
}

func TestAuthController_RegisterAnswersWithoutASession(t *testing.T) {
	tests := []struct {
		name       string
		registered *dto.RegistrationResponse
		want       string
	}{
		{
			name:       "an account that must verify its address first",
			registered: &dto.RegistrationResponse{Email: authEmail, VerificationRequired: true},
			want:       `{"email":"alice@example.com","verification_required":true}`,
		},
		{
			name:       "the first account, which starts verified",
			registered: &dto.RegistrationResponse{Email: authEmail},
			want:       `{"email":"alice@example.com","verification_required":false}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().Register(mock.Anything, spec.AccountRegistration{Email: authEmail, Password: authPassword, DisplayName: "Alice"}).Return(tc.registered, nil)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/register").WithJSONBody(registerBody()).Record()

			// then the new account signs in on its own once it may
			require.Equal(t, http.StatusCreated, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestAuthController_RegisterStillAnswersCreatedWhenTheVerificationEmailFails(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().Register(mock.Anything, mock.Anything).Return(&dto.RegistrationResponse{Email: authEmail, VerificationRequired: true}, fmt.Errorf("%w: %w", auth.ErrEmailNotSent, errServiceDown))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/register").WithJSONBody(registerBody()).Record()

	// then the account exists, so the user can ask for the link again from the login page
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"email":"alice@example.com","verification_required":true}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_RegisterRefusesAMissingEmailOrPassword(t *testing.T) {
	tests := []struct {
		name string
		body dto.RegisterRequest
	}{
		{name: "no email", body: dto.RegisterRequest{Password: authPassword}},
		{name: "no password", body: dto.RegisterRequest{Email: authEmail}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/register").WithJSONBody(tc.body).Record()

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"email and password are required"}`, rec.Body.String())
		})
	}
}

func TestAuthController_RegisterRefusesABodyThatIsNotJSON(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/register").WithRawBody("{", "application/json").Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid request body"}`, rec.Body.String())
}

func TestAuthController_RegisterMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "a private site that already has an account", err: auth.ErrRegistrationPrivate, status: http.StatusForbidden, body: `{"error":"this site is private, ask an admin to invite you","code":"site_private"}`},
		{name: "a site that cannot send email and already has an account", err: auth.ErrRegistrationClosed, status: http.StatusForbidden, body: `{"error":"sign-up is closed until this site can send email","code":"registration_closed"}`},
		{name: "invalid email", err: fmt.Errorf("%w: mail: missing @", auth.ErrInvalidEmail), status: http.StatusBadRequest, body: `{"error":"a valid email address is required"}`},
		{name: "email taken", err: auth.ErrEmailTaken, status: http.StatusConflict, body: `{"error":"that email address is already in use"}`},
		{name: "password too short", err: user.ErrPasswordTooShort, status: http.StatusBadRequest, body: `{"error":"the password is too short","code":"password_too_short"}`},
		{name: "password too long", err: user.ErrPasswordTooLong, status: http.StatusBadRequest, body: `{"error":"the password is longer than 72 bytes"}`},
		{name: "an unexpected failure", err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to register"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/register").WithJSONBody(registerBody()).Record()

			// then the cause stays in the log and no session is handed out
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
			assert.NotContains(t, rec.Body.String(), "database down")
		})
	}
}

func TestAuthController_GetInvitationShowsTheInvitedAddress(t *testing.T) {
	// given no session, because the invitee has no account yet
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().GetInvitation(mock.Anything, issuedToken).Return(&dto.InvitationResponse{Email: "carol@example.com", ExpiresAt: "2026-10-03T12:00:00Z"}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/invitation").WithJSONBody(dto.InvitationTokenRequest{Token: issuedToken}).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"email":"carol@example.com","expires_at":"2026-10-03T12:00:00Z"}`, rec.Body.String())
}

func TestAuthController_GetInvitationMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "an invalid, used or expired link", err: auth.ErrInvalidInvitation, status: http.StatusBadRequest, body: `{"error":"the invitation link is invalid, used or has expired","code":"invitation_invalid"}`},
		{name: "an unexpected failure", err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to read the invitation"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().GetInvitation(mock.Anything, issuedToken).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/invitation").WithJSONBody(dto.InvitationTokenRequest{Token: issuedToken}).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAuthController_GetInvitationNeedsTheToken(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/invitation").WithJSONBody(dto.InvitationTokenRequest{}).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"token is required"}`, rec.Body.String())
}

func TestAuthController_AcceptInvitationCreatesTheAccountWithoutASession(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().AcceptInvitation(mock.Anything, spec.InvitationAcceptance{Token: issuedToken, DisplayName: "Carol", Password: authPassword, IP: "198.51.100.7"}).Return(&dto.InvitationAcceptedResponse{Email: "carol@example.com"}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/invitation/accept").
		WithJSONBody(dto.AcceptInvitationRequest{Token: issuedToken, DisplayName: "Carol", Password: authPassword}).
		WithRemoteAddr(publicPeer).
		Record()

	// then the page offers Sign in, as after verifying an address
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"email":"carol@example.com"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_AcceptInvitationStillAnswersCreatedWhenTheAuditEntryFails(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().AcceptInvitation(mock.Anything, mock.Anything).Return(&dto.InvitationAcceptedResponse{Email: "carol@example.com"}, fmt.Errorf("%w: %w", auth.ErrNotAudited, errServiceDown))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/invitation/accept").WithJSONBody(dto.AcceptInvitationRequest{Token: issuedToken, DisplayName: "Carol", Password: authPassword}).Record()

	// then the account exists, so the invitee can sign in
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"email":"carol@example.com"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_AcceptInvitationNeedsTheTokenAndAPassword(t *testing.T) {
	tests := []struct {
		name string
		body dto.AcceptInvitationRequest
	}{
		{name: "no token", body: dto.AcceptInvitationRequest{DisplayName: "Carol", Password: authPassword}},
		{name: "no password", body: dto.AcceptInvitationRequest{Token: issuedToken, DisplayName: "Carol"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/invitation/accept").WithJSONBody(tc.body).Record()

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"token and password are required"}`, rec.Body.String())
		})
	}
}

func TestAuthController_AcceptInvitationMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "an invalid, used or expired link", err: auth.ErrInvalidInvitation, status: http.StatusBadRequest, body: `{"error":"the invitation link is invalid, used or has expired","code":"invitation_invalid"}`},
		{name: "no display name", err: user.ErrDisplayNameRequired, status: http.StatusBadRequest, body: `{"error":"a display name is required"}`},
		{name: "password too short", err: user.ErrPasswordTooShort, status: http.StatusBadRequest, body: `{"error":"the password is too short","code":"password_too_short"}`},
		{name: "an account took the address meanwhile", err: auth.ErrEmailTaken, status: http.StatusConflict, body: `{"error":"that email address is already in use"}`},
		{name: "an unexpected failure", err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to accept the invitation"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().AcceptInvitation(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/invitation/accept").WithJSONBody(dto.AcceptInvitationRequest{Token: issuedToken, DisplayName: "Carol", Password: authPassword}).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestAuthController_LoginWithRememberMeSetsALastingSessionCookie(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	account := signedInAccount(uuid.New())
	signIn := spec.PasswordSignIn{UserCredentials: spec.UserCredentials{Email: authEmail, Password: authPassword}, Remember: true}
	h.auth.EXPECT().Login(mock.Anything, signIn).Return(&auth.LoginResult{User: account, SessionToken: issuedToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: authEmail, Password: authPassword, Remember: true}).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, account.ID, testutil.UnmarshalJSON[dto.UserResponse](t, rec.Body.Bytes()).ID)
	assertSessionCookie(t, rec, testutil.SessionCookieName, true)
	assert.False(t, sessionCookieSet(rec, loginChallengeCookie))
}

func TestAuthController_LoginWithoutRememberMeSetsASessionCookieThatEndsWithTheBrowser(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	signIn := spec.PasswordSignIn{UserCredentials: spec.UserCredentials{Email: authEmail, Password: authPassword}}
	h.auth.EXPECT().Login(mock.Anything, signIn).Return(&auth.LoginResult{User: signedInAccount(uuid.New()), SessionToken: issuedToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: authEmail, Password: authPassword}).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	cookie := testutil.ResponseCookie(t, rec, testutil.SessionCookieName)
	assert.Equal(t, issuedToken, cookie.Value)
	assert.Zero(t, cookie.MaxAge)
	assert.True(t, cookie.Expires.IsZero())
	assert.NotContains(t, rec.Header().Get("Set-Cookie"), "Max-Age")
}

func TestAuthController_LoginHandsOnTheTrustedDevice(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	signIn := spec.PasswordSignIn{UserCredentials: spec.UserCredentials{Email: authEmail, Password: authPassword}, TrustedDeviceToken: trustedDeviceToken}
	h.auth.EXPECT().Login(mock.Anything, signIn).Return(&auth.LoginResult{User: signedInAccount(uuid.New()), SessionToken: issuedToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").
		WithJSONBody(dto.LoginRequest{Email: authEmail, Password: authPassword}).
		WithCookie(&http.Cookie{Name: trustedDeviceCookie, Value: trustedDeviceToken}).
		Record()

	// then the device stays trusted, so the cookie is not written again
	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, sessionCookieSet(rec, trustedDeviceCookie))
}

func TestAuthController_LoginWithTwoFactorSetsOnlyTheChallengeCookie(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	account := signedInAccount(uuid.New())
	h.auth.EXPECT().Login(mock.Anything, spec.PasswordSignIn{UserCredentials: spec.UserCredentials{Email: authEmail, Password: authPassword}}).Return(&auth.LoginResult{User: account, ChallengeToken: loginChallengeToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: authEmail, Password: authPassword}).Record()

	// then the account is not described and no session exists until the code is checked
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"two_factor_required":true}`, rec.Body.String())
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))

	challenge := testutil.ResponseCookie(t, rec, loginChallengeCookie)
	assert.Equal(t, loginChallengeToken, challenge.Value)
	assert.Equal(t, "/", challenge.Path)
	assert.True(t, challenge.HttpOnly)
	assert.True(t, challenge.Secure)
	assert.Equal(t, http.SameSiteLaxMode, challenge.SameSite)
	assert.Equal(t, loginChallengeTTL, challenge.MaxAge)
}

func TestAuthController_PasswordRoutesAnswerForbiddenWhilePasswordSignInIsOff(t *testing.T) {
	tests := []struct {
		path   string
		body   any
		expect func(h *authHarness)
	}{
		{path: "/api/v1/auth/login", body: dto.LoginRequest{Email: authEmail, Password: authPassword}, expect: func(h *authHarness) {
			h.auth.EXPECT().Login(mock.Anything, mock.Anything).Return(nil, auth.ErrPasswordSignInDisabled)
		}},
		{path: "/api/v1/auth/register", body: registerBody(), expect: func(h *authHarness) {
			h.auth.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, auth.ErrPasswordSignInDisabled)
		}},
		{path: "/api/v1/auth/forgot-password", body: dto.ForgotPasswordRequest{Email: authEmail}, expect: func(h *authHarness) {
			h.auth.EXPECT().ForgotPassword(mock.Anything, authEmail).Return(auth.ErrPasswordSignInDisabled)
		}},
		{path: "/api/v1/auth/reset-password", body: dto.ResetPasswordRequest{Token: issuedToken, NewPassword: authNewPassword}, expect: func(h *authHarness) {
			h.auth.EXPECT().ResetPassword(mock.Anything, mock.Anything).Return(auth.ErrPasswordSignInDisabled)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			tc.expect(h)

			// when
			rec := h.NewRequest(http.MethodPost, tc.path).WithJSONBody(tc.body).Record()

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.JSONEq(t, `{"error":"password sign-in is turned off on this site","code":"password_sign_in_disabled"}`, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestAuthController_LoginMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "an unknown email or a wrong password", err: user.ErrInvalidCredentials, status: http.StatusUnauthorized, body: `{"error":"invalid email or password"}`},
		{name: "a banned user", err: auth.ErrUserBanned, status: http.StatusForbidden, body: `{"error":"your account has been banned"}`},
		{name: "the right password on an unverified account", err: auth.ErrEmailUnverified, status: http.StatusForbidden, body: `{"error":"verify your email address before signing in","code":"email_unverified"}`},
		{name: "password sign-in switched off", err: auth.ErrPasswordSignInDisabled, status: http.StatusForbidden, body: `{"error":"password sign-in is turned off on this site","code":"password_sign_in_disabled"}`},
		{name: "an unexpected failure", err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to sign in"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().Login(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: authEmail, Password: "wrong"}).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestAuthController_LoginRefusesAMissingEmailOrPassword(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: authEmail}).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"email and password are required"}`, rec.Body.String())
}

func TestAuthController_TurnstileGuardsEveryPublicCredentialAndMailRoute(t *testing.T) {
	tests := []struct {
		path string
		body any
	}{
		{path: "/api/v1/auth/register", body: registerBody()},
		{path: "/api/v1/auth/login", body: dto.LoginRequest{Email: authEmail, Password: authPassword}},
		{path: "/api/v1/auth/forgot-password", body: dto.ForgotPasswordRequest{Email: authEmail}},
		{path: "/api/v1/auth/resend-verification", body: dto.ResendVerificationRequest{Email: authEmail}},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given Turnstile is switched on and the body carries no token
			h := newAuthHarness(t, testutil.TestBaseURL, true)
			h.SettingsService.EXPECT().Get(mock.Anything, config.SettingTurnstileSecretKey).Return("secret", nil)

			// when
			rec := h.NewRequest(http.MethodPost, tc.path).WithJSONBody(tc.body).Record()

			// then the handler never runs, so the strict service mock sees no call
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"verification required"}`, rec.Body.String())
		})
	}
}

func TestAuthController_RegisterLoginResetAndInvitationAcceptanceShareOneCredentialBucketPerIP(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, auth.ErrEmailTaken)
	h.auth.EXPECT().Login(mock.Anything, mock.Anything).Return(nil, user.ErrInvalidCredentials)
	h.auth.EXPECT().ResetPassword(mock.Anything, mock.Anything).Return(auth.ErrInvalidResetToken)
	h.auth.EXPECT().AcceptInvitation(mock.Anything, mock.Anything).Return(nil, auth.ErrInvalidInvitation)

	attempts := []struct {
		path string
		body any
	}{
		{path: "/api/v1/auth/login", body: dto.LoginRequest{Email: "one@example.com", Password: "x"}},
		{path: "/api/v1/auth/register", body: dto.RegisterRequest{Email: "two@example.com", Password: "x"}},
		{path: "/api/v1/auth/reset-password", body: dto.ResetPasswordRequest{Token: "t", NewPassword: "x"}},
		{path: "/api/v1/auth/invitation/accept", body: dto.AcceptInvitationRequest{Token: "t", DisplayName: "Carol", Password: "x"}},
	}

	for i := range middleware.CredentialAttemptsPerMinute {
		attempt := attempts[i%len(attempts)]
		rec := h.NewRequest(http.MethodPost, attempt.path).WithJSONBody(attempt.body).WithRemoteAddr(publicPeer).Record()
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "attempt %d on %s", i+1, attempt.path)
	}

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: "three@example.com", Password: "x"}).WithRemoteAddr(publicPeer).Record()

	// then
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestAuthController_LoginIsAlsoLimitedPerAccount(t *testing.T) {
	// given one account attacked from many addresses
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().Login(mock.Anything, mock.Anything).Return(nil, user.ErrInvalidCredentials)

	for i := range middleware.CredentialAttemptsPerMinute {
		rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: authEmail, Password: "x"}).WithRemoteAddr(fmt.Sprintf("198.51.100.%d:4444", i+1)).Record()
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: "ALICE@example.com", Password: "x"}).WithRemoteAddr("203.0.113.9:4444").Record()

	// then
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestAuthController_ForgotPasswordIsLimitedPerIPAndPerAccount(t *testing.T) {
	tests := []struct {
		name    string
		address func(i int) string
		peer    func(i int) string
	}{
		{name: "per ip", address: func(i int) string { return fmt.Sprintf("user%d@example.com", i) }, peer: func(int) string { return publicPeer }},
		{name: "per account", address: func(int) string { return authEmail }, peer: func(i int) string { return fmt.Sprintf("198.51.100.%d:4444", i+1) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().ForgotPassword(mock.Anything, mock.Anything).Return(nil)

			for i := range middleware.MailAttemptsPerHour {
				rec := h.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password").WithJSONBody(dto.ForgotPasswordRequest{Email: tc.address(i)}).WithRemoteAddr(tc.peer(i)).Record()
				require.Equal(t, http.StatusNoContent, rec.Code)
			}

			// when
			last := middleware.MailAttemptsPerHour
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password").WithJSONBody(dto.ForgotPasswordRequest{Email: tc.address(last)}).WithRemoteAddr(tc.peer(last)).Record()

			// then
			assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		})
	}
}

func TestAuthController_ForgotPasswordAnswersNoContent(t *testing.T) {
	// given the service decides the answer is the same for an unknown address
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().ForgotPassword(mock.Anything, "nobody@example.com").Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password").WithJSONBody(dto.ForgotPasswordRequest{Email: "nobody@example.com"}).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestAuthController_ForgotPasswordMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		body   dto.ForgotPasswordRequest
		err    error
		status int
		want   string
	}{
		{name: "no email", body: dto.ForgotPasswordRequest{}, status: http.StatusBadRequest, want: `{"error":"email is required"}`},
		{name: "no provider", body: dto.ForgotPasswordRequest{Email: authEmail}, err: auth.ErrEmailUnavailable, status: http.StatusServiceUnavailable, want: `{"error":"email is not available on this site","code":"email_unavailable"}`},
		{name: "invalid email", body: dto.ForgotPasswordRequest{Email: "nope"}, err: auth.ErrInvalidEmail, status: http.StatusBadRequest, want: `{"error":"a valid email address is required"}`},
		{name: "a failed send", body: dto.ForgotPasswordRequest{Email: authEmail}, err: errServiceDown, status: http.StatusInternalServerError, want: `{"error":"failed to send the password reset email"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			if tc.err != nil {
				h.auth.EXPECT().ForgotPassword(mock.Anything, tc.body.Email).Return(tc.err)
			}

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password").WithJSONBody(tc.body).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestAuthController_ResendVerificationNeedsNoSession(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{name: "an unverified account", address: authEmail},
		{name: "an unknown address, which the service answers the same way", address: "nobody@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given no session cookie, because an unverified account can never sign in
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().ResendVerification(mock.Anything, tc.address).Return(nil)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification").WithJSONBody(dto.ResendVerificationRequest{Email: tc.address}).Record()

			// then
			assert.Equal(t, http.StatusNoContent, rec.Code)
			assert.Empty(t, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestAuthController_ResendVerificationAnswersNoContentWhenTheEmailFails(t *testing.T) {
	// given the service found an unverified account but could not send its link
	var recorded []error
	base := testutil.NewHarness(t)
	base.UseBaseURL(testutil.TestBaseURL)
	base.Engine.Use(func(c *gin.Context) {
		c.Next()

		for _, entry := range c.Errors {
			recorded = append(recorded, entry.Err)
		}
	})
	h := mountAuthRoutes(t, base, false)
	h.auth.EXPECT().ResendVerification(mock.Anything, authEmail).Return(fmt.Errorf("%w to user %s: %w", auth.ErrVerificationNotSent, uuid.New(), errServiceDown))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification").WithJSONBody(dto.ResendVerificationRequest{Email: authEmail}).Record()

	// then the answer matches an unknown address, and the cause is still recorded
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
	require.Len(t, recorded, 1)
	assert.ErrorIs(t, recorded[0], errServiceDown)
}

func TestAuthController_ResendVerificationMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		body   dto.ResendVerificationRequest
		err    error
		status int
		want   string
	}{
		{name: "no email", body: dto.ResendVerificationRequest{}, status: http.StatusBadRequest, want: `{"error":"email is required"}`},
		{name: "no provider", body: dto.ResendVerificationRequest{Email: authEmail}, err: auth.ErrEmailUnavailable, status: http.StatusServiceUnavailable, want: `{"error":"email is not available on this site","code":"email_unavailable"}`},
		{name: "invalid email", body: dto.ResendVerificationRequest{Email: "nope"}, err: auth.ErrInvalidEmail, status: http.StatusBadRequest, want: `{"error":"a valid email address is required"}`},
		{name: "an unexpected failure", body: dto.ResendVerificationRequest{Email: authEmail}, err: errServiceDown, status: http.StatusInternalServerError, want: `{"error":"failed to resend the verification email"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			if tc.err != nil {
				h.auth.EXPECT().ResendVerification(mock.Anything, tc.body.Email).Return(tc.err)
			}

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification").WithJSONBody(tc.body).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestAuthController_ResendVerificationIsLimitedPerIPAndPerAccount(t *testing.T) {
	tests := []struct {
		name    string
		address func(i int) string
		peer    func(i int) string
	}{
		{name: "per ip", address: func(i int) string { return fmt.Sprintf("user%d@example.com", i) }, peer: func(int) string { return publicPeer }},
		{name: "per account", address: func(int) string { return authEmail }, peer: func(i int) string { return fmt.Sprintf("198.51.100.%d:4444", i+1) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			h.auth.EXPECT().ResendVerification(mock.Anything, mock.Anything).Return(nil)

			for i := range middleware.MailAttemptsPerHour {
				rec := h.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification").WithJSONBody(dto.ResendVerificationRequest{Email: tc.address(i)}).WithRemoteAddr(tc.peer(i)).Record()
				require.Equal(t, http.StatusNoContent, rec.Code)
			}

			// when
			last := middleware.MailAttemptsPerHour
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification").WithJSONBody(dto.ResendVerificationRequest{Email: tc.address(last)}).WithRemoteAddr(tc.peer(last)).Record()

			// then
			assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		})
	}
}

func TestAuthController_ResendVerificationSharesTheMailBucketWithForgotPassword(t *testing.T) {
	// given one account whose mail budget forgotten password has spent
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().ForgotPassword(mock.Anything, authEmail).Return(nil)

	for i := range middleware.MailAttemptsPerHour {
		rec := h.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password").WithJSONBody(dto.ForgotPasswordRequest{Email: authEmail}).WithRemoteAddr(fmt.Sprintf("198.51.100.%d:4444", i+1)).Record()
		require.Equal(t, http.StatusNoContent, rec.Code)
	}

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification").WithJSONBody(dto.ResendVerificationRequest{Email: authEmail}).WithRemoteAddr("203.0.113.9:4444").Record()

	// then the resend is refused before the service runs, which the strict mock would report
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestAuthController_LogoutDeletesTheSessionAndClearsTheCookie(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().Logout(mock.Anything, testutil.TestSessionToken).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/logout").WithCookie(&http.Cookie{Name: testutil.SessionCookieName, Value: testutil.TestSessionToken}).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assertClearedCookie(t, rec)
}

func TestAuthController_LogoutWithoutACookieStillClearsIt(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().Logout(mock.Anything, "").Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/logout").Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assertClearedCookie(t, rec)
}

func TestAuthController_LogoutKeepsTheCookieWhenTheSessionCannotBeDeleted(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().Logout(mock.Anything, testutil.TestSessionToken).Return(errServiceDown)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/logout").WithCookie(&http.Cookie{Name: testutil.SessionCookieName, Value: testutil.TestSessionToken}).Record()

	// then the browser is not told it signed out while the session still works
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to sign out"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_SessionReportsAnAnonymousVisitor(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"authenticated":false}`, rec.Body.String())
}

func TestAuthController_SessionReportsAnExpiredSessionAsAnonymous(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.SessionRepo.EXPECT().Get(mock.Anything, testutil.TestSessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(&http.Cookie{Name: testutil.SessionCookieName, Value: testutil.TestSessionToken}).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"authenticated":false}`, rec.Body.String())
}

func TestAuthController_SessionReportsAGuestByItsDeviceOnly(t *testing.T) {
	// given a browser that joined a board as a guest and holds no session
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	deviceID := uuid.MustParse("6e7f8091-a2b3-44c5-96d7-4c5d6e7f8091")
	cookie := h.SignInAsGuest(deviceID)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(cookie).Record()

	// then the device reaches only its own browser, and the visitor is not signed in
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"authenticated":false,"guest":{"id":"6e7f8091-a2b3-44c5-96d7-4c5d6e7f8091"}}`, rec.Body.String())
}

func TestAuthController_SessionPrefersTheUserOverTheGuest(t *testing.T) {
	// given a browser holding both cookies, as one does straight after a sign-in from a board
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
	h.Authz.EXPECT().EffectivePermissions(mock.Anything, userID).Return([]authz.Permission{}, nil)
	h.Authz.EXPECT().GetRole(mock.Anything, userID).Return("", nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").
		WithCookie(cookie).
		WithCookie(&http.Cookie{Name: testutil.GuestCookieName, Value: testutil.TestGuestToken}).
		Record()

	// then the guest device is never looked up, which the strict mock would report
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "guest")
}

func TestAuthController_SessionReturnsTheSignedInUser(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
	h.Authz.EXPECT().EffectivePermissions(mock.Anything, userID).Return([]authz.Permission{}, nil)
	h.Authz.EXPECT().GetRole(mock.Anything, userID).Return("", nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(cookie).Record()

	// then a user without a staff role carries no permissions and no site role
	require.Equal(t, http.StatusOK, rec.Code)
	session := testutil.UnmarshalJSON[dto.SessionResponse](t, rec.Body.Bytes())
	assert.True(t, session.Authenticated)
	require.NotNil(t, session.User)
	assert.Equal(t, userID, session.User.ID)
	assert.Empty(t, session.Permissions)
	assert.NotContains(t, rec.Body.String(), "site_role")
}

func TestAuthController_SessionCarriesTheStaffPermissions(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
	h.Authz.EXPECT().EffectivePermissions(mock.Anything, userID).Return([]authz.Permission{authz.PermViewAdminPanel, authz.PermViewUsers}, nil)
	h.Authz.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(cookie).Record()

	// then the admin panel can show only what the viewer may use, and member lists can place the viewer above every organisation role
	require.Equal(t, http.StatusOK, rec.Code)
	session := testutil.UnmarshalJSON[dto.SessionResponse](t, rec.Body.Bytes())
	assert.Equal(t, []string{"view_admin_panel", "view_users"}, session.Permissions)
	assert.Equal(t, role.Support, session.SiteRole)
}

func TestAuthController_SessionReportsAFailedRoleLookup(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
	h.Authz.EXPECT().EffectivePermissions(mock.Anything, userID).Return([]authz.Permission{}, nil)
	h.Authz.EXPECT().GetRole(mock.Anything, userID).Return("", errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(cookie).Record()

	// then a blip never reads as a session without a site role
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to load the session"}`, rec.Body.String())
}

func TestAuthController_SessionReportsAFailedPermissionLookup(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(signedInAccount(userID), nil)
	h.Authz.EXPECT().EffectivePermissions(mock.Anything, userID).Return(nil, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(cookie).Record()

	// then a blip never reads as a session without staff permissions
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to load the session"}`, rec.Body.String())
}

func TestAuthController_SessionOfADeletedUserIsAnonymous(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(nil, user.ErrUserNotFound)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"authenticated":false}`, rec.Body.String())
}

func TestAuthController_SessionReportsAFailedLookup(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(nil, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/session").WithCookie(cookie).Record()

	// then a blip is never reported as signed out
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to load the session"}`, rec.Body.String())
}

func TestAuthController_VerifyEmailNeedsOnlyTheToken(t *testing.T) {
	// given no session at all, because the link may open in another browser
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	h.auth.EXPECT().VerifyEmail(mock.Anything, issuedToken).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/verify-email").WithJSONBody(dto.VerifyEmailRequest{Token: issuedToken}).Record()

	// then the address is verified, and the user signs in afterwards on their own
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_VerifyEmailMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		err    error
		status int
		want   string
	}{
		{name: "no token", token: "", status: http.StatusBadRequest, want: `{"error":"token is required"}`},
		{name: "an expired or used token", token: issuedToken, err: auth.ErrInvalidVerificationToken, status: http.StatusBadRequest, want: `{"error":"the verification link is invalid or has expired"}`},
		{name: "a pending address taken meanwhile", token: issuedToken, err: auth.ErrEmailTaken, status: http.StatusConflict, want: `{"error":"that email address is already in use"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			if tc.err != nil {
				h.auth.EXPECT().VerifyEmail(mock.Anything, tc.token).Return(tc.err)
			}

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/verify-email").WithJSONBody(dto.VerifyEmailRequest{Token: tc.token}).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestAuthController_ResetPasswordMapsEveryOutcome(t *testing.T) {
	tests := []struct {
		name   string
		body   dto.ResetPasswordRequest
		err    error
		status int
		want   string
	}{
		{name: "a reset", body: dto.ResetPasswordRequest{Token: issuedToken, NewPassword: authNewPassword}, status: http.StatusNoContent},
		{name: "no token", body: dto.ResetPasswordRequest{NewPassword: authNewPassword}, status: http.StatusBadRequest, want: `{"error":"token and new password are required"}`},
		{name: "an expired or used token", body: dto.ResetPasswordRequest{Token: issuedToken, NewPassword: authNewPassword}, err: auth.ErrInvalidResetToken, status: http.StatusBadRequest, want: `{"error":"the reset link is invalid or has expired"}`},
		{name: "a short password", body: dto.ResetPasswordRequest{Token: issuedToken, NewPassword: "short"}, err: user.ErrPasswordTooShort, status: http.StatusBadRequest, want: `{"error":"the password is too short","code":"password_too_short"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			if tc.body.Token != "" {
				h.auth.EXPECT().ResetPassword(mock.Anything, spec.PasswordResetCompletion{Token: tc.body.Token, NewPassword: tc.body.NewPassword}).Return(tc.err)
			}

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/reset-password").WithJSONBody(tc.body).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			if tc.want != "" {
				assert.JSONEq(t, tc.want, rec.Body.String())
			}
		})
	}
}

func TestAuthController_AccountRoutesRunTheAuthFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: "/api/v1/auth/set-email", Body: dto.SetEmailRequest{Email: "alice@work.example", Password: authPassword}},
		{Method: http.MethodPut, Path: "/api/v1/auth/password", Body: dto.ChangePasswordRequest{CurrentPassword: authPassword, NewPassword: authNewPassword}},
		{Method: http.MethodPut, Path: "/api/v1/auth/profile", Body: dto.UpdateDisplayNameRequest{DisplayName: "Ally"}},
		{Method: http.MethodPost, Path: "/api/v1/auth/sign-out-other-devices"},
		{Method: http.MethodDelete, Path: "/api/v1/auth/account", Body: dto.DeleteAccountRequest{Password: authPassword}},
		{Method: http.MethodGet, Path: "/api/v1/auth/identities"},
		{Method: http.MethodDelete, Path: "/api/v1/auth/identities/google"},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newAuthHarness(t, testutil.TestBaseURL, false).Harness
			}, route)
		})
	}
}

func TestAuthController_ALockedUserCannotDeleteTheAccount(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/auth/account").WithJSONBody(dto.DeleteAccountRequest{Password: authPassword}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"your account is locked"}`, rec.Body.String())
}

func TestAuthController_ABannedSessionIsRevokedAtTheGuard(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	h.SessionRepo.EXPECT().Get(mock.Anything, testutil.TestSessionToken).Return(&model.Session{UserID: userID}, nil)
	h.Authz.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
	h.SessionRepo.EXPECT().Delete(mock.Anything, testutil.TestSessionToken).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/auth/password").WithJSONBody(dto.ChangePasswordRequest{CurrentPassword: authPassword, NewPassword: authNewPassword}).WithCookie(&http.Cookie{Name: testutil.SessionCookieName, Value: testutil.TestSessionToken}).Record()

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"your account has been banned"}`, rec.Body.String())
}

func TestAuthController_ChangePasswordKeepsThePresentedSession(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.auth.EXPECT().ChangePassword(mock.Anything, spec.PasswordUpdate{UserID: userID, CurrentPassword: authPassword, NewPassword: authNewPassword, KeepToken: testutil.TestSessionToken}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/auth/password").WithJSONBody(dto.ChangePasswordRequest{CurrentPassword: authPassword, NewPassword: authNewPassword}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_ChangePasswordMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		body   dto.ChangePasswordRequest
		err    error
		status int
		want   string
	}{
		{name: "no current password", body: dto.ChangePasswordRequest{NewPassword: authNewPassword}, status: http.StatusBadRequest, want: `{"error":"current password and new password are required"}`},
		{name: "a wrong current password", body: dto.ChangePasswordRequest{CurrentPassword: "wrong", NewPassword: authNewPassword}, err: user.ErrIncorrectPassword, status: http.StatusForbidden, want: `{"error":"the current password is incorrect","code":"incorrect_password"}`},
		{name: "a short new password", body: dto.ChangePasswordRequest{CurrentPassword: authPassword, NewPassword: "short"}, err: user.ErrPasswordTooShort, status: http.StatusBadRequest, want: `{"error":"the password is too short","code":"password_too_short"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			if tc.err != nil {
				h.auth.EXPECT().ChangePassword(mock.Anything, mock.Anything).Return(tc.err)
			}

			// when
			rec := h.NewRequest(http.MethodPut, "/api/v1/auth/password").WithJSONBody(tc.body).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestAuthController_SetEmailMapsEveryRefusal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{name: "a wrong password", err: user.ErrIncorrectPassword, status: http.StatusForbidden, want: `{"error":"the current password is incorrect","code":"incorrect_password"}`},
		{name: "an address in use", err: auth.ErrEmailTaken, status: http.StatusConflict, want: `{"error":"that email address is already in use"}`},
		{name: "the same address", err: auth.ErrEmailUnchanged, status: http.StatusBadRequest, want: `{"error":"that is already your email address"}`},
		{name: "no provider", err: auth.ErrEmailUnavailable, status: http.StatusServiceUnavailable, want: `{"error":"email is not available on this site","code":"email_unavailable"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			h.auth.EXPECT().SetEmail(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/auth/set-email").WithJSONBody(dto.SetEmailRequest{Email: "alice@work.example", Password: authPassword}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestAuthController_UpdateProfileReturnsTheUser(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.users.EXPECT().UpdateDisplayName(mock.Anything, spec.UserDisplayNameUpdate{UserID: userID, DisplayName: "Ally"}).Return(signedInAccount(userID), nil)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/auth/profile").WithJSONBody(dto.UpdateDisplayNameRequest{DisplayName: "Ally"}).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID, testutil.UnmarshalJSON[dto.UserResponse](t, rec.Body.Bytes()).ID)
}

func TestAuthController_UpdateProfileRefusesAnEmptyName(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.users.EXPECT().UpdateDisplayName(mock.Anything, mock.Anything).Return(nil, user.ErrDisplayNameRequired)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/auth/profile").WithJSONBody(dto.UpdateDisplayNameRequest{DisplayName: "<b></b>"}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"a display name is required"}`, rec.Body.String())
}

func TestAuthController_SignOutOtherDevicesKeepsThePresentedSession(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.auth.EXPECT().SignOutOtherDevices(mock.Anything, spec.SessionDeletionExcept{UserID: userID, KeepToken: testutil.TestSessionToken}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/sign-out-other-devices").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_SignOutOtherDevicesReportsAFailure(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.auth.EXPECT().SignOutOtherDevices(mock.Anything, mock.Anything).Return(errServiceDown)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/auth/sign-out-other-devices").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to sign out the other devices"}`, rec.Body.String())
}

func TestAuthController_DeleteAccountClearsTheCookie(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.auth.EXPECT().DeleteAccount(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: authPassword}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/auth/account").WithJSONBody(dto.DeleteAccountRequest{Password: authPassword}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assertClearedCookie(t, rec)
}

func TestAuthController_DeleteAccountRefusesAWrongPassword(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.auth.EXPECT().DeleteAccount(mock.Anything, mock.Anything).Return(user.ErrIncorrectPassword)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/auth/account").WithJSONBody(dto.DeleteAccountRequest{Password: "wrong"}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"the current password is incorrect","code":"incorrect_password"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestAuthController_DeleteAccountMapsWhatTheOrganisationsRefused(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{
			name:   "the sole owner of an organisation with other members",
			err:    org.ErrSoleOwner,
			status: http.StatusConflict,
			body:   `{"error":"you own an organisation that has other members, so transfer ownership or delete it before deleting your account","code":"sole_owner"}`,
		},
		{
			name:   "an account deleted meanwhile",
			err:    org.ErrAccountNotFound,
			status: http.StatusNotFound,
			body:   `{"error":"account not found"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			h.auth.EXPECT().DeleteAccount(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: authPassword}).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodDelete, "/api/v1/auth/account").WithJSONBody(dto.DeleteAccountRequest{Password: authPassword}).WithCookie(cookie).Record()

			// then the session cookie stays, because the account does
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Header().Values("Set-Cookie"))
		})
	}
}

func TestAuthController_ListIdentitiesListsTheAccountsSignInMethods(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.identity.EXPECT().ListIdentities(mock.Anything, userID).Return(&dto.IdentityListResponse{Identities: []dto.IdentityResponse{{Provider: "google", ProviderName: "Google", Email: "alice@gmail.com", CreatedAt: "2026-09-25T12:00:00Z"}}}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/identities").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"identities":[{"provider":"google","provider_name":"Google","email":"alice@gmail.com","created_at":"2026-09-25T12:00:00Z"}]}`, rec.Body.String())
}

func TestAuthController_ListIdentitiesReportsAFailedRead(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.identity.EXPECT().ListIdentities(mock.Anything, userID).Return(nil, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/auth/identities").WithCookie(cookie).Record()

	// then a blip is never shown as an account without sign-in methods
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to list the sign-in methods"}`, rec.Body.String())
}

func TestAuthController_UnlinkIdentityMapsEveryOutcome(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{name: "an unlink", status: http.StatusNoContent},
		{name: "an unlink whose audit entry failed", err: fmt.Errorf("%w: %w", identity.ErrNotAudited, errServiceDown), status: http.StatusNoContent},
		{name: "an unlink whose notice failed", err: fmt.Errorf("%w: %w", identity.ErrNoticeNotSent, errServiceDown), status: http.StatusNoContent},
		{name: "no identity from that provider", err: fmt.Errorf("%w: %w", identity.ErrIdentityNotFound, dao.ErrNotFound), status: http.StatusNotFound, want: `{"error":"the account has no identity from that provider"}`},
		{name: "the last way to sign in", err: fmt.Errorf("%w: %w", identity.ErrLastSignInMethod, dao.ErrLastSignInMethod), status: http.StatusConflict, want: `{"error":"that is the account's last way to sign in","code":"last_sign_in_method"}`},
		{name: "an unexpected failure", err: errServiceDown, status: http.StatusInternalServerError, want: `{"error":"failed to remove the sign-in method"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAuthHarness(t, testutil.TestBaseURL, false)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			h.identity.EXPECT().Unlink(mock.Anything, spec.IdentityUnlinking{UserID: userID, Provider: model.ProviderGoogle, IP: "198.51.100.7"}).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodDelete, "/api/v1/auth/identities/google").WithCookie(cookie).WithRemoteAddr(publicPeer).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			if tc.want != "" {
				assert.JSONEq(t, tc.want, rec.Body.String())
			}
			assert.NotContains(t, rec.Body.String(), "database down")
		})
	}
}

func TestAuthController_DeleteAccountNeedsThePassword(t *testing.T) {
	// given
	h := newAuthHarness(t, testutil.TestBaseURL, false)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/auth/account").WithJSONBody(dto.DeleteAccountRequest{}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"password is required"}`, rec.Body.String())
}
