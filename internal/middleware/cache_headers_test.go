package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cacheHeaderEngine(t *testing.T, status int) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	answer := func(c *gin.Context) {
		c.String(status, "body")
	}
	engine.GET("/assets/*filepath", answer)
	engine.GET("/api/v1/x", answer)
	engine.GET("/some/page", answer)
	engine.GET("/robots.txt", answer)
	engine.GET("/sitemap.xml", answer)
	engine.GET(uploadsRoute, answer)

	return engine
}

func TestCacheHeaders_KeepsAnUploadedImageInTheViewersOwnCacheOnly(t *testing.T) {
	// given an image whose path names a media ID that is never reused
	engine := cacheHeaderEngine(t, http.StatusOK)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/uploads/0e1f2a3b-4c5d-4e6f-a071-8c9d0e1f2a3b/1f2a3b4c-5d6e-4f70-b182-9d0e1f2a3b4c.webp", nil))

	// then the browser keeps it for good, and no shared cache ever stores a board's image
	assert.Equal(t, "private, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestCacheHeaders_NeverCachesAnUploadThatWasRefused(t *testing.T) {
	// given
	engine := cacheHeaderEngine(t, http.StatusNotFound)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/uploads/0e1f2a3b-4c5d-4e6f-a071-8c9d0e1f2a3b/1f2a3b4c-5d6e-4f70-b182-9d0e1f2a3b4c.webp", nil))

	// then
	assert.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
}

func TestCacheHeaders_NeverLetsAnErrorIntoASharedCache(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		status int
	}{
		{name: "a hashed asset refusal", path: "/assets/index-a1b2c3.js", status: http.StatusForbidden},
		{name: "a missing hashed asset", path: "/assets/index-a1b2c3.js", status: http.StatusNotFound},
		{name: "an api refusal", path: "/api/v1/x", status: http.StatusUnauthorized},
		{name: "an origin error on a page", path: "/some/page", status: http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a path whose success response is cacheable
			engine := cacheHeaderEngine(t, tc.status)

			// when the request is refused instead
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))

			// then cloudflare must never store one caller's rejection and serve it to everyone
			got := rec.Header().Get("Cache-Control")
			assert.Equal(t, tc.status, rec.Code)
			assert.NotContains(t, got, "public")
			assert.Contains(t, got, "no-store")
		})
	}
}

func TestCacheHeaders_LeavesHashedAssetsImmutable(t *testing.T) {
	// given
	engine := cacheHeaderEngine(t, http.StatusOK)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/index-a1b2c3.js", nil))

	// then content hashed bundles are the same bytes for everyone
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestCacheHeaders_RevalidatesTheAPI(t *testing.T) {
	// given
	engine := cacheHeaderEngine(t, http.StatusOK)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/x", nil))

	// then
	assert.Equal(t, "no-cache, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestCacheHeaders_RevalidatesTheCrawlerFiles(t *testing.T) {
	tests := []string{"/robots.txt", "/sitemap.xml"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			engine := cacheHeaderEngine(t, http.StatusOK)

			// when
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))

			// then a crawler may keep its copy and revalidate it with the etag, since base_url can change at any time
			assert.Equal(t, "no-cache, must-revalidate", rec.Header().Get("Cache-Control"))
			assert.Empty(t, rec.Header().Get("Pragma"))
		})
	}
}

func TestCacheHeaders_NeverStoresPages(t *testing.T) {
	// given
	engine := cacheHeaderEngine(t, http.StatusOK)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/some/page", nil))

	// then
	assert.Equal(t, "no-cache, no-store, must-revalidate", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
	assert.Equal(t, "0", rec.Header().Get("Expires"))
}

func TestCacheHeaders_CoversResponsesThatNeverWriteABody(t *testing.T) {
	modified := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	serveFile := func(c *gin.Context) {
		c.Header("ETag", `"v1"`)
		http.ServeContent(c.Writer, c.Request, "index-a1b2c3.js", modified, strings.NewReader("console.log(1)"))
	}

	tests := []struct {
		name        string
		method      string
		path        string
		ifNoneMatch string
		register    func(engine *gin.Engine)
		wantStatus  int
		wantCache   string
	}{
		{
			name:   "a no content answer",
			method: http.MethodDelete,
			path:   "/api/v1/x",
			register: func(engine *gin.Engine) {
				engine.DELETE("/api/v1/x", func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				})
			},
			wantStatus: http.StatusNoContent,
			wantCache:  "no-cache, must-revalidate",
		},
		{
			name:   "a head request for a file",
			method: http.MethodHead,
			path:   "/assets/index-a1b2c3.js",
			register: func(engine *gin.Engine) {
				engine.HEAD("/assets/*filepath", serveFile)
			},
			wantStatus: http.StatusOK,
			wantCache:  "public, max-age=31536000, immutable",
		},
		{
			name:        "a not modified answer for a file",
			method:      http.MethodGet,
			path:        "/assets/index-a1b2c3.js",
			ifNoneMatch: `"v1"`,
			register: func(engine *gin.Engine) {
				engine.GET("/assets/*filepath", serveFile)
			},
			wantStatus: http.StatusNotModified,
			wantCache:  "public, max-age=31536000, immutable",
		},
		{
			name:   "a panic answered by recover",
			method: http.MethodGet,
			path:   "/some/page",
			register: func(engine *gin.Engine) {
				engine.GET("/some/page", func(c *gin.Context) {
					panic("boom")
				})
			},
			wantStatus: http.StatusInternalServerError,
			wantCache:  "private, no-store",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := newTestEngine(t)
			tc.register(engine)
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			if tc.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tc.ifNoneMatch)
			}

			// when
			rec := serve(engine, req)

			// then the header goes out even though nothing wrote through the writer
			require.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, tc.wantCache, rec.Header().Get("Cache-Control"))
		})
	}
}
