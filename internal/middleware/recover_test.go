package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecover_AnswersAPanicBeforeTheFirstByteWithJSON(t *testing.T) {
	// given
	engine := newTestEngine(t)
	engine.GET("/boom", func(c *gin.Context) {
		panic("boom")
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
	assert.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestRecover_DropsTheRepresentationHeadersTheHandlerLeftBehind(t *testing.T) {
	// given a handler that prepared a download and then panicked before writing it
	engine := newTestEngine(t)
	engine.GET("/export", func(c *gin.Context) {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", `attachment; filename="board.csv"`)
		panic("boom")
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/export", nil))

	// then the error is plain JSON, not a csv attachment
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Empty(t, rec.Header().Get("Content-Disposition"))
	assert.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
}

func TestRecover_AbortsTheConnectionAfterTheFirstByte(t *testing.T) {
	// given a handler that has already sent part of its response
	engine := newTestEngine(t)
	engine.GET("/partial", func(c *gin.Context) {
		c.String(http.StatusOK, "partial")
		panic("boom")
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/partial", nil)

	// when / then net/http is told to drop the connection instead of finishing a broken body
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		serve(engine, req)
	})
}

func TestRecover_PassesADeliberateAbortOn(t *testing.T) {
	// given a streamed export failing on purpose after it started
	engine := newTestEngine(t)
	engine.GET("/export", func(c *gin.Context) {
		c.Error(errRenderFailed)
		panic(http.ErrAbortHandler)
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/export", nil)

	// when / then
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		serve(engine, req)
	})
}
