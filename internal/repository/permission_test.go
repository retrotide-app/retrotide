package repository

import (
	"database/sql"
	"errors"
	"testing"

	"retrotide/internal/cache"
	"retrotide/internal/cache/engines"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	valkeymock "github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

func newCachedPermissionRepo(t *testing.T) (PermissionRepository, *dao.MockPermissionDAO, *valkeymock.Client) {
	t.Helper()

	client := valkeymock.NewClient(gomock.NewController(t))
	permissionDAO := dao.NewMockPermissionDAO(t)

	return NewPermissionRepo(permissionDAO, cache.NewManager(engines.NewValkeyWithClient(client))), permissionDAO, client
}

func TestPermissionGetRolePermissions_CachesTheTableForAMinute(t *testing.T) {
	// given a cold cache
	repo, permissionDAO, client := newCachedPermissionRepo(t)
	table := map[role.Role][]string{role.Support: {"ban_user", "view_users"}}
	permissionDAO.EXPECT().GetRolePermissions(mock.Anything).Return(table, nil)
	commands := captureSettingsCommands(client, 2, settingsMissThenStore)

	// when
	got, err := repo.GetRolePermissions(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, table, got)
	assert.Equal(t, [][]string{
		{"GET", cache.RolePermissions.Key()},
		{"SET", cache.RolePermissions.Key(), `{"support":["ban_user","view_users"]}`, "NX", "PX", "60000"},
	}, *commands)
}

func TestPermissionGetRolePermissions_ServesTheCacheWithoutTouchingTheDao(t *testing.T) {
	// given a warm cache, and a dao mock with no expectations so any call fails the test
	repo, _, client := newCachedPermissionRepo(t)
	captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString(`{"support":["view_admin_panel"]}`))
	})

	// when
	got, err := repo.GetRolePermissions(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, map[role.Role][]string{role.Support: {"view_admin_panel"}}, got)
}

func TestPermissionSetRolePermissions_InvalidatesTheTableAfterTheWrite(t *testing.T) {
	// given
	repo, permissionDAO, client := newCachedPermissionRepo(t)
	update := spec.RolePermissionsUpdate{Role: role.Support, Permissions: []string{"view_users"}}
	permissionDAO.EXPECT().SetRolePermissions(mock.Anything, update).Return(nil)
	commands := captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyInt64(1))
	})

	// when
	err := repo.SetRolePermissions(t.Context(), update)

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"DEL", cache.RolePermissions.Key()}}, *commands)
}

func TestPermissionSetRolePermissions_AFailedWriteLeavesTheCacheAlone(t *testing.T) {
	// given
	repo, permissionDAO, client := newCachedPermissionRepo(t)
	update := spec.RolePermissionsUpdate{Role: role.Support}
	writeErr := errors.New("db down")
	permissionDAO.EXPECT().SetRolePermissions(mock.Anything, update).Return(writeErr)
	client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	err := repo.SetRolePermissions(t.Context(), update)

	// then
	require.ErrorIs(t, err, writeErr)
}

func TestPermissionSetRolePermissions_ReportsAFailedInvalidation(t *testing.T) {
	// given a write that commits while the cache refuses the delete
	repo, permissionDAO, client := newCachedPermissionRepo(t)
	update := spec.RolePermissionsUpdate{Role: role.Support}
	permissionDAO.EXPECT().SetRolePermissions(mock.Anything, update).Return(nil)
	cacheErr := errors.New("valkey unreachable")
	captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.ErrorResult(cacheErr)
	})

	// when
	err := repo.SetRolePermissions(t.Context(), update)

	// then
	require.ErrorIs(t, err, cacheErr)
	assert.ErrorIs(t, err, ErrStaleStaffCache)
}

func TestPermissionSetRolePermissions_RefusesAnInboundTransaction(t *testing.T) {
	// given dao and cache mocks with no expectations, so any call fails the test
	repo, _, client := newCachedPermissionRepo(t)
	client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	err := repo.SetRolePermissions(t.Context(), spec.RolePermissionsUpdate{Role: role.Support}, new(sql.Tx))

	// then
	require.ErrorIs(t, err, db.ErrInboundTx)
}
