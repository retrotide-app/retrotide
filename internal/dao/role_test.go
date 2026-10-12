package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleDAO_GetRole_AnAccountWithoutARoleHasNone(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	got, err := repos.Role.GetRole(t.Context(), user.ID)

	// then no role is the ordinary state, not a missing row
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRoleDAO_SetRole_AssignsThenReplaces(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	require.NoError(t, repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.Support}))

	// when
	err := repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.Admin})

	// then an account holds one staff role at a time
	require.NoError(t, err)
	got, err := repos.Role.GetRole(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, role.Admin, got)
	var rows int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM user_roles WHERE user_id = $1`, user.ID).Scan(&rows))
	assert.Equal(t, 1, rows)
}

func TestRoleDAO_SetRole_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: uuid.New(), Role: role.Support})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestRoleDAO_RemoveRole_RemovesOnlyTheNamedRole(t *testing.T) {
	// given an account promoted to admin since the remover last looked
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	require.NoError(t, repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.Admin}))

	// when
	err := repos.Role.RemoveRole(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.Support})

	// then the stale removal matches nothing and the admin role stands
	require.ErrorIs(t, err, dao.ErrNotFound)
	got, err := repos.Role.GetRole(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, role.Admin, got)
}

func TestRoleDAO_RemoveRole_ClearsAMatchingRole(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	require.NoError(t, repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.Support}))

	// when
	err := repos.Role.RemoveRole(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.Support})

	// then
	require.NoError(t, err)
	got, err := repos.Role.GetRole(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRoleDAO_DeletingTheUserRemovesTheirRole(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	require.NoError(t, repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.Admin}))

	// when
	require.NoError(t, repos.User.Delete(t.Context(), user.ID))

	// then
	var rows int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM user_roles`).Scan(&rows))
	assert.Zero(t, rows)
}

func TestRoleRepository_AssignToNewAccount_NeedsTheCreatingTransaction(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	err := repos.Role.AssignToNewAccount(t.Context(), spec.RoleAssignment{UserID: user.ID, Role: role.SuperAdmin})

	// then
	require.ErrorIs(t, err, dao.ErrTxRequired)
	got, err := repos.Role.GetRole(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}
