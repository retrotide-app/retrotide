package dao_test

import (
	"slices"
	"testing"

	"retrotide/internal/authz"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionDAO_TheMigrationSeedsTheSupportDefaults(t *testing.T) {
	// given a freshly migrated database
	repos := daotest.NewRepos(t)
	var want []string
	for _, perm := range authz.DefaultRolePermissions(role.Support) {
		want = append(want, string(perm))
	}
	slices.Sort(want)

	// when
	got, err := repos.Permission.GetRolePermissions(t.Context())

	// then the seeded rows and the code's record of them agree
	require.NoError(t, err)
	assert.Equal(t, map[role.Role][]string{role.Support: want}, got)
}

func TestPermissionDAO_SetRolePermissions_ReplacesTheWholeSet(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Permission.SetRolePermissions(t.Context(), spec.RolePermissionsUpdate{
		Role:        role.Support,
		Permissions: []string{"view_users", "reset_password", "view_users"},
	})

	// then the old set is gone and a repeated permission is stored once
	require.NoError(t, err)
	got, err := repos.Permission.GetRolePermissions(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[role.Role][]string{role.Support: {"reset_password", "view_users"}}, got)
}

func TestPermissionDAO_SetRolePermissions_AnEmptySetClearsTheRole(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Permission.SetRolePermissions(t.Context(), spec.RolePermissionsUpdate{Role: role.Support})

	// then
	require.NoError(t, err)
	got, err := repos.Permission.GetRolePermissions(t.Context())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPermissionDAO_SetRolePermissions_OnlySupportIsEditable(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	before, err := repos.Permission.GetRolePermissions(t.Context())
	require.NoError(t, err)

	// when
	err = repos.Permission.SetRolePermissions(t.Context(), spec.RolePermissionsUpdate{Role: role.Admin, Permissions: []string{"view_users"}})

	// then the schema refuses a table for an immutable role, and nothing is half written
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role_permissions_editable_role_only")
	after, err := repos.Permission.GetRolePermissions(t.Context())
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
