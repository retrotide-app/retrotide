package authz

import (
	"context"
	"testing"

	"retrotide/internal/repository"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	serviceMocks struct {
		roles       *repository.MockRoleRepository
		permissions *repository.MockPermissionRepository
		users       *repository.MockUserRepository
	}

	gateCheck func(svc Service, ctx context.Context, userID uuid.UUID) (bool, error)

	gateAnswer func(mocks serviceMocks, userID uuid.UUID, answer bool, err error)

	accountGate struct {
		name   string
		answer gateAnswer
		check  gateCheck
	}
)

func newTestService(t *testing.T) (Service, serviceMocks) {
	t.Helper()

	mocks := serviceMocks{
		roles:       repository.NewMockRoleRepository(t),
		permissions: repository.NewMockPermissionRepository(t),
		users:       repository.NewMockUserRepository(t),
	}

	return NewService(mocks.roles, mocks.permissions, mocks.users), mocks
}

func seededSupportTable() map[role.Role][]string {
	stored := make([]string, 0, len(defaultSupportPermissions))
	for _, perm := range defaultSupportPermissions {
		stored = append(stored, string(perm))
	}

	return map[role.Role][]string{role.Support: stored}
}

func TestCan_AnUnknownPermissionIsAnError(t *testing.T) {
	// given role and permission mocks with no expectations, so reading either fails the test
	svc, _ := newTestService(t)

	// when
	allowed, err := svc.Can(t.Context(), uuid.New(), "teleport")

	// then a guard naming a permission nobody declared fails loudly rather than quietly denying
	require.ErrorIs(t, err, ErrUnknownPermission)
	assert.False(t, allowed)
}

func TestCan_NoUserIsDeniedWithoutAnyRead(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	allowed, err := svc.Can(t.Context(), uuid.Nil, PermViewAdminPanel)

	// then
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestCan_ARoleReadFailureIsReportedAndDenied(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return("", errDatabaseDown)

	// when
	allowed, err := svc.Can(t.Context(), userID, PermViewAdminPanel)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.False(t, allowed)
}

func TestCan_ImmutableRolesHoldEveryPermissionWithoutReadingTheTable(t *testing.T) {
	tests := []struct {
		name string
		role role.Role
		perm Permission
	}{
		{name: "super admin holds a restricted permission", role: role.SuperAdmin, perm: PermManageSettings},
		{name: "admin holds a restricted permission", role: role.Admin, perm: PermManageRoles},
		{name: "admin holds a staff permission", role: role.Admin, perm: PermDeleteAnyUser},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a permission mock with no expectations, so reading the table fails the test
			svc, mocks := newTestService(t)
			userID := uuid.New()
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(tc.role, nil)

			// when
			allowed, err := svc.Can(t.Context(), userID, tc.perm)

			// then
			require.NoError(t, err)
			assert.True(t, allowed)
		})
	}
}

func TestCan_NoRoleAndUnknownRolesAreDeniedWithoutReadingTheTable(t *testing.T) {
	tests := []struct {
		name string
		role role.Role
	}{
		{name: "an account with no role", role: ""},
		{name: "a role the code does not know", role: "moderator"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, mocks := newTestService(t)
			userID := uuid.New()
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(tc.role, nil)

			// when
			allowed, err := svc.Can(t.Context(), userID, PermViewAdminPanel)

			// then
			require.NoError(t, err)
			assert.False(t, allowed)
		})
	}
}

func TestCan_SupportUsesTheStoredTable(t *testing.T) {
	tests := []struct {
		name string
		perm Permission
		want bool
	}{
		{name: "view admin panel is seeded", perm: PermViewAdminPanel, want: true},
		{name: "view stats is seeded", perm: PermViewStats, want: true},
		{name: "view users is seeded", perm: PermViewUsers, want: true},
		{name: "ban user is seeded", perm: PermBanUser, want: true},
		{name: "manage user account is seeded", perm: PermManageUserAccount, want: true},
		{name: "manage user email is not", perm: PermManageUserEmail, want: false},
		{name: "set email verified is not", perm: PermSetEmailVerified, want: false},
		{name: "reset password is not", perm: PermResetPassword, want: false},
		{name: "delete any user is not", perm: PermDeleteAnyUser, want: false},
		{name: "view audit log is not", perm: PermViewAuditLog, want: false},
		{name: "view organisations is seeded", perm: PermViewOrganisations, want: true},
		{name: "grant entitlements is not", perm: PermGrantEntitlements, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, mocks := newTestService(t)
			userID := uuid.New()
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)
			mocks.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(seededSupportTable(), nil)

			// when
			allowed, err := svc.Can(t.Context(), userID, tc.perm)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, allowed)
		})
	}
}

