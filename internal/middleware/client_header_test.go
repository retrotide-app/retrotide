package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireClientHeader(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		wantStatus int
	}{
		{name: "a write to the api without the header is refused", method: http.MethodPost, path: "/api/v1/x", wantStatus: http.StatusForbidden},
		{name: "a write to the api with the header passes", method: http.MethodPost, path: "/api/v1/x", headers: map[string]string{"X-RetroTide-Client": "web"}, wantStatus: http.StatusOK},
		{name: "a write to the api with an empty header is refused", method: http.MethodDelete, path: "/api/v1/x", headers: map[string]string{"X-RetroTide-Client": ""}, wantStatus: http.StatusForbidden},
		{name: "a read of the api needs no header", method: http.MethodGet, path: "/api/v1/x", wantStatus: http.StatusOK},
		{name: "a write outside the api needs no header", method: http.MethodPost, path: "/outside", wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := newTestEngine(t)
			engine.Use(RequireClientHeader())
			answer := func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			}
			engine.Any("/api/v1/x", answer)
			engine.Any("/outside", answer)
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			for name, value := range tc.headers {
				req.Header.Set(name, value)
			}

			// when
			rec := serve(engine, req)

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantStatus == http.StatusForbidden {
				assert.JSONEq(t, `{"error":"missing client header"}`, rec.Body.String())
			}
		})
	}
}
