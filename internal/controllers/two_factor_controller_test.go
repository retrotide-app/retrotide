package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/auth"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/cookie"
	"retrotide/internal/dto"
	"retrotide/internal/middleware"
	"retrotide/internal/model/spec"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	twoFactorHarness struct {
		*testutil.Harness
		twoFactor *twofactor.MockService
		auth      *auth.MockService
		users     *user.MockService
	}
)

const (
	twoFactorPath             = "/api/v1/auth/two-factor"
	twoFactorEnrolmentPath    = "/api/v1/auth/two-factor/enrolment"
	twoFactorConfirmationPath = "/api/v1/auth/two-factor/confirmation"
	twoFactorVerifyPath       = "/api/v1/auth/two-factor/verify"
	twoFactorCookieName       = "__Host-rt_2fa"
	twoFactorChallengeToken   = "9c1e3b5d7f9a1c3e5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5f7a9c1e"
	twoFactorSessionToken     = "1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5f7a9c1e3b5d7f9a1c3e5b7d9f1a3c"
	twoFactorEmail            = "alice@example.com"
	twoFactorPassword         = "correct horse battery"
	twoFactorCode             = "081804"
	twoFactorSecret           = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	twoFactorURI              = "otpauth://totp/RetroTide:alice@example.com?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=RetroTide&algorithm=SHA1&digits=6&period=30"
	twoFactorLocation         = "/o/acme/boards"
	twoFactorClientIP         = "198.51.100.7"
	twoFactorPeer             = twoFactorClientIP + ":4444"
)

var (
	twoFactorRecoveryCodes = []string{"abcde-fgh23", "jkmnp-qrs45"}
)

func newTwoFactorHarness(t *testing.T) *twoFactorHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	th := &twoFactorHarness{Harness: h, twoFactor: twofactor.NewMockService(t), auth: auth.NewMockService(t), users: user.NewMockService(t)}

	s := &Service{
		TwoFactorService: th.twoFactor,
		AuthService:      th.auth,
		UserService:      th.users,
		AuthzService:     h.Authz,
		Sessions:         h.Sessions,
		TwoFactorCookies: cookie.NewBuilder(h.SettingsService, twofactor.ChallengeCookieName, cookie.Fixed(twofactor.ChallengeTTL)),
		TrustedDevices:   cookie.NewBuilder(h.SettingsService, twofactor.TrustedDeviceCookieName, cookie.Fixed(twofactor.TrustedDeviceTTL)),
		Guards:           h.Guards,
	}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllTwoFactorRoutes())

	return th
}

func (h *twoFactorHarness) signInToWrite(userID uuid.UUID) *http.Cookie {
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)

	return cookie
}

func (h *twoFactorHarness) ownsChallenge(userID uuid.UUID) {
	h.twoFactor.EXPECT().ChallengeOwner(mock.Anything, twoFactorChallengeToken).Return(userID, nil)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
}

func twoFactorAccount(userID uuid.UUID) *dto.UserResponse {
	return &dto.UserResponse{ID: userID, Email: twoFactorEmail, DisplayName: "Alice", HasPassword: true, CreatedAt: "2026-09-25T12:00:00Z"}
}

func challengeCookie() *http.Cookie {
	return &http.Cookie{Name: twoFactorCookieName, Value: twoFactorChallengeToken}
}

func assertTwoFactorCookieCleared(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	cleared := testutil.ResponseCookie(t, rec, twoFactorCookieName)
	assert.Empty(t, cleared.Value)
	assert.Equal(t, -1, cleared.MaxAge)
	assert.Equal(t, "/", cleared.Path)
	assert.True(t, cleared.Secure)
	assert.True(t, cleared.HttpOnly)
}

func TestTwoFactorController_AccountRoutesRunTheAuthFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodGet, Path: twoFactorPath},
		{Method: http.MethodPost, Path: twoFactorEnrolmentPath, Body: dto.TwoFactorEnrolmentRequest{CurrentPassword: twoFactorPassword}},
		{Method: http.MethodPost, Path: twoFactorConfirmationPath, Body: dto.TwoFactorCodeRequest{Code: twoFactorCode}},
		{Method: http.MethodDelete, Path: twoFactorPath, Body: dto.TwoFactorCodeRequest{Code: twoFactorCode}},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newTwoFactorHarness(t).Harness
			}, route)
		})
	}
}

