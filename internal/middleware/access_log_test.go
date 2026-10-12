package middleware

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureLog(t *testing.T, level zerolog.Level) *bytes.Buffer {
	t.Helper()

	previousLogger := logger.Log
	previousLevel := zerolog.GlobalLevel()
	t.Cleanup(func() {
		logger.Log = previousLogger
		zerolog.SetGlobalLevel(previousLevel)
	})

	buf := &bytes.Buffer{}
	logger.Log = zerolog.New(buf)
	zerolog.SetGlobalLevel(level)

	return buf
}

func accessLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var lines []map[string]any
	for line := range bytes.Lines(buf.Bytes()) {
		var entry map[string]any
		require.NoError(t, json.Unmarshal(line, &entry))

		if entry["message"] == "request" {
			lines = append(lines, entry)
		}
	}

	return lines
}

func accessLogEngine(t *testing.T) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(AccessLog())
	answer := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	engine.GET("/items/:id", answer)
	engine.GET("/api/v1/boards/:boardID", answer)

	return engine
}

func TestAccessLog_RedactsTheSlugOfAnUnmatchedBoardPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantRoute string
	}{
		{name: "a share link", path: "/b/secret-slug/x", wantRoute: "/b/<redacted>/x"},
		{name: "a board page", path: "/boards/secret-slug", wantRoute: "/boards/<redacted>"},
		{name: "a share link typed in capitals", path: "/B/secret-slug", wantRoute: "/B/<redacted>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			buf := captureLog(t, zerolog.DebugLevel)
			engine := accessLogEngine(t)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil)
			req.RemoteAddr = "198.51.100.7:4444"

			// when
			serve(engine, req)

			// then the slug and the visitor's address never reach the log
			lines := accessLogLines(t, buf)
			require.Len(t, lines, 1)
			assert.Equal(t, tc.wantRoute, lines[0]["route"])
			assert.NotContains(t, lines[0], "ip")
			assert.NotContains(t, buf.String(), "secret-slug")
			assert.NotContains(t, buf.String(), "198.51.100.7")
		})
	}
}

func TestAccessLog_NamesAMatchedRouteByItsTemplateWithTheClientIP(t *testing.T) {
	// given
	buf := captureLog(t, zerolog.DebugLevel)
	engine := accessLogEngine(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/42", nil)
	req.RemoteAddr = "198.51.100.7:4444"

	// when
	serve(engine, req)

	// then
	lines := accessLogLines(t, buf)
	require.Len(t, lines, 1)
	assert.Equal(t, "/items/:id", lines[0]["route"])
	assert.Equal(t, "198.51.100.7", lines[0]["ip"])
	assert.Equal(t, http.MethodGet, lines[0]["method"])
	assert.InDelta(t, http.StatusOK, lines[0]["status"], 0)
	assert.Equal(t, "info", lines[0]["level"])
}

func TestAccessLog_DropsTheIPOnBoardRoutes(t *testing.T) {
	// given
	buf := captureLog(t, zerolog.DebugLevel)
	engine := accessLogEngine(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/boards/7c9e6679-7425-40de-944b-e07fc1f90ae7", nil)
	req.RemoteAddr = "198.51.100.7:4444"

	// when
	serve(engine, req)

	// then
	lines := accessLogLines(t, buf)
	require.Len(t, lines, 1)
	assert.Equal(t, "/api/v1/boards/:boardID", lines[0]["route"])
	assert.NotContains(t, lines[0], "ip")
}

func TestAccessLog_NamesAnUnmatchedAPIPathOther(t *testing.T) {
	// given
	buf := captureLog(t, zerolog.DebugLevel)
	engine := accessLogEngine(t)

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/nope", nil))

	// then
	lines := accessLogLines(t, buf)
	require.Len(t, lines, 1)
	assert.Equal(t, otherRoute, lines[0]["route"])
	assert.InDelta(t, http.StatusNotFound, lines[0]["status"], 0)
	assert.Equal(t, "warn", lines[0]["level"])
}

func TestAccessLog_RedactsSensitiveQueryValuesInWireOrder(t *testing.T) {
	// given
	buf := captureLog(t, zerolog.DebugLevel)
	engine := accessLogEngine(t)

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/42?token=abc&page=2", nil))

	// then
	lines := accessLogLines(t, buf)
	require.Len(t, lines, 1)
	assert.Equal(t, "token=<redacted>&page=2", lines[0]["query"])
	assert.NotContains(t, buf.String(), "abc")
}

func TestAccessLog_RedactsTheGoogleCallbacksCodeAndState(t *testing.T) {
	// given
	buf := captureLog(t, zerolog.DebugLevel)
	engine := newTestEngine(t)
	engine.Use(AccessLog())
	engine.GET("/api/v1/auth/google/callback", func(c *gin.Context) {
		c.Redirect(http.StatusSeeOther, "https://retrotide.test/")
	})

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/google/callback?state=Zm9vYmFyc3RhdGU&code=4%2F0AbCdEfG&scope=email+openid&authuser=0", nil))

	// then a leaked code is useless without the verifier, and still never reaches the log
	lines := accessLogLines(t, buf)
	require.Len(t, lines, 1)
	assert.Equal(t, "/api/v1/auth/google/callback", lines[0]["route"])
	assert.Equal(t, "state=<redacted>&code=<redacted>&scope=email+openid&authuser=0", lines[0]["query"])
	assert.NotContains(t, buf.String(), "Zm9vYmFyc3RhdGU")
	assert.NotContains(t, buf.String(), "0AbCdEfG")
}

func TestAccessLog_SkipsSuccessfulReadsAboveDebug(t *testing.T) {
	// given
	buf := captureLog(t, zerolog.InfoLevel)
	engine := accessLogEngine(t)

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/42", nil))
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/nope", nil))

	// then only the failed read is logged
	lines := accessLogLines(t, buf)
	require.Len(t, lines, 1)
	assert.Equal(t, otherRoute, lines[0]["route"])
}

func TestRedactQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "no query", query: "", want: ""},
		{name: "nothing sensitive", query: "page=2&sort=votes", want: "page=2&sort=votes"},
		{name: "keys are matched without case", query: "Token=abc&PASSWORD=hunter2", want: "Token=<redacted>&PASSWORD=<redacted>"},
		{name: "escaped keys are matched unescaped", query: "to%6Ben=abc", want: "to%6Ben=<redacted>"},
		{name: "every sensitive key", query: "access_token=a&session=b&key=c&secret=d", want: "access_token=<redacted>&session=<redacted>&key=<redacted>&secret=<redacted>"},
		{name: "the google callback's code and state", query: "state=s7ate&code=4%2F0Ab&scope=email+openid", want: "state=<redacted>&code=<redacted>&scope=email+openid"},
		{name: "a key without a value is kept", query: "token&page=2", want: "token&page=2"},
		{name: "empty pairs keep their place", query: "a=1&&token=x", want: "a=1&&token=<redacted>"},
		{name: "a key that does not unescape is redacted", query: "bad%zz=1&page=2", want: "bad%zz=<redacted>&page=2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			query := tc.query

			// when
			got := redactQuery(query)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
