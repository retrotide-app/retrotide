package utils

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"retrotide/internal/entitlements"
	"retrotide/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	internalErrorPath    = "/internal-error"
	entitlementErrorPath = "/entitlement-error"
)

func TestInternalError_AnswersInternalServerErrorAndRecordsTheWrappedCause(t *testing.T) {
	// given
	var (
		recorded []*gin.Error
		aborted  bool
	)

	engine := gin.New()
	engine.GET(internalErrorPath, func(c *gin.Context) {
		InternalError(c, "load board", errInternalErrorCause)

		recorded = slices.Clone(c.Errors)
		aborted = c.IsAborted()
	})

	// when
	rec := perform(t, engine, http.MethodGet, internalErrorPath, nil)

	// then
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"error":"load board"}`, rec.Body.String())
	require.True(t, aborted)
	require.Len(t, recorded, 1)
	require.ErrorIs(t, recorded[0].Err, errInternalErrorCause)
	require.EqualError(t, recorded[0].Err, "load board: connection refused")
}

func TestMapEntitlementError_AnswersPaymentRequiredWithTheLimitAndThePlan(t *testing.T) {
	// given a refusal wrapped on its way up from the service
	denied := &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitTeams, Allowed: 1, Current: 1}
	var (
		mapped  bool
		aborted bool
		errs    []*gin.Error
	)

	engine := gin.New()
	engine.POST(entitlementErrorPath, func(c *gin.Context) {
		mapped = MapEntitlementError(c, fmt.Errorf("create a team: %w", denied))

		aborted = c.IsAborted()
		errs = slices.Clone(c.Errors)
	})

	// when
	rec := perform(t, engine, http.MethodPost, entitlementErrorPath, nil)

	// then the SPA gets the code its upgrade prompt opens on, and an expected refusal records no cause
	require.True(t, mapped)
	require.True(t, aborted)
	require.Empty(t, errs)
	require.Equal(t, http.StatusPaymentRequired, rec.Code)
	require.JSONEq(t, `{"error":"upgrade required","code":"upgrade_required","limit":"teams","allowed":1,"current":1,"plan":"free"}`, rec.Body.String())
}

func TestMapEntitlementError_LeavesAnyOtherErrorToTheCaller(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "a plain error", err: errInternalErrorCause},
		{name: "the sentinel alone, which carries no limit", err: entitlements.ErrUpgradeRequired},
		{name: "an unknown limit", err: fmt.Errorf("%w: %q", entitlements.ErrUnknownLimit, "boards")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var (
				mapped  bool
				aborted bool
			)

			engine := gin.New()
			engine.POST(entitlementErrorPath, func(c *gin.Context) {
				mapped = MapEntitlementError(c, tc.err)
				aborted = c.IsAborted()
			})

			// when
			rec := perform(t, engine, http.MethodPost, entitlementErrorPath, nil)

			// then nothing is written, so the caller answers
			require.False(t, mapped)
			require.False(t, aborted)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Empty(t, rec.Body.String())
		})
	}
}
