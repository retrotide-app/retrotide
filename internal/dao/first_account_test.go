package dao_test

import (
	"fmt"
	"sync"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/repository"
	"retrotide/internal/role"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	concurrentFirstAccounts = 12
)

func countRows(t *testing.T, repos *repository.Repositories, table string) int {
	t.Helper()

	var rows int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&rows))

	return rows
}

func TestUserRepository_RegisterAccount_TheFirstAccountBecomesSuperAdmin(t *testing.T) {
	// given an empty users table
	repos := daotest.NewRepos(t)
	registration := newRegistration("founder@example.com")
	registration.FirstAccount = true

	// when
	created, err := repos.User.RegisterAccount(t.Context(), registration)

	// then it can sign in and configure the site at once, with no link to wait for and no session handed out
	require.NoError(t, err)
	got, err := repos.Role.GetRole(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, role.SuperAdmin, got)
	assert.True(t, created.EmailVerified)
	stored, err := repos.User.GetByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.True(t, stored.EmailVerified)
	assert.Zero(t, countRows(t, repos, "email_verification_tokens"))
	assert.Zero(t, countRows(t, repos, "sessions"))
}

func TestUserRepository_RegisterAccount_ASecondAccountStartsUnverified(t *testing.T) {
	// given the first account exists
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())

	// when
	created, err := repos.User.RegisterAccount(t.Context(), newRegistration("second@example.com"))

	// then
	require.NoError(t, err)
	assert.False(t, created.EmailVerified)
	assert.Equal(t, 1, countRows(t, repos, "email_verification_tokens"))
}

func TestUserRepository_RegisterAccount_AFirstAccountClaimAfterAnyAccountWritesNothing(t *testing.T) {
	// given an account already exists
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos)
	registration := newRegistration("latecomer@example.com")
	registration.FirstAccount = true

	// when
	_, err := repos.User.RegisterAccount(t.Context(), registration)

	// then no account and no super admin appear
	require.ErrorIs(t, err, repository.ErrNotFirstAccount)
	_, err = repos.User.GetByEmail(t.Context(), "latecomer@example.com")
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Zero(t, countRows(t, repos, "user_roles"))
}

func TestUserRepository_RegisterAccount_OnlyOneOfManySimultaneousFirstAccountsWins(t *testing.T) {
	// given an empty users table and a burst of first registrations
	repos := daotest.NewRepos(t)
	errs := make([]error, concurrentFirstAccounts)
	wg := new(sync.WaitGroup)

	// when
	for i := range concurrentFirstAccounts {
		wg.Go(func() {
			registration := newRegistration(fmt.Sprintf("founder-%d@example.com", i))
			registration.FirstAccount = true

			_, errs[i] = repos.User.RegisterAccount(t.Context(), registration)
		})
	}
	wg.Wait()

	// then exactly one account exists, it is the only super admin, and every other claim was refused whole
	var won int
	for _, err := range errs {
		if err == nil {
			won++
			continue
		}

		require.ErrorIs(t, err, repository.ErrNotFirstAccount)
	}
	assert.Equal(t, 1, won)
	assert.Equal(t, 1, countRows(t, repos, "users"))
	assert.Zero(t, countRows(t, repos, "email_verification_tokens"))
	var superAdmins int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM user_roles WHERE role = 'super_admin'`).Scan(&superAdmins))
	assert.Equal(t, 1, superAdmins)
}

func TestUserRepository_RegisterAccount_AnOrdinaryRegistrationGrantsNoRole(t *testing.T) {
	// given an empty users table, but a registration that did not claim the first account
	repos := daotest.NewRepos(t)

	// when
	created, err := repos.User.RegisterAccount(t.Context(), newRegistration("open@example.com"))

	// then
	require.NoError(t, err)
	got, err := repos.Role.GetRole(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}
