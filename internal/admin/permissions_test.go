package admin

import (
	"fmt"
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/authz"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func rolePermissionsUpdate(actorID uuid.UUID, target role.Role, permissions []string) spec.StaffRolePermissionsUpdate {
	return spec.StaffRolePermissionsUpdate{StaffAction: staffAction(actorID), Role: target, Permissions: permissions}
}

func TestGetPermissions_ListsTheAssignableCatalogueAndSupportsGrants(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(map[role.Role][]string{
		role.Support: {"view_users", "ban_user", "manage_settings", "retired_permission", "ban_user"},
	}, nil)

	// when
	got, err := svc.GetPermissions(t.Context())

	// then restricted and unknown stored entries never reach the form, so saving it back cannot fail on them
	require.NoError(t, err)
	assert.Equal(t, []dto.RolePermissionsItem{{Role: role.Support, Label: "Support", Permissions: []string{"ban_user", "view_users"}}}, got.Roles)

	catalogue := make([]string, 0, len(got.Permissions))
	for _, item := range got.Permissions {
		catalogue = append(catalogue, item.Permission)
		assert.NotEmpty(t, item.Label)
	}

	assert.Len(t, catalogue, len(authz.RoleAssignablePermissions()))
	assert.NotContains(t, catalogue, string(authz.PermManageSettings))
	assert.NotContains(t, catalogue, string(authz.PermManageRoles))
	assert.Contains(t, catalogue, string(authz.PermManageUserEmail))
}

func TestGetPermissions_SupportWithNothingStoredHasAnEmptyList(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(map[role.Role][]string{}, nil)

	// when
	got, err := svc.GetPermissions(t.Context())

	// then
	require.NoError(t, err)
	require.Len(t, got.Roles, 1)
	assert.NotNil(t, got.Roles[0].Permissions)
	assert.Empty(t, got.Roles[0].Permissions)
}

func TestGetPermissions_ReturnsAReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.permissions.EXPECT().GetRolePermissions(mock.Anything).Return(nil, errDatabaseDown)

	// when
	got, err := svc.GetPermissions(t.Context())

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, got)
}

func TestUpdateRolePermissions_StoresASortedDeduplicatedSet(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.permissions.EXPECT().SetRolePermissions(mock.Anything, spec.RolePermissionsUpdate{Role: role.Support, Permissions: []string{"ban_user", "view_users"}}).Return(nil)
	m.audited(audit.NewEntry{
		ActorID:    actorID,
		Action:     audit.ActionUpdateRolePermissions,
		TargetType: audit.TargetRole,
		TargetID:   "support",
		Details:    map[string]string{"permissions": "ban_user,view_users"},
	})

	// when
	err := svc.UpdateRolePermissions(t.Context(), rolePermissionsUpdate(actorID, role.Support, []string{" view_users ", "ban_user", "view_users"}))

	// then
	assert.NoError(t, err)
}

func TestUpdateRolePermissions_AnEmptySetRevokesEverything(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.permissions.EXPECT().SetRolePermissions(mock.Anything, spec.RolePermissionsUpdate{Role: role.Support, Permissions: []string{}}).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	// when
	err := svc.UpdateRolePermissions(t.Context(), rolePermissionsUpdate(actorID, role.Support, nil))

	// then
	assert.NoError(t, err)
}

func TestUpdateRolePermissions_Refuses(t *testing.T) {
	tests := []struct {
		name        string
		target      role.Role
		permissions []string
		want        error
	}{
		{name: "admin, which holds everything", target: role.Admin, permissions: []string{"view_users"}, want: ErrImmutableRole},
		{name: "super admin, which holds everything", target: role.SuperAdmin, permissions: []string{"view_users"}, want: ErrImmutableRole},
		{name: "a role outside the catalogue", target: "moderator", permissions: []string{"view_users"}, want: ErrUnknownRole},
		{name: "an unknown permission", target: role.Support, permissions: []string{"view_users", "fly"}, want: ErrUnknownPermission},
		{name: "manage_settings", target: role.Support, permissions: []string{"manage_settings"}, want: ErrRestrictedPermission},
		{name: "manage_roles", target: role.Support, permissions: []string{"manage_roles"}, want: ErrRestrictedPermission},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			err := svc.UpdateRolePermissions(t.Context(), rolePermissionsUpdate(uuid.New(), tc.target, tc.permissions))

			// then nothing is written or audited
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestUpdateRolePermissions_ReturnsAWriteFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.permissions.EXPECT().SetRolePermissions(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	err := svc.UpdateRolePermissions(t.Context(), rolePermissionsUpdate(uuid.New(), role.Support, []string{"view_users"}))

	// then nothing is audited
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrStaffCacheStale)
}

func TestUpdateRolePermissions_AStaleCacheIsReportedButTheChangeIsStillAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	stale := fmt.Errorf("permissions of role support %w: valkey unreachable", repository.ErrStaleStaffCache)
	m.permissions.EXPECT().SetRolePermissions(mock.Anything, spec.RolePermissionsUpdate{Role: role.Support, Permissions: []string{"view_users"}}).Return(stale)
	m.audited(audit.NewEntry{
		ActorID:    actorID,
		Action:     audit.ActionUpdateRolePermissions,
		TargetType: audit.TargetRole,
		TargetID:   "support",
		Details:    map[string]string{"permissions": "view_users"},
	})

	// when
	err := svc.UpdateRolePermissions(t.Context(), rolePermissionsUpdate(actorID, role.Support, []string{"view_users"}))

	// then the grants are saved, so the change is recorded, and the admin still learns of the stale cache
	assert.ErrorIs(t, err, ErrStaffCacheStale)
	assert.ErrorIs(t, err, repository.ErrStaleStaffCache)
	assert.NotErrorIs(t, err, ErrNotAudited)
}
