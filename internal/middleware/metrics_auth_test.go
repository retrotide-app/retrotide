package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func tokenEngine(t *testing.T, configured string, readErr error) *gin.Engine {
	t.Helper()

	svc := settings.NewMockService(t)
	svc.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return(configured, readErr).Maybe()

	guards := NewGuards(GuardDeps{Settings: svc})

	engine := newTestEngine(t)
	engine.GET("/metrics", guards.MetricsToken, func(c *gin.Context) {
		c.String(http.StatusOK, "go_goroutines 12")
	})

	return engine
}

func getWithToken(t *testing.T, engine *gin.Engine, path string, header string, value string) int {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	if header != "" {
		req.Header.Set(header, value)
	}

	return serve(engine, req).Code
}

func TestRequireMetricsToken_OpenWhenNoTokenIsConfigured(t *testing.T) {
	// given scraping must keep working until an operator opts in
	engine := tokenEngine(t, "", nil)

	// when
	status := getWithToken(t, engine, "/metrics", "", "")

	// then
	assert.Equal(t, http.StatusOK, status)
}

func TestRequireMetricsToken_RefusesWhenTheTokenCannotBeRead(t *testing.T) {
	// given
	engine := tokenEngine(t, "", errors.New("db down"))

	// when
	status := getWithToken(t, engine, "/metrics", "", "")

	// then a failed read is never taken to mean that no token is configured
	assert.Equal(t, http.StatusServiceUnavailable, status)
}

func TestRequireMetricsToken_AcceptsTheConfiguredToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{name: "a bearer token in the Authorization header", header: "Authorization", value: "Bearer s3cret"},
		{name: "explicit metrics header", header: "X-Metrics-Token", value: "s3cret"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := tokenEngine(t, "s3cret", nil)

			// when
			status := getWithToken(t, engine, "/metrics", tc.header, tc.value)

			// then
			assert.Equal(t, http.StatusOK, status)
		})
	}
}

func TestRequireMetricsToken_HidesTheEndpointWithoutTheToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{name: "no credentials at all", header: "", value: ""},
		{name: "wrong token", header: "X-Metrics-Token", value: "guess"},
		{name: "wrong bearer", header: "Authorization", value: "Bearer guess"},
		{name: "empty bearer", header: "Authorization", value: "Bearer "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := tokenEngine(t, "s3cret", nil)

			// when
			status := getWithToken(t, engine, "/metrics", tc.header, tc.value)

			// then a 404 rather than a 401, so the endpoint does not advertise that it exists
			assert.Equal(t, http.StatusNotFound, status)
		})
	}
}
