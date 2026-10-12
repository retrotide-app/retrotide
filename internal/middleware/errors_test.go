package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	errDatabaseDown = errors.New("database down")
	errRenderFailed = errors.New("render failed")
)

func TestErrorResponder_AnswersACauseLeftUnanswered(t *testing.T) {
	// given a handler that recorded a cause and wrote nothing
	engine := newTestEngine(t)
	engine.GET("/render", func(c *gin.Context) {
		c.Error(errRenderFailed)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/render", nil))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
}

func TestErrorResponder_LeavesAnAnsweredErrorAlone(t *testing.T) {
	// given a handler that answered its own failure
	engine := newTestEngine(t)
	engine.GET("/conflict", func(c *gin.Context) {
		c.Error(errRenderFailed)
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "board is locked", "code": "board_locked"})
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/conflict", nil))

	// then
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"board is locked","code":"board_locked"}`, rec.Body.String())
}

func TestErrorResponder_AnswersAnUnmatchedPathWithJSON(t *testing.T) {
	// given
	engine := newTestEngine(t)
	engine.GET("/items", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nowhere", nil))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
}
