package controllers

import (
	"net/http"
	"testing"

	"retrotide/internal/controllers/utils/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func registeredRoutes(engine *gin.Engine) []string {
	var registered []string
	for _, route := range engine.Routes() {
		registered = append(registered, route.Method+" "+route.Path)
	}

	return registered
}

func TestRegisterRoutes_AddsHeadBesideEveryGet(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	answer := func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	}

	// when
	RegisterRoutes(h.Engine, []FRoute{
		{Method: http.MethodGet, Path: "/read", Handler: answer},
		{Method: http.MethodPost, Path: "/write", Handler: answer},
	})

	// then Gin adds no HEAD route of its own, and a POST gains nothing
	assert.ElementsMatch(t, []string{"GET /read", "HEAD /read", "POST /write"}, registeredRoutes(h.Engine))
}

func TestRegisterRoutes_RoutesSharingABeforeSliceKeepTheirOwnHandlers(t *testing.T) {
	// given a guard slice with spare capacity, which a bare append would write into
	h := testutil.NewHarness(t)
	shared := make(gin.HandlersChain, 1, 4)
	shared[0] = func(c *gin.Context) {
		c.Header("X-Guard", "ran")
	}

	RegisterRoutes(h.Engine, []FRoute{
		{Method: http.MethodGet, Path: "/first", Before: shared, Handler: func(c *gin.Context) {
			c.String(http.StatusOK, "first")
		}},
		{Method: http.MethodGet, Path: "/second", Before: shared, Handler: func(c *gin.Context) {
			c.String(http.StatusOK, "second")
		}},
	})

	// when
	first := h.NewRequest(http.MethodGet, "/first").Record()
	second := h.NewRequest(http.MethodGet, "/second").Record()

	// then each route runs the shared guard and then its own handler, and the spare capacity is never written
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, "first", first.Body.String())
	assert.Equal(t, "ran", first.Header().Get("X-Guard"))
	assert.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, "second", second.Body.String())
	assert.Equal(t, "ran", second.Header().Get("X-Guard"))
	assert.Nil(t, shared[:cap(shared)][1])
}
