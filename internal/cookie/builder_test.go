package cookie

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testName       = "rt_test"
	testValue      = "9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c5b4a39281706f5e4d3c2b1a0"
	httpsBaseURL   = "https://retrotide.test"
	httpBaseURL    = "http://localhost:4323"
	testLifetime   = 10 * time.Minute
	testMaxAge     = 600
	testCookiePath = "/api/v1/auth/example/callback"
)

func newTestBuilder(t *testing.T, baseURL string, lifetime Lifetime) *Builder {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(baseURL, nil)

	return NewBuilder(settingsSvc, testName, lifetime)
}

func TestCookie_NamesAndFlagsTheCookieForTheBaseURLsScheme(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    *http.Cookie
	}{
		{
			name:    "https uses the host prefix and Secure",
			baseURL: httpsBaseURL,
			want:    &http.Cookie{Name: "__Host-rt_test", Value: testValue, Path: "/", MaxAge: testMaxAge, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode},
		},
		{
			name:    "local http uses the bare name",
			baseURL: httpBaseURL,
			want:    &http.Cookie{Name: "rt_test", Value: testValue, Path: "/", MaxAge: testMaxAge, HttpOnly: true, SameSite: http.SameSiteLaxMode},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			builder := newTestBuilder(t, tc.baseURL, Fixed(testLifetime))

			// when
			built, err := builder.Cookie(t.Context(), testValue)

			// then it has no Domain, so no sibling subdomain can read or shadow it
			require.NoError(t, err)
			assert.Equal(t, tc.want, built)
		})
	}
}

func TestCookie_SerialisesWithHttpOnlySecureAndSameSiteLax(t *testing.T) {
	// given
	builder := newTestBuilder(t, httpsBaseURL, Fixed(testLifetime))
	built, err := builder.Cookie(t.Context(), testValue)
	require.NoError(t, err)
	rec := httptest.NewRecorder()

	// when
	http.SetCookie(rec, built)

	// then
	assert.Equal(t, "__Host-rt_test="+testValue+"; Path=/; Max-Age=600; HttpOnly; Secure; SameSite=Lax", rec.Header().Get("Set-Cookie"))
}

func TestCookie_AsksTheLifetimeEachTime(t *testing.T) {
	// given a lifetime read from a setting that changes between two cookies
	lifetimes := []time.Duration{30 * 24 * time.Hour, 7 * 24 * time.Hour}
	asked := 0
	builder := newTestBuilder(t, httpsBaseURL, func(context.Context) (time.Duration, error) {
		lifetime := lifetimes[asked]
		asked++

		return lifetime, nil
	})

	// when
	first, firstErr := builder.Cookie(t.Context(), testValue)
	second, secondErr := builder.Cookie(t.Context(), testValue)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.Equal(t, 30*24*60*60, first.MaxAge)
	assert.Equal(t, 7*24*60*60, second.MaxAge)
}

func TestCookie_ReturnsTheLifetimeError(t *testing.T) {
	// given
	builder := newTestBuilder(t, httpsBaseURL, func(context.Context) (time.Duration, error) {
		return 0, errSettingsDown
	})

	// when
	built, err := builder.Cookie(t.Context(), testValue)

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Nil(t, built)
}

func TestCookie_ReportsABaseURLThatCannotBeRead(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		err     error
		says    string
	}{
		{name: "a failed read", err: errSettingsDown, says: "read base_url"},
		{name: "a value that does not parse", baseURL: "https://retrotide.test/%zz", says: "parse base_url"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the lifetime is never asked, because the name depends on the scheme
			settingsSvc := settings.NewMockService(t)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(tc.baseURL, tc.err)
			builder := NewBuilder(settingsSvc, testName, func(context.Context) (time.Duration, error) {
				t.Error("the lifetime was asked before the scheme was known")

				return testLifetime, nil
			})

			// when
			built, err := builder.Cookie(t.Context(), testValue)

			// then
			require.ErrorContains(t, err, tc.says)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			}

			assert.Nil(t, built)
		})
	}
}

func TestExpiredCookie_ClearsTheCookieUnderTheSameName(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    *http.Cookie
	}{
		{
			name:    "https",
			baseURL: httpsBaseURL,
			want:    &http.Cookie{Name: "__Host-rt_test", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode},
		},
		{
			name:    "http",
			baseURL: httpBaseURL,
			want:    &http.Cookie{Name: "rt_test", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			builder := newTestBuilder(t, tc.baseURL, Fixed(testLifetime))

			// when
			cleared, err := builder.ExpiredCookie(t.Context())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, cleared)
		})
	}
}

func TestExpiredCookie_ReturnsTheBaseURLError(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errSettingsDown)
	builder := NewBuilder(settingsSvc, testName, Fixed(testLifetime))

	// when
	cleared, err := builder.ExpiredCookie(t.Context())

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Nil(t, cleared)
}

func TestToken_ReadsOnlyTheNameForTheBaseURLsScheme(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		cookies []*http.Cookie
		want    string
	}{
		{name: "the prefixed cookie under https", baseURL: httpsBaseURL, cookies: []*http.Cookie{{Name: "__Host-rt_test", Value: testValue}}, want: testValue},
		{name: "the bare cookie under http", baseURL: httpBaseURL, cookies: []*http.Cookie{{Name: "rt_test", Value: testValue}}, want: testValue},
		{name: "a bare cookie planted under https is ignored", baseURL: httpsBaseURL, cookies: []*http.Cookie{{Name: "rt_test", Value: testValue}}, want: ""},
		{name: "a prefixed cookie under http is ignored", baseURL: httpBaseURL, cookies: []*http.Cookie{{Name: "__Host-rt_test", Value: testValue}}, want: ""},
		{name: "another cookie's value is never read", baseURL: httpsBaseURL, cookies: []*http.Cookie{{Name: "__Host-rt_other", Value: testValue}}, want: ""},
		{name: "no cookie at all", baseURL: httpsBaseURL, want: ""},
		{name: "an empty cookie", baseURL: httpsBaseURL, cookies: []*http.Cookie{{Name: "__Host-rt_test"}}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			builder := newTestBuilder(t, tc.baseURL, Fixed(testLifetime))
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testCookiePath, nil)
			for _, sent := range tc.cookies {
				request.AddCookie(sent)
			}

			// when
			read, err := builder.Token(request)

			// then a missing cookie is no token rather than an error
			require.NoError(t, err)
			assert.Equal(t, tc.want, read)
		})
	}
}

func TestToken_ReturnsTheBaseURLError(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errSettingsDown)
	builder := NewBuilder(settingsSvc, testName, Fixed(testLifetime))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testCookiePath, nil)
	request.AddCookie(&http.Cookie{Name: "__Host-rt_test", Value: testValue})

	// when
	read, err := builder.Token(request)

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Empty(t, read)
}

func TestFixed_AlwaysAnswersTheSameLifetime(t *testing.T) {
	// given
	lifetime := Fixed(testLifetime)

	// when
	first, firstErr := lifetime(t.Context())
	second, secondErr := lifetime(t.Context())

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.Equal(t, testLifetime, first)
	assert.Equal(t, testLifetime, second)
}