func TestTwoFactorController_StatusAnswersWhetherTwoFactorIsOn(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.twoFactor.EXPECT().Status(mock.Anything, userID).Return(&dto.TwoFactorStatusResponse{Enabled: true}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, twoFactorPath).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"enabled":true}`, rec.Body.String())
}

func TestTwoFactorController_EnrolmentNeedsTheCurrentPassword(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	cookie := h.signInToWrite(userID)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorEnrolmentPath).WithJSONBody(dto.TwoFactorEnrolmentRequest{}).WithCookie(cookie).Record()

	// then the strict mocks see no lookup and no enrolment
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"current password is required"}`, rec.Body.String())
}

func TestTwoFactorController_EnrolmentAnswersTheSecretAndItsURI(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	cookie := h.signInToWrite(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
	h.twoFactor.EXPECT().BeginEnrolment(mock.Anything, spec.TwoFactorEnrolmentStart{UserID: userID, CurrentPassword: twoFactorPassword, IP: twoFactorClientIP}).Return(&dto.TwoFactorEnrolmentResponse{Secret: twoFactorSecret, URI: twoFactorURI}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorEnrolmentPath).WithJSONBody(dto.TwoFactorEnrolmentRequest{CurrentPassword: twoFactorPassword}).WithCookie(cookie).WithRemoteAddr(twoFactorPeer).Record()

	// then
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"secret":"`+twoFactorSecret+`","otpauth_uri":"`+twoFactorURI+`"}`, rec.Body.String())
}

func TestTwoFactorController_EnrolmentMapsTheRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{name: "the wrong password", err: user.ErrIncorrectPassword, status: http.StatusForbidden, want: `{"error":"the current password is incorrect","code":"incorrect_password"}`},
		{name: "two-factor already on", err: fmt.Errorf("begin: %w", twofactor.ErrAlreadyEnabled), status: http.StatusConflict, want: `{"error":"two-factor authentication is already on","code":"two_factor_enabled"}`},
		{name: "an account without a password", err: twofactor.ErrPasswordRequired, status: http.StatusConflict, want: `{"error":"set a password before you turn on two-factor authentication","code":"password_required"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newTwoFactorHarness(t)
			userID := uuid.New()
			cookie := h.signInToWrite(userID)
			h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
			h.twoFactor.EXPECT().BeginEnrolment(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, twoFactorEnrolmentPath).WithJSONBody(dto.TwoFactorEnrolmentRequest{CurrentPassword: twoFactorPassword}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestTwoFactorController_ConfirmationAnswersTheRecoveryCodesOnce(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "audited", err: nil},
		{name: "turned on but not audited", err: fmt.Errorf("%w: %w", twofactor.ErrNotAudited, errServiceDown)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newTwoFactorHarness(t)
			userID := uuid.New()
			cookie := h.signInToWrite(userID)
			h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
			confirmation := spec.TwoFactorConfirmation{TwoFactorCodeEntry: spec.TwoFactorCodeEntry{UserID: userID, Code: twoFactorCode, IP: twoFactorClientIP}, KeepToken: cookie.Value}
			h.twoFactor.EXPECT().ConfirmEnrolment(mock.Anything, confirmation).Return(&dto.TwoFactorRecoveryCodesResponse{RecoveryCodes: twoFactorRecoveryCodes}, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, twoFactorConfirmationPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(cookie).WithRemoteAddr(twoFactorPeer).Record()

			// then the session that turned it on is the one kept
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.JSONEq(t, `{"recovery_codes":["abcde-fgh23","jkmnp-qrs45"]}`, rec.Body.String())
		})
	}
}

func TestTwoFactorController_CodeEntriesAreLimitedPerAccount(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		given  func(h *twoFactorHarness)
	}{
		{
			name:   "confirming",
			method: http.MethodPost,
			path:   twoFactorConfirmationPath,
			given: func(h *twoFactorHarness) {
				h.twoFactor.EXPECT().ConfirmEnrolment(mock.Anything, mock.Anything).Return(nil, twofactor.ErrInvalidCode)
			},
		},
		{
			name:   "turning off",
			method: http.MethodDelete,
			path:   twoFactorPath,
			given: func(h *twoFactorHarness) {
				h.twoFactor.EXPECT().Disable(mock.Anything, mock.Anything).Return(twofactor.ErrInvalidCode)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given one session guessing codes from many addresses
			h := newTwoFactorHarness(t)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
			tc.given(h)

			for i := range middleware.CredentialAttemptsPerMinute {
				rec := h.NewRequest(tc.method, tc.path).WithJSONBody(dto.TwoFactorCodeRequest{Code: "123456"}).WithCookie(cookie).WithRemoteAddr(fmt.Sprintf("198.51.100.%d:4444", i+1)).Record()
				require.Equal(t, http.StatusBadRequest, rec.Code)
			}

			// when
			rec := h.NewRequest(tc.method, tc.path).WithJSONBody(dto.TwoFactorCodeRequest{Code: "123456"}).WithCookie(cookie).WithRemoteAddr("203.0.113.9:4444").Record()

			// then
			assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		})
	}
}

