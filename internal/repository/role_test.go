package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"retrotide/internal/cache"
	"retrotide/internal/cache/engines"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	valkeymock "github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

func newCachedRoleRepo(t *testing.T) (RoleRepository, *dao.MockRoleDAO, *valkeymock.Client) {
	t.Helper()

	client := valkeymock.NewClient(gomock.NewController(t))
	roleDAO := dao.NewMockRoleDAO(t)

	return NewRoleRepo(roleDAO, cache.NewManager(engines.NewValkeyWithClient(client))), roleDAO, client
}

func TestRoleGetRole_CachesTheDaoResultForAMinute(t *testing.T) {
	// given a cold cache
	repo, roleDAO, client := newCachedRoleRepo(t)
	userID := uuid.New()
	roleDAO.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)
	commands := captureSettingsCommands(client, 2, settingsMissThenStore)

	// when
	got, err := repo.GetRole(t.Context(), userID)

	// then the miss falls through to the dao, and the fill expires, so a failed invalidation heals itself
	require.NoError(t, err)
	assert.Equal(t, role.Support, got)
	assert.Equal(t, [][]string{
		{"GET", cache.UserRole.Key(userID.String())},
		{"SET", cache.UserRole.Key(userID.String()), `"support"`, "NX", "PX", "60000"},
	}, *commands)
}

func TestRoleGetRole_ServesTheCacheWithoutTouchingTheDao(t *testing.T) {
	// given a warm cache, and a dao mock with no expectations so any call fails the test
	repo, _, client := newCachedRoleRepo(t)
	captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString(`"admin"`))
	})

	// when
	got, err := repo.GetRole(t.Context(), uuid.New())

	// then
	require.NoError(t, err)
	assert.Equal(t, role.Admin, got)
}

func TestRoleGetRole_ADaoErrorIsNeverCached(t *testing.T) {
	// given
	repo, roleDAO, client := newCachedRoleRepo(t)
	userID := uuid.New()
	readErr := errors.New("db down")
	roleDAO.EXPECT().GetRole(mock.Anything, userID).Return("", readErr)
	commands := captureSettingsCommands(client, 1, settingsMissThenStore)

	// when
	_, err := repo.GetRole(t.Context(), userID)

	// then
	require.ErrorIs(t, err, readErr)
	assert.Equal(t, [][]string{{"GET", cache.UserRole.Key(userID.String())}}, *commands)
}

func TestRoleWrites_InvalidateTheUsersKeyAfterTheWrite(t *testing.T) {
	tests := []struct {
		name   string
		expect func(roleDAO *dao.MockRoleDAO, assignment spec.RoleAssignment)
		write  func(ctx context.Context, repo RoleRepository, assignment spec.RoleAssignment) error
	}{
		{
			name: "SetRole",
			expect: func(roleDAO *dao.MockRoleDAO, assignment spec.RoleAssignment) {
				roleDAO.EXPECT().SetRole(mock.Anything, assignment).Return(nil)
			},
			write: func(ctx context.Context, repo RoleRepository, assignment spec.RoleAssignment) error {
				return repo.SetRole(ctx, assignment)
			},
		},
		{
			name: "RemoveRole",
			expect: func(roleDAO *dao.MockRoleDAO, assignment spec.RoleAssignment) {
				roleDAO.EXPECT().RemoveRole(mock.Anything, assignment).Return(nil)
			},
			write: func(ctx context.Context, repo RoleRepository, assignment spec.RoleAssignment) error {
				return repo.RemoveRole(ctx, assignment)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repo, roleDAO, client := newCachedRoleRepo(t)
			assignment := spec.RoleAssignment{UserID: uuid.New(), Role: role.Support}
			tc.expect(roleDAO, assignment)
			commands := captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
				return valkeymock.Result(valkeymock.ValkeyInt64(1))
			})

			// when
			err := tc.write(t.Context(), repo, assignment)

			// then the next permission check reads the new role from the database
			require.NoError(t, err)
			assert.Equal(t, [][]string{{"DEL", cache.UserRole.Key(assignment.UserID.String())}}, *commands)
		})
	}
}

func TestRoleWrites_AFailedWriteLeavesTheCacheAlone(t *testing.T) {
	// given
	repo, roleDAO, client := newCachedRoleRepo(t)
	assignment := spec.RoleAssignment{UserID: uuid.New(), Role: role.Support}
	roleDAO.EXPECT().RemoveRole(mock.Anything, assignment).Return(dao.ErrNotFound)
	client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	err := repo.RemoveRole(t.Context(), assignment)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestRoleWrites_ReportAFailedInvalidation(t *testing.T) {
	// given a write that commits while the cache refuses the delete
	repo, roleDAO, client := newCachedRoleRepo(t)
	assignment := spec.RoleAssignment{UserID: uuid.New(), Role: role.Admin}
	roleDAO.EXPECT().SetRole(mock.Anything, assignment).Return(nil)
	cacheErr := errors.New("valkey unreachable")
	captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.ErrorResult(cacheErr)
	})

	// when
	err := repo.SetRole(t.Context(), assignment)

	// then the admin learns the old role may be served for up to a minute
	require.ErrorIs(t, err, cacheErr)
	assert.ErrorIs(t, err, ErrStaleStaffCache)
}

func TestRoleWrites_RefuseAnInboundTransaction(t *testing.T) {
	tests := []struct {
		name  string
		write func(ctx context.Context, repo RoleRepository, tx *sql.Tx) error
	}{
		{name: "SetRole", write: func(ctx context.Context, repo RoleRepository, tx *sql.Tx) error {
			return repo.SetRole(ctx, spec.RoleAssignment{UserID: uuid.New(), Role: role.Support}, tx)
		}},
		{name: "RemoveRole", write: func(ctx context.Context, repo RoleRepository, tx *sql.Tx) error {
			return repo.RemoveRole(ctx, spec.RoleAssignment{UserID: uuid.New(), Role: role.Support}, tx)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given dao and cache mocks with no expectations, so any call fails the test
			repo, _, client := newCachedRoleRepo(t)
			client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

			// when
			err := tc.write(t.Context(), repo, new(sql.Tx))

			// then an invalidation can never run inside a transaction it did not open
			require.ErrorIs(t, err, db.ErrInboundTx)
		})
	}
}

func TestRoleAssignToNewAccount_WritesInsideTheCreatingTransactionWithoutTouchingTheCache(t *testing.T) {
	// given
	repo, roleDAO, client := newCachedRoleRepo(t)
	tx := new(sql.Tx)
	assignment := spec.RoleAssignment{UserID: uuid.New(), Role: role.SuperAdmin}
	roleDAO.EXPECT().SetRole(mock.Anything, assignment, []*sql.Tx{tx}).Return(nil)
	client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	err := repo.AssignToNewAccount(t.Context(), assignment, tx)

	// then an account created in this transaction has no cached role to clear
	require.NoError(t, err)
}

func TestRoleAssignToNewAccount_RefusesToRunOutsideATransaction(t *testing.T) {
	// given a dao mock with no expectations, so any write fails the test
	repo, _, client := newCachedRoleRepo(t)
	client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	err := repo.AssignToNewAccount(t.Context(), spec.RoleAssignment{UserID: uuid.New(), Role: role.SuperAdmin})

	// then a grant without the creating transaction could leave a stale cached role behind
	require.ErrorIs(t, err, dao.ErrTxRequired)
}
