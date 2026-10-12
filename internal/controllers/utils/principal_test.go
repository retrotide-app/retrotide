package utils

import (
	"net/http"
	"slices"
	"testing"

	"retrotide/internal/authctx"
	"retrotide/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	principalPath = "/principal"
	guestKind     = model.PrincipalKind("guest")
)

func principalEngine(principal *model.Principal, read gin.HandlerFunc) *gin.Engine {
	engine := gin.New()
	engine.GET(principalPath, func(c *gin.Context) {
		if principal != nil {
			c.Request = c.Request.WithContext(authctx.WithPrincipal(c.Request.Context(), *principal))
		}

		c.Next()
	}, read)

	return engine
}

func TestPrincipal_ReadsTheCallerOfEitherKind(t *testing.T) {
	tests := []struct {
		name string
		kind model.PrincipalKind
	}{
		{name: "a signed-in user", kind: model.PrincipalUser},
		{name: "a guest", kind: guestKind},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			caller := model.Principal{Kind: tc.kind, ID: uuid.New()}

			var (
				read model.Principal
				ok   bool
			)

			engine := principalEngine(&caller, func(c *gin.Context) {
				read, ok = Principal(c)
				c.Status(http.StatusNoContent)
			})

			// when
			rec := perform(t, engine, http.MethodGet, principalPath, nil)

			// then
			assert.Equal(t, http.StatusNoContent, rec.Code)
			assert.True(t, ok)
			assert.Equal(t, caller, read)
		})
	}
}

func TestPrincipal_AnswersInternalErrorOnARouteWithoutAPrincipalGuard(t *testing.T) {
	// given
	var (
		ok       bool
		recorded []*gin.Error
	)

	engine := principalEngine(nil, func(c *gin.Context) {
		_, ok = Principal(c)
		recorded = slices.Clone(c.Errors)
	})

	// when
	rec := perform(t, engine, http.MethodGet, principalPath, nil)

	// then a missing guard is a bug in the route table, never an anonymous request
	assert.False(t, ok)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to identify the caller"}`, rec.Body.String())
	require.Len(t, recorded, 1)
	assert.ErrorIs(t, recorded[0].Err, errNoPrincipal)
}

func TestUserID_ReadsTheSignedInUser(t *testing.T) {
	// given
	caller := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}

	var (
		read uuid.UUID
		ok   bool
	)

	engine := principalEngine(&caller, func(c *gin.Context) {
		read, ok = UserID(c)
		c.Status(http.StatusNoContent)
	})

	// when
	rec := perform(t, engine, http.MethodGet, principalPath, nil)

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, ok)
	assert.Equal(t, caller.ID, read)
}

func TestUserID_AnswersInternalErrorWhenTheCallerIsNotASignedInUser(t *testing.T) {
	tests := []struct {
		name      string
		principal *model.Principal
		body      string
		cause     error
	}{
		{name: "no principal guard", principal: nil, body: `{"error":"failed to identify the caller"}`, cause: errNoPrincipal},
		{name: "a guest on a user route", principal: &model.Principal{Kind: guestKind, ID: uuid.New()}, body: `{"error":"failed to identify the signed-in user"}`, cause: errNoUserPrincipal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var (
				ok       bool
				recorded []*gin.Error
			)

			engine := principalEngine(tc.principal, func(c *gin.Context) {
				_, ok = UserID(c)
				recorded = slices.Clone(c.Errors)
			})

			// when
			rec := perform(t, engine, http.MethodGet, principalPath, nil)

			// then the route table is wrong, and the cause is recorded
			assert.False(t, ok)
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			require.Len(t, recorded, 1)
			assert.ErrorIs(t, recorded[0].Err, tc.cause)
		})
	}
}
