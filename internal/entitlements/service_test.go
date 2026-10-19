package entitlements

import (
	"errors"
	"testing"

	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFor_EveryOrganisationIsOnFreeWhileBillingIsOff(t *testing.T) {
	// given
	svc := NewService()

	// when
	plan, err := svc.For(t.Context(), uuid.New())

	// then
	require.NoError(t, err)
	assert.Equal(t, &Plan{Code: model.PlanFree, Limits: map[model.Limit]int{model.LimitTeams: 1, model.LimitMembers: 10, model.LimitSavedTemplates: 3, model.LimitCardImageBytes: 5242880, model.LimitMediaBytes: 1073741824}}, plan)
}

func TestFor_AFallbackPlanMissingFromTheCatalogueIsAnError(t *testing.T) {
	// given
	svc := &service{fallback: "platinum"}

	// when
	plan, err := svc.For(t.Context(), uuid.New())

	// then it is never read as unlimited
	assert.ErrorIs(t, err, ErrUnknownPlan)
	assert.Nil(t, plan)
}

func TestCheckWithin_RefusesOnlyAtOrPastTheAllowance(t *testing.T) {
	tests := []struct {
		name    string
		limit   model.Limit
		current int
		allowed int
		denied  bool
	}{
		{name: "the first team", limit: model.LimitTeams, current: 0, allowed: 1, denied: false},
		{name: "a second team", limit: model.LimitTeams, current: 1, allowed: 1, denied: true},
		{name: "the tenth member", limit: model.LimitMembers, current: 9, allowed: 10, denied: false},
		{name: "an eleventh member", limit: model.LimitMembers, current: 10, allowed: 10, denied: true},
		{name: "a member past the allowance", limit: model.LimitMembers, current: 12, allowed: 10, denied: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc := NewService()

			// when
			err := svc.CheckWithin(t.Context(), spec.LimitCheck{OrgID: uuid.New(), Limit: tc.limit, Current: tc.current})

			// then
			if !tc.denied {
				assert.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, ErrUpgradeRequired)
			denied, ok := errors.AsType[*DeniedError](err)
			require.True(t, ok)
			assert.Equal(t, &DeniedError{Plan: model.PlanFree, Limit: tc.limit, Allowed: tc.allowed, Current: tc.current}, denied)
		})
	}
}

func TestCheckWithin_AnUnknownLimitIsAnErrorNotARefusal(t *testing.T) {
	// given
	svc := NewService()

	// when
	err := svc.CheckWithin(t.Context(), spec.LimitCheck{OrgID: uuid.New(), Limit: "boards", Current: 0})

	// then a limit the plan does not know is never taken for unlimited or for a paywall
	assert.ErrorIs(t, err, ErrUnknownLimit)
	assert.NotErrorIs(t, err, ErrUpgradeRequired)
}

func TestCheckWithin_ReturnsAPlanFailure(t *testing.T) {
	// given
	svc := &service{fallback: "platinum"}

	// when
	err := svc.CheckWithin(t.Context(), spec.LimitCheck{OrgID: uuid.New(), Limit: model.LimitTeams, Current: 0})

	// then
	assert.ErrorIs(t, err, ErrUnknownPlan)
	assert.NotErrorIs(t, err, ErrUpgradeRequired)
}

func TestUpgradeRequiredCode_IsTheCodeTheUpgradePromptOpensOn(t *testing.T) {
	// when
	code := UpgradeRequiredCode

	// then section 6.8 fixes the 402 code, and the SPA shows "paid plans are coming" on it while billing is off
	assert.Equal(t, "upgrade_required", code)
}