func TestCan_SupportNeverHoldsARestrictedPermissionEvenIfStored(t *testing.T) {
	tests := []Permission{PermManageSettings, PermManageRoles}

	for _, perm := range tests {
		t.Run(string(perm), func(t *testing.T) {
			// given a permission mock with no expectations, so the table is never consulted
			svc, mocks := newTestService(t)
			userID := uuid.New()
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)

			// when
			allowed, err := svc.Can(t.Context(), userID, perm)

			// then a row smuggled into role_permissions cannot hand support a restricted permission
			require.NoError(t, err)
			assert.False(t, allowed)
		})
	}
}

func TestCan_AnUntickedSupportPermissionIsRevoked(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)
	mocks.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(map[role.Role][]string{role.Support: {string(PermViewAdminPanel)}}, nil)

	// when
	allowed, err := svc.Can(t.Context(), userID, PermBanUser)

	// then
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestCan_ATableReadFailureIsReportedAndDenied(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)
	mocks.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(nil, errDatabaseDown)

	// when
	allowed, err := svc.Can(t.Context(), userID, PermViewAdminPanel)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.False(t, allowed)
}

func TestEffectivePermissions(t *testing.T) {
	tests := []struct {
		name    string
		role    role.Role
		table   map[role.Role][]string
		readsIt bool
		want    []Permission
	}{
		{
			name: "admin holds the whole catalogue",
			role: role.Admin,
			want: []Permission{
				PermViewAdminPanel, PermViewStats, PermViewAuditLog, PermManageSettings, PermManageRoles, PermViewUsers, PermBanUser,
				PermManageUserAccount, PermManageUserEmail, PermSetEmailVerified, PermResetPassword, PermDeleteAnyUser, PermViewOrganisations, PermGrantEntitlements,
			},
		},
		{
			name:    "support holds the stored set, in catalogue order",
			role:    role.Support,
			table:   map[role.Role][]string{role.Support: {string(PermBanUser), string(PermViewAdminPanel)}},
			readsIt: true,
			want:    []Permission{PermViewAdminPanel, PermBanUser},
		},
		{
			name:    "support never holds a restricted or unknown permission, even if stored",
			role:    role.Support,
			table:   map[role.Role][]string{role.Support: {string(PermManageRoles), "teleport", string(PermViewUsers)}},
			readsIt: true,
			want:    []Permission{PermViewUsers},
		},
		{
			name: "an account with no role holds nothing",
			role: "",
			want: []Permission{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, mocks := newTestService(t)
			userID := uuid.New()
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(tc.role, nil)
			if tc.readsIt {
				mocks.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(tc.table, nil)
			}

			// when
			got, err := svc.EffectivePermissions(t.Context(), userID)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEffectivePermissions_NoUserHoldsNothing(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.EffectivePermissions(t.Context(), uuid.Nil)

	// then
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NotNil(t, got)
}

func TestEffectivePermissions_ReportsReadFailures(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(mocks serviceMocks, userID uuid.UUID)
	}{
		{name: "the role cannot be read", prepare: func(mocks serviceMocks, userID uuid.UUID) {
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return("", errDatabaseDown)
		}},
		{name: "the table cannot be read", prepare: func(mocks serviceMocks, userID uuid.UUID) {
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)
			mocks.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(nil, errDatabaseDown)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, mocks := newTestService(t)
			userID := uuid.New()
			tc.prepare(mocks, userID)

			// when
			got, err := svc.EffectivePermissions(t.Context(), userID)

			// then
			require.ErrorIs(t, err, errDatabaseDown)
			assert.Nil(t, got)
		})
	}
}

func TestGetRole(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(role.Admin, nil)

	// when
	got, err := svc.GetRole(t.Context(), userID)

	// then
	require.NoError(t, err)
	assert.Equal(t, role.Admin, got)
}

func TestGetRole_ReportsAReadFailure(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return("", errDatabaseDown)

	// when
	got, err := svc.GetRole(t.Context(), userID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Empty(t, got)
}

func accountGates() []accountGate {
	return []accountGate{
		{
			name: "IsBanned",
			answer: func(mocks serviceMocks, userID uuid.UUID, answer bool, err error) {
				mocks.users.EXPECT().IsBanned(mock.Anything, userID).Return(answer, err)
			},
			check: func(svc Service, ctx context.Context, userID uuid.UUID) (bool, error) {
				return svc.IsBanned(ctx, userID)
			},
		},
		{
			name: "IsLocked",
			answer: func(mocks serviceMocks, userID uuid.UUID, answer bool, err error) {
				mocks.users.EXPECT().IsLocked(mock.Anything, userID).Return(answer, err)
			},
			check: func(svc Service, ctx context.Context, userID uuid.UUID) (bool, error) {
				return svc.IsLocked(ctx, userID)
			},
		},
	}
}

func TestAccountGates_AnUnrestrictedAccountNeverReadsTheRole(t *testing.T) {
	for _, gate := range accountGates() {
		t.Run(gate.name, func(t *testing.T) {
			// given a role mock with no expectations, so reading the role fails the test
			svc, mocks := newTestService(t)
			userID := uuid.New()
			gate.answer(mocks, userID, false, nil)

			// when
			got, err := gate.check(svc, t.Context(), userID)

			// then
			require.NoError(t, err)
			assert.False(t, got)
		})
	}
}

func TestAccountGates_HoldEveryRoleButSuperAdmin(t *testing.T) {
	tests := []struct {
		name string
		role role.Role
		want bool
	}{
		{name: "super admin bypasses the restriction", role: role.SuperAdmin, want: false},
		{name: "admin is held", role: role.Admin, want: true},
		{name: "support is held", role: role.Support, want: true},
		{name: "a user without a role is held", role: "", want: true},
	}

	for _, gate := range accountGates() {
		for _, tc := range tests {
			t.Run(gate.name+": "+tc.name, func(t *testing.T) {
				// given the account row carries the restriction
				svc, mocks := newTestService(t)
				userID := uuid.New()
				gate.answer(mocks, userID, true, nil)
				mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(tc.role, nil)

				// when
				got, err := gate.check(svc, t.Context(), userID)

				// then
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}
	}
}

func TestAccountGates_FailClosed(t *testing.T) {
	tests := []struct {
		name   string
		expect func(mocks serviceMocks, userID uuid.UUID, answer gateAnswer)
	}{
		{
			name: "the account row cannot be read",
			expect: func(mocks serviceMocks, userID uuid.UUID, answer gateAnswer) {
				answer(mocks, userID, false, errDatabaseDown)
			},
		},
		{
			name: "the role of a restricted account cannot be read",
			expect: func(mocks serviceMocks, userID uuid.UUID, answer gateAnswer) {
				answer(mocks, userID, true, nil)
				mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return("", errDatabaseDown)
			},
		},
	}

	for _, gate := range accountGates() {
		for _, tc := range tests {
			t.Run(gate.name+": "+tc.name, func(t *testing.T) {
				// given
				svc, mocks := newTestService(t)
				userID := uuid.New()
				tc.expect(mocks, userID, gate.answer)

				// when
				restricted, err := gate.check(svc, t.Context(), userID)

				// then the cause is reported, and a caller that ignores it still refuses
				require.ErrorIs(t, err, errDatabaseDown)
				assert.True(t, restricted)
			})
		}
	}
}

func TestBypassesRestrictions_OnlyTheSuperAdmin(t *testing.T) {
	tests := []struct {
		name string
		role role.Role
		want bool
	}{
		{name: "super admin", role: role.SuperAdmin, want: true},
		{name: "admin", role: role.Admin, want: false},
		{name: "support", role: role.Support, want: false},
		{name: "no role", role: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, mocks := newTestService(t)
			userID := uuid.New()
			mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return(tc.role, nil)

			// when
			got, err := svc.BypassesRestrictions(t.Context(), userID)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBypassesRestrictions_AFailedRoleReadGrantsNothing(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.roles.EXPECT().GetRole(mock.Anything, userID).Return("", errDatabaseDown)

	// when
	got, err := svc.BypassesRestrictions(t.Context(), userID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.False(t, got)
}
