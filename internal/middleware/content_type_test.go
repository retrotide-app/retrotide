package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRejectPlainTextBodies(t *testing.T) {
	cases := []struct {
		name            string
		method          string
		contentType     string
		contentEncoding string
		body            string
		wantStatus      int
		wantError       string
	}{
		{name: "json body passes", method: http.MethodPost, contentType: "application/json", body: `{"username":"a"}`, wantStatus: http.StatusOK},
		{name: "json body with charset passes", method: http.MethodPost, contentType: "application/json; charset=utf-8", body: `{"username":"a"}`, wantStatus: http.StatusOK},
		{name: "multipart body passes", method: http.MethodPost, contentType: "multipart/form-data; boundary=x", body: "--x--", wantStatus: http.StatusOK},
		{name: "empty body passes regardless of type", method: http.MethodPost, contentType: "text/plain", body: "", wantStatus: http.StatusOK},
		{name: "text/plain body is rejected", method: http.MethodPost, contentType: "text/plain", body: `{"username":"a","password":"b="}`, wantStatus: http.StatusUnsupportedMediaType, wantError: "unsupported content type"},
		{name: "text/plain with charset is rejected", method: http.MethodPost, contentType: "Text/Plain; charset=UTF-8", body: `{"username":"a"}`, wantStatus: http.StatusUnsupportedMediaType, wantError: "unsupported content type"},
		{name: "text/plain on PUT is rejected", method: http.MethodPut, contentType: "text/plain", body: `{}`, wantStatus: http.StatusUnsupportedMediaType, wantError: "unsupported content type"},
		{name: "a malformed content type is rejected", method: http.MethodPost, contentType: "application/json; charset", body: `{}`, wantStatus: http.StatusUnsupportedMediaType, wantError: "unsupported content type"},
		{name: "a gzip body is rejected", method: http.MethodPost, contentType: "application/json", contentEncoding: "gzip", body: `{}`, wantStatus: http.StatusUnsupportedMediaType, wantError: "unsupported content encoding"},
		{name: "an identity body passes", method: http.MethodPost, contentType: "application/json", contentEncoding: "identity", body: `{}`, wantStatus: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := newTestEngine(t)
			engine.Use(RejectPlainTextBodies())
			engine.Any("/target", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			req := httptest.NewRequestWithContext(t.Context(), tc.method, "/target", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			if tc.contentEncoding != "" {
				req.Header.Set("Content-Encoding", tc.contentEncoding)
			}

			// when
			rec := serve(engine, req)

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantError != "" {
				assert.JSONEq(t, `{"error":"`+tc.wantError+`"}`, rec.Body.String())
			}
		})
	}
}

func TestRejectPlainTextBodies_DecidesAChunkedBodyFromItsHeaders(t *testing.T) {
	// given a body of unknown length, which the middleware must not read to find out
	engine := newTestEngine(t)
	engine.Use(RejectPlainTextBodies())
	engine.POST("/target", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/target", io.MultiReader(strings.NewReader("hello")))
	req.Header.Set("Content-Type", "text/plain")
	require.Equal(t, int64(-1), req.ContentLength)

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}