func TestTwoFactorController_ConfirmationMapsTheRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{name: "the wrong code", err: twofactor.ErrInvalidCode, status: http.StatusBadRequest, want: `{"error":"that code is not right","code":"invalid_code"}`},
		{name: "no enrolment started", err: twofactor.ErrNotEnrolling, status: http.StatusConflict, want: `{"error":"start setting up two-factor authentication first","code":"two_factor_not_started"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newTwoFactorHarness(t)
			userID := uuid.New()
			cookie := h.signInToWrite(userID)
			h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
			h.twoFactor.EXPECT().ConfirmEnrolment(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, twoFactorConfirmationPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestTwoFactorController_DisableAnswersNoContent(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	cookie := h.signInToWrite(userID)
	h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
	h.twoFactor.EXPECT().Disable(mock.Anything, spec.TwoFactorCodeEntry{UserID: userID, Code: twoFactorCode, IP: twoFactorClientIP}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, twoFactorPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(cookie).WithRemoteAddr(twoFactorPeer).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTwoFactorController_DisableMapsTheRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{name: "the wrong code", err: twofactor.ErrInvalidCode, status: http.StatusBadRequest, want: `{"error":"that code is not right","code":"invalid_code"}`},
		{name: "two-factor not on", err: twofactor.ErrNotEnabled, status: http.StatusNotFound, want: `{"error":"two-factor authentication is not on","code":"two_factor_not_enabled"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newTwoFactorHarness(t)
			userID := uuid.New()
			cookie := h.signInToWrite(userID)
			h.users.EXPECT().GetByID(mock.Anything, userID).Return(twoFactorAccount(userID), nil)
			h.twoFactor.EXPECT().Disable(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodDelete, twoFactorPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestTwoFactorController_VerifyWithoutTheChallengeCookieAnswersExpired(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).Record()

	// then the strict mock sees no verification
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"signing in took too long, sign in again","code":"two_factor_expired"}`, rec.Body.String())
	assertTwoFactorCookieCleared(t, rec)
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
}

func TestTwoFactorController_VerifyOfAnExpiredChallengeClearsTheCookie(t *testing.T) {
	// given a challenge that is gone before its owner can be found, so the strict mocks see no code checked
	h := newTwoFactorHarness(t)
	h.twoFactor.EXPECT().ChallengeOwner(mock.Anything, twoFactorChallengeToken).Return(uuid.Nil, fmt.Errorf("read: %w", twofactor.ErrChallengeExpired))

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(challengeCookie()).Record()

	// then
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"signing in took too long, sign in again","code":"two_factor_expired"}`, rec.Body.String())
	assertTwoFactorCookieCleared(t, rec)
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
}

func TestTwoFactorController_VerifyOfAChallengeThatExpiresWhileTheCodeIsCheckedClearsTheCookie(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	h.ownsChallenge(uuid.New())
	h.auth.EXPECT().CompleteTwoFactor(mock.Anything, spec.TwoFactorVerification{ChallengeToken: twoFactorChallengeToken, Code: twoFactorCode}).Return(nil, fmt.Errorf("attempt: %w", twofactor.ErrChallengeExpired))

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(challengeCookie()).Record()

	// then
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assertTwoFactorCookieCleared(t, rec)
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
}

