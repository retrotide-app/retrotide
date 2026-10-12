package entitlements

import (
	"errors"
	"fmt"
	"testing"

	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanFor_FreeAllowsOneTeamAndTenMembers(t *testing.T) {
	tests := []struct {
		limit model.Limit
		want  int
	}{
		{limit: model.LimitTeams, want: 1},
		{limit: model.LimitMembers, want: 10},
		{limit: model.LimitSavedTemplates, want: 3},
		{limit: model.LimitCardImageBytes, want: 5242880},
	}

	for _, tc := range tests {
		t.Run(string(tc.limit), func(t *testing.T) {
			// given
			plan, ok := PlanFor(model.PlanFree)
			require.True(t, ok)

			// when
			allowed, known := plan.Allowance(tc.limit)

			// then
			assert.True(t, known)
			assert.Equal(t, tc.want, allowed)
		})
	}
}

func TestPlanFor_AnUnknownPlanIsNotFound(t *testing.T) {
	// when
	plan, ok := PlanFor("platinum")

	// then
	assert.False(t, ok)
	assert.Nil(t, plan)
}

func TestPlanFor_HandsOutACopyOfTheCatalogue(t *testing.T) {
	// given
	first, ok := PlanFor(model.PlanFree)
	require.True(t, ok)

	// when
	first.Limits[model.LimitTeams] = 99
	second, _ := PlanFor(model.PlanFree)

	// then
	allowed, _ := second.Allowance(model.LimitTeams)
	assert.Equal(t, 1, allowed)
}

func TestPlan_AnUnknownLimitHasNoAllowance(t *testing.T) {
	// given
	plan, ok := PlanFor(model.PlanFree)
	require.True(t, ok)

	// when
	allowed, known := plan.Allowance("boards")

	// then
	assert.False(t, known)
	assert.Zero(t, allowed)
}

func TestCountsAsMember_OnlyASignedInUserHoldingAMembership(t *testing.T) {
	userID := uuid.New()
	membership := &model.OrganisationMember{OrgID: uuid.New(), UserID: userID, Role: model.OrgRoleMember}

	tests := []struct {
		name       string
		principal  model.Principal
		membership *model.OrganisationMember
		want       bool
	}{
		{name: "a user holding the membership", principal: model.Principal{Kind: model.PrincipalUser, ID: userID}, membership: membership, want: true},
		{name: "a user with no membership", principal: model.Principal{Kind: model.PrincipalUser, ID: userID}, membership: nil, want: false},
		{name: "a user holding someone else's membership", principal: model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}, membership: membership, want: false},
		{name: "a guest never counts", principal: model.Principal{Kind: "guest", ID: userID}, membership: membership, want: false},
		{name: "no principal", principal: model.Principal{}, membership: nil, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := CountsAsMember(tc.principal, tc.membership)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDeniedError_UnwrapsToUpgradeRequired(t *testing.T) {
	// given
	denied := &DeniedError{Plan: model.PlanFree, Limit: model.LimitTeams, Allowed: 1, Current: 1}

	// when
	wrapped := fmt.Errorf("create team: %w", denied)

	// then
	require.ErrorIs(t, wrapped, ErrUpgradeRequired)
	found, ok := errors.AsType[*DeniedError](wrapped)
	require.True(t, ok)
	assert.Equal(t, denied, found)
	assert.Equal(t, "upgrade required: the free plan allows 1 teams and the organisation has 1", denied.Error())
}
