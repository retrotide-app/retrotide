package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetach(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		wantErr error
	}{
		{name: "a write finishes after its client has gone", method: http.MethodPost, wantErr: nil},
		{name: "a read stops when its client has gone", method: http.MethodGet, wantErr: context.Canceled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a client that disconnected before the handler ran
			var seen error

			engine := newTestEngine(t)
			engine.Use(Detach())
			engine.Handle(tc.method, "/work", func(c *gin.Context) {
				seen = c.Request.Context().Err()
				c.Status(http.StatusNoContent)
			})

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			// when
			rec := serve(engine, httptest.NewRequestWithContext(ctx, tc.method, "/work", nil))

			// then
			require.Equal(t, http.StatusNoContent, rec.Code)
			assert.ErrorIs(t, seen, tc.wantErr)
		})
	}
}
