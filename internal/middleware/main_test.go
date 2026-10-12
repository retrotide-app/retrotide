package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

const (
	testBaseURL = "https://retrotide.test"
	testHost    = "retrotide.test"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	os.Exit(m.Run())
}

func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()

	engine := gin.New()
	engine.Use(CacheHeaders(), Recover(), ErrorResponder())

	return engine
}

func serve(engine *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	return rec
}
