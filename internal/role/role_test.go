package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRole_RankOrdersTheStaffRoles(t *testing.T) {
	tests := []struct {
		name      string
		role      Role
		wantRank  int
		wantStaff bool
	}{
		{name: "super admin outranks everyone", role: SuperAdmin, wantRank: 3, wantStaff: true},
		{name: "admin sits below super admin", role: Admin, wantRank: 2, wantStaff: true},
		{name: "support is the lowest staff role", role: Support, wantRank: 1, wantStaff: true},
		{name: "no role is not staff", role: "", wantRank: 0, wantStaff: false},
		{name: "an unknown role is not staff", role: "moderator", wantRank: 0, wantStaff: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			r := tc.role

			// when
			rank, staff := r.Rank(), r.IsSiteStaff()

			// then
			assert.Equal(t, tc.wantRank, rank)
			assert.Equal(t, tc.wantStaff, staff)
		})
	}
}

func TestRole_OnlyTheSuperAdminBypassesRestrictions(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want bool
	}{
		{name: "super admin", role: SuperAdmin, want: true},
		{name: "admin", role: Admin, want: false},
		{name: "support", role: Support, want: false},
		{name: "no role", role: "", want: false},
		{name: "an unknown role", role: "moderator", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			r := tc.role

			// when
			bypasses := r.BypassesRestrictions()

			// then
			assert.Equal(t, tc.want, bypasses)
		})
	}
}
