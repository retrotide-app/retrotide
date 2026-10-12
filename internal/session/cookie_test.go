package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	httpsBaseURL = "https://retrotide.test"
	httpBaseURL  = "http://localhost:4323"
)

func TestCookie_UsesTheHostPrefixAndSecureWhenBaseURLIsHTTPS(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(httpsBaseURL, nil)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil)

	// when
	cookie, err := mgr.Cookie(t.Context(), testToken)

	// then
	require.NoError(t, err)
	assert.Equal(t, &http.Cookie{
		Name:     "__Host-rt_session",
		Value:    testToken,
		Path:     "/",
		MaxAge:   testDurationDays * 24 * 60 * 60,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}, cookie)
}

func TestCookie_UsesTheBareNameWithoutSecureWhenBaseURLIsHTTP(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(httpBaseURL, nil)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil)

	// when
	cookie, err := mgr.Cookie(t.Context(), testToken)

	// then
	require.NoError(t, err)
	assert.Equal(t, &http.Cookie{
		Name:     "rt_session",
		Value:    testToken,
		Path:     "/",
		MaxAge:   testDurationDays * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}, cookie)
}

func TestGrantCookie_LastsTheConfiguredDaysWhenRemembered(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(httpsBaseURL, nil)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil)

	// when
	cookie, err := mgr.GrantCookie(t.Context(), testToken, true)

	// then
	require.NoError(t, err)
	assert.Equal(t, testDurationDays*24*60*60, cookie.MaxAge)
}

func TestGrantCookie_EndsWithTheBrowserWhenNotRemembered(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(httpsBaseURL, nil)

	// when
	cookie, err := mgr.GrantCookie(t.Context(), testToken, false)

	// then
	require.NoError(t, err)
	assert.Equal(t, &http.Cookie{
		Name:     "__Host-rt_session",
		Value:    testToken,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}, cookie)
	assert.NotContains(t, cookie.String(), "Max-Age")
}

func TestCookie_SerialisesWithHttpOnlySecureAndSameSiteLax(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(httpsBaseURL, nil)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil)
	cookie, err := mgr.Cookie(t.Context(), testToken)
	require.NoError(t, err)
	rec := httptest.NewRecorder()

	// when
	http.SetCookie(rec, cookie)

	// then
	assert.Equal(t, "__Host-rt_session="+testToken+"; Path=/; Max-Age=2592000; HttpOnly; Secure; SameSite=Lax", rec.Header().Get("Set-Cookie"))
}

func TestCookie_ReturnsTheBaseURLError(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errSettingsDown)

	// when
	cookie, err := mgr.Cookie(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Nil(t, cookie)
}

func TestCookie_RefusesABaseURLThatDoesNotParse(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("https://retrotide.test/%zz", nil)

	// when
	cookie, err := mgr.Cookie(t.Context(), testToken)

	// then
	require.ErrorContains(t, err, "parse base_url")
	assert.Nil(t, cookie)
}

func TestCookie_ReturnsTheDurationError(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(httpsBaseURL, nil)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(0, errSettingsDown)

	// when
	cookie, err := mgr.Cookie(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Nil(t, cookie)
}

func TestExpiredCookie_ClearsTheSessionCookieUnderTheSameName(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    *http.Cookie
	}{
		{
			name:    "https",
			baseURL: httpsBaseURL,
			want:    &http.Cookie{Name: "__Host-rt_session", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode},
		},
		{
			name:    "http",
			baseURL: httpBaseURL,
			want:    &http.Cookie{Name: "rt_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mgr, _, settingsSvc := newTestManager(t)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(tc.baseURL, nil)

			// when
			cookie, err := mgr.ExpiredCookie(t.Context())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, cookie)
		})
	}
}

func TestExpiredCookie_ReturnsTheBaseURLError(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errSettingsDown)

	// when
	cookie, err := mgr.ExpiredCookie(t.Context())

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Nil(t, cookie)
}

func TestToken_ReadsTheCookieForTheBaseURLsScheme(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		cookies []*http.Cookie
		want    string
	}{
		{name: "the prefixed cookie under https", baseURL: httpsBaseURL, cookies: []*http.Cookie{{Name: "__Host-rt_session", Value: testToken}}, want: testToken},
		{name: "the bare cookie under http", baseURL: httpBaseURL, cookies: []*http.Cookie{{Name: "rt_session", Value: testToken}}, want: testToken},
		{name: "a bare cookie planted under https is ignored", baseURL: httpsBaseURL, cookies: []*http.Cookie{{Name: "rt_session", Value: testToken}}, want: ""},
		{name: "a prefixed cookie under http is ignored", baseURL: httpBaseURL, cookies: []*http.Cookie{{Name: "__Host-rt_session", Value: testToken}}, want: ""},
		{name: "no cookie at all", baseURL: httpsBaseURL, want: ""},
		{name: "an empty cookie", baseURL: httpsBaseURL, cookies: []*http.Cookie{{Name: "__Host-rt_session"}}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mgr, _, settingsSvc := newTestManager(t)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(tc.baseURL, nil)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/session", nil)
			for _, cookie := range tc.cookies {
				req.AddCookie(cookie)
			}

			// when
			token, err := mgr.Token(req)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, token)
		})
	}
}

func TestToken_ReturnsTheBaseURLError(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errSettingsDown)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-rt_session", Value: testToken})

	// when
	token, err := mgr.Token(req)

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Empty(t, token)
}
