package utils

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	os.Exit(m.Run())
}

func perform(t *testing.T, engine *gin.Engine, method string, target string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, body))

	return rec
}
