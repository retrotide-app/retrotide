package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEngine(t *testing.T, mode config.DeploymentMode) *gin.Engine {
	t.Helper()

	engine, err := NewEngine(DefaultTrustedProxies, mode)
	require.NoError(t, err)

	return engine
}

func serve(engine *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	return rec
}

func TestNewEngine_AnswersAWrongMethodWithJSON(t *testing.T) {
	// given
	engine := newTestEngine(t, config.DeploymentHosted)
	engine.GET("/items", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items", nil))

	// then
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.MethodGet, rec.Header().Get("Allow"))
	assert.JSONEq(t, `{"error":"method not allowed"}`, rec.Body.String())
}

func TestNewEngine_AnswersAPanicWithJSONThatIsNeverCached(t *testing.T) {
	// given
	engine := newTestEngine(t, config.DeploymentHosted)
	engine.GET("/boom", func(c *gin.Context) {
		panic("boom")
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil))

	// then the cache-header writer wraps Recover, which wraps ErrorResponder
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
}

func TestNewEngine_AnswersATrailingSlashWithJSONRatherThanARedirect(t *testing.T) {
	// given
	engine := newTestEngine(t, config.DeploymentHosted)
	engine.GET("/items", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/", nil))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, rec.Header().Get("Location"))
	assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
}

func TestNewEngine_ReadsTheClientIPOnlyFromATrustedPeer(t *testing.T) {
	tests := []struct {
		name       string
		mode       config.DeploymentMode
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{name: "caddy on a private address forwards the visitor", mode: config.DeploymentHosted, remoteAddr: "10.0.0.2:4444", headers: map[string]string{"CF-Connecting-IP": "198.51.100.7"}, want: "198.51.100.7"},
		{name: "a public peer cannot claim another address", mode: config.DeploymentHosted, remoteAddr: "203.0.113.9:4444", headers: map[string]string{"CF-Connecting-IP": "198.51.100.7"}, want: "203.0.113.9"},
		{name: "a trusted peer sending something that is not an address", mode: config.DeploymentHosted, remoteAddr: "10.0.0.2:4444", headers: map[string]string{"CF-Connecting-IP": "not-an-ip"}, want: "10.0.0.2"},
		{name: "x-forwarded-for is never read on the hosted site", mode: config.DeploymentHosted, remoteAddr: "10.0.0.2:4444", headers: map[string]string{"X-Forwarded-For": "198.51.100.7"}, want: "10.0.0.2"},
		{name: "a self-hosted proxy forwards the visitor", mode: config.DeploymentSelfHosted, remoteAddr: "172.18.0.2:4444", headers: map[string]string{"X-Forwarded-For": "198.51.100.7"}, want: "198.51.100.7"},
		{name: "a self-hosted visitor's own forwarded-for entry is skipped for the one the proxy appended", mode: config.DeploymentSelfHosted, remoteAddr: "172.18.0.2:4444", headers: map[string]string{"X-Forwarded-For": "203.0.113.50, 198.51.100.7"}, want: "198.51.100.7"},
		{name: "cf-connecting-ip is never read on a self-hosted instance", mode: config.DeploymentSelfHosted, remoteAddr: "172.18.0.2:4444", headers: map[string]string{"CF-Connecting-IP": "203.0.113.50"}, want: "172.18.0.2"},
		{name: "a public peer cannot claim another address on a self-hosted instance", mode: config.DeploymentSelfHosted, remoteAddr: "203.0.113.9:4444", headers: map[string]string{"X-Forwarded-For": "198.51.100.7"}, want: "203.0.113.9"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := newTestEngine(t, tc.mode)
			engine.GET("/ip", func(c *gin.Context) {
				c.String(http.StatusOK, c.ClientIP())
			})
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ip", nil)
			req.RemoteAddr = tc.remoteAddr
			for name, value := range tc.headers {
				req.Header.Set(name, value)
			}

			// when
			rec := serve(engine, req)

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tc.want, rec.Body.String())
		})
	}
}

func TestNewEngine_RefusesAnInvalidTrustedProxy(t *testing.T) {
	// given
	proxies := []string{"not-a-network"}

	// when
	engine, err := NewEngine(proxies, config.DeploymentHosted)

	// then
	require.Error(t, err)
	assert.Nil(t, engine)
}

func TestNewEngine_RefusesAnUnknownDeploymentMode(t *testing.T) {
	// given
	mode := config.DeploymentMode("somewhere")

	// when
	engine, err := NewEngine(DefaultTrustedProxies, mode)

	// then
	require.Error(t, err)
	assert.Nil(t, engine)
}