func TestTwoFactorController_VerifyGuessesAreLimitedPerAccountAcrossSignIns(t *testing.T) {
	// given one account's codes guessed from many addresses, each with a fresh sign-in
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	h.ownsChallenge(userID)
	h.auth.EXPECT().CompleteTwoFactor(mock.Anything, mock.Anything).Return(nil, twofactor.ErrInvalidCode)
	for i := range middleware.CredentialAttemptsPerMinute {
		rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: "123456"}).WithCookie(challengeCookie()).WithRemoteAddr(fmt.Sprintf("198.51.100.%d:4444", i+1)).Record()
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: "123456"}).WithCookie(challengeCookie()).WithRemoteAddr("203.0.113.9:4444").Record()

	// then the account's own bucket refuses before the code is checked again
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestTwoFactorController_VerifyOfAWrongCodeKeepsTheChallenge(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	h.ownsChallenge(uuid.New())
	h.auth.EXPECT().CompleteTwoFactor(mock.Anything, mock.Anything).Return(nil, twofactor.ErrInvalidCode)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: "123456"}).WithCookie(challengeCookie()).Record()

	// then the browser keeps its cookie for the next try
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"that code is not right","code":"invalid_code"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestTwoFactorController_VerifyRefusesABannedAccount(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	h.ownsChallenge(uuid.New())
	h.auth.EXPECT().CompleteTwoFactor(mock.Anything, mock.Anything).Return(nil, auth.ErrUserBanned)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(challengeCookie()).Record()

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"your account has been banned"}`, rec.Body.String())
	assert.False(t, sessionCookieSet(rec, testutil.SessionCookieName))
}

func TestTwoFactorController_VerifySignsInAndClearsTheChallengeCookie(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	account := twoFactorAccount(userID)
	verification := spec.TwoFactorVerification{ChallengeToken: twoFactorChallengeToken, Code: twoFactorCode, TrustDevice: true}
	signedIn := auth.TwoFactorSignIn{User: *account, SessionToken: twoFactorSessionToken, Remember: true, TrustedDeviceToken: trustedDeviceToken, Location: twoFactorLocation}
	h.ownsChallenge(userID)
	h.auth.EXPECT().CompleteTwoFactor(mock.Anything, verification).Return(&signedIn, nil)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorVerifyRequest{Code: twoFactorCode, TrustDevice: true}).WithCookie(challengeCookie()).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	got := testutil.UnmarshalJSON[dto.TwoFactorVerifiedResponse](t, rec.Body.Bytes())
	assert.Equal(t, *account, got.User)
	assert.Equal(t, twoFactorLocation, got.Location)
	assertTwoFactorCookieCleared(t, rec)
	session := testutil.ResponseCookie(t, rec, testutil.SessionCookieName)
	assert.Equal(t, twoFactorSessionToken, session.Value)
	assert.True(t, session.HttpOnly)
	assert.True(t, session.Secure)
	assert.Equal(t, testutil.TestSessionDays*24*60*60, session.MaxAge)
	trusted := testutil.ResponseCookie(t, rec, trustedDeviceCookie)
	assert.Equal(t, trustedDeviceToken, trusted.Value)
	assert.Equal(t, trustedDeviceTTL, trusted.MaxAge)
	assert.True(t, trusted.HttpOnly)
	assert.True(t, trusted.Secure)
	assert.Equal(t, http.SameSiteLaxMode, trusted.SameSite)
}

func TestTwoFactorController_VerifyWithoutRememberMeOrTrustSetsOnlyABrowserSession(t *testing.T) {
	// given
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	h.ownsChallenge(userID)
	h.auth.EXPECT().CompleteTwoFactor(mock.Anything, spec.TwoFactorVerification{ChallengeToken: twoFactorChallengeToken, Code: twoFactorCode}).Return(&auth.TwoFactorSignIn{User: *twoFactorAccount(userID), SessionToken: twoFactorSessionToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorVerifyRequest{Code: twoFactorCode}).WithCookie(challengeCookie()).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	session := testutil.ResponseCookie(t, rec, testutil.SessionCookieName)
	assert.Equal(t, twoFactorSessionToken, session.Value)
	assert.Zero(t, session.MaxAge)
	assert.False(t, sessionCookieSet(rec, trustedDeviceCookie))
}

func TestTwoFactorController_VerifyLeavesOutAnEmptyLocation(t *testing.T) {
	// given a password sign-in, whose challenge stored no landing path
	h := newTwoFactorHarness(t)
	userID := uuid.New()
	h.ownsChallenge(userID)
	h.auth.EXPECT().CompleteTwoFactor(mock.Anything, mock.Anything).Return(&auth.TwoFactorSignIn{User: *twoFactorAccount(userID), SessionToken: twoFactorSessionToken}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, twoFactorVerifyPath).WithJSONBody(dto.TwoFactorCodeRequest{Code: twoFactorCode}).WithCookie(challengeCookie()).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"location"`)
}
