package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func securityHeadersEngine(t *testing.T, baseURL string) *gin.Engine {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(baseURL, nil).Maybe()

	engine := newTestEngine(t)
	engine.Use(SecurityHeaders(settingsSvc))
	engine.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "page")
	})

	return engine
}

func TestSecurityHeaders_KeepsBoardPathsOutOfSearchEngines(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/b/share-slug", want: "noindex, nofollow"},
		{path: "/b", want: "noindex, nofollow"},
		{path: "/boards/7c9e6679", want: "noindex, nofollow"},
		{path: "/o/acme", want: "noindex, nofollow"},
		{path: "/invite", want: "noindex, nofollow"},
		{path: "/api/v1/b/share-slug", want: "noindex, nofollow"},
		{path: "/B/share-slug", want: "noindex, nofollow"},
		{path: "/uploads/7c9e6679/1b4e28ba.webp", want: "noindex, nofollow"},
		{path: "/pricing", want: ""},
		{path: "/uploadsfolder", want: ""},
		{path: "/bold", want: ""},
		{path: "/invites", want: ""},
		{path: "/", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given
			engine := securityHeadersEngine(t, testBaseURL)

			// when
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))

			// then
			assert.Equal(t, tc.want, rec.Header().Get("X-Robots-Tag"))
		})
	}
}

func TestSecurityHeaders_SendsHSTSOnlyForAnHTTPSSite(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{name: "https", baseURL: testBaseURL, want: "max-age=15552000; includeSubDomains; preload"},
		{name: "plain http in development", baseURL: "http://localhost:4323", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := securityHeadersEngine(t, tc.baseURL)

			// when
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil))

			// then
			assert.Equal(t, tc.want, rec.Header().Get("Strict-Transport-Security"))
		})
	}
}

func TestSecurityHeaders_SetsBothContentSecurityPolicies(t *testing.T) {
	// given
	engine := securityHeadersEngine(t, testBaseURL)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil))

	// then
	assert.Equal(t, "base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; img-src 'self' data:", rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; script-src 'self' https://challenges.cloudflare.com; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-src https://challenges.cloudflare.com; worker-src 'self'", rec.Header().Get("Content-Security-Policy-Report-Only"))
}

func TestSecurityHeaders_SetsTheFixedHeaders(t *testing.T) {
	// given
	engine := securityHeadersEngine(t, testBaseURL)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil))

	// then
	for _, h := range securityHeaders {
		assert.Equal(t, h.value, rec.Header().Get(h.name), h.name)
	}
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, rec.Header().Get("Server"))
}

func TestSecurityHeaders_SetsHeadersOnAnErrorAnsweredLater(t *testing.T) {
	// given
	engine := securityHeadersEngine(t, testBaseURL)
	engine.GET("/boom", func(c *gin.Context) {
		panic("boom")
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}
