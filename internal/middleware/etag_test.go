package middleware

import (
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	etagTestBody = `{"board":"retro"}`
)

var (
	etagTestTag = fmt.Sprintf(`"%d-%d"`, len(etagTestBody), crc32.Checksum([]byte(etagTestBody), crc32.MakeTable(0xD5828281)))
)

func etagEngine(t *testing.T) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(ETag())

	board := func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(etagTestBody))
	}
	engine.GET("/board", board)
	engine.HEAD("/board", board)
	engine.POST("/board", board)
	engine.GET("/assets/*filepath", board)
	engine.GET("/missing", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "board not found"})
	})
	engine.GET("/tagged", func(c *gin.Context) {
		c.Header("ETag", `"own"`)
		c.String(http.StatusOK, "tagged")
	})
	engine.GET("/partial", func(c *gin.Context) {
		c.String(http.StatusOK, "partial")
		panic("boom")
	})
	engine.GET("/stream", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/plain")
		_, err := c.Writer.WriteString("first ")
		require.NoError(t, err)
		c.Writer.Flush()
		_, err = c.Writer.WriteString("second")
		require.NoError(t, err)
	})
	engine.GET("/cause-only", func(c *gin.Context) {
		c.Error(errRenderFailed)
	})

	download := func(c *gin.Context) {
		if _, buffered := c.Writer.(*etagWriter); buffered {
			c.String(http.StatusInternalServerError, "buffered")
			return
		}

		c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte(etagTestBody))
	}
	for _, route := range []string{boardExportRoute, orgExportRoute, selfExportRoute} {
		engine.GET(route, download)
	}

	return engine
}

func TestETag_LeavesEveryExportStreamUnbuffered(t *testing.T) {
	paths := []string{
		"/api/v1/boards/0e1f2a3b-4c5d-4e6f-a071-8c9d0e1f2a3b/export?format=csv",
		"/api/v1/orgs/5d6e7f80-91a2-43b4-b5c6-3b4c5d6e7f80/export",
		"/api/v1/auth/export",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			// given
			engine := etagEngine(t)

			// when
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))

			// then the handler writes straight through, so nothing holds the stream back to hash it
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, etagTestBody, rec.Body.String())
			assert.Empty(t, rec.Header().Get("ETag"))
		})
	}
}

func TestETag_TagsAnOKBodyWithFibersStrongTag(t *testing.T) {
	// given
	engine := etagEngine(t)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/board", nil))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, etagTestTag, rec.Header().Get("ETag"))
	assert.Equal(t, etagTestBody, rec.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestETag_AnswersAMatchingRevalidationWithNotModified(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		ifNoneMatch string
	}{
		{name: "the exact tag", method: http.MethodGet, ifNoneMatch: etagTestTag},
		{name: "the weak form of the tag", method: http.MethodGet, ifNoneMatch: "W/" + etagTestTag},
		{name: "the tag in a list", method: http.MethodGet, ifNoneMatch: `"stale", ` + etagTestTag},
		{name: "any tag", method: http.MethodGet, ifNoneMatch: "*"},
		{name: "a head request", method: http.MethodHead, ifNoneMatch: etagTestTag},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := etagEngine(t)
			req := httptest.NewRequestWithContext(t.Context(), tc.method, "/board", nil)
			req.Header.Set("If-None-Match", tc.ifNoneMatch)

			// when
			rec := serve(engine, req)

			// then
			assert.Equal(t, http.StatusNotModified, rec.Code)
			assert.Empty(t, rec.Body.String())
			assert.Equal(t, etagTestTag, rec.Header().Get("ETag"))
			assert.Empty(t, rec.Header().Get("Content-Type"))
			assert.NotEmpty(t, rec.Header().Get("Cache-Control"))
		})
	}
}

func TestETag_SendsTheBodyWhenTheTagIsStale(t *testing.T) {
	// given
	engine := etagEngine(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/board", nil)
	req.Header.Set("If-None-Match", `"stale"`)

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, etagTestBody, rec.Body.String())
}

func TestETag_LeavesOtherResponsesUntouched(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
		wantETag   string
	}{
		{name: "an error", method: http.MethodGet, path: "/missing", wantStatus: http.StatusNotFound, wantBody: `{"error":"board not found"}`},
		{name: "a hashed asset", method: http.MethodGet, path: "/assets/index-a1b2c3.js", wantStatus: http.StatusOK, wantBody: etagTestBody},
		{name: "a write", method: http.MethodPost, path: "/board", wantStatus: http.StatusOK, wantBody: etagTestBody},
		{name: "a tag the handler chose", method: http.MethodGet, path: "/tagged", wantStatus: http.StatusOK, wantBody: "tagged", wantETag: `"own"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := etagEngine(t)

			// when
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil))

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, tc.wantBody, rec.Body.String())
			assert.Equal(t, tc.wantETag, rec.Header().Get("ETag"))
		})
	}
}

func TestETag_DropsTheBufferWhenTheHandlerPanics(t *testing.T) {
	// given a handler that wrote into the buffer and then panicked
	engine := etagEngine(t)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/partial", nil))

	// then recover answers on a clean writer
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Empty(t, rec.Header().Get("ETag"))
}

func TestETag_LetsAFlushingHandlerStream(t *testing.T) {
	// given
	engine := etagEngine(t)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/stream", nil))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, rec.Flushed)
	assert.Equal(t, "first second", rec.Body.String())
	assert.Empty(t, rec.Header().Get("ETag"))
}

func TestETag_LeavesAnUnansweredCauseToTheErrorResponder(t *testing.T) {
	// given
	engine := etagEngine(t)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/cause-only", nil))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
}

func TestETag_LeavesAnUnmatchedPathToTheErrorResponder(t *testing.T) {
	// given
	engine := etagEngine(t)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nowhere", nil))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
}
