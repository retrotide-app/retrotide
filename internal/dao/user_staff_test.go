package dao_test

import (
	"context"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listedEmails(users []model.UserWithRole) []string {
	emails := make([]string, 0, len(users))
	for _, listed := range users {
		emails = append(emails, listed.Email)
	}

	return emails
}

func TestUserDAO_AnyExists(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	before, beforeErr := repos.User.AnyExists(t.Context())
	daotest.CreateUser(t, repos)
	after, afterErr := repos.User.AnyExists(t.Context())

	// then
	require.NoError(t, beforeErr)
	require.NoError(t, afterErr)
	assert.False(t, before)
	assert.True(t, after)
}

func TestUserDAO_LockFirstAccountClaim_NeedsATransaction(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.User.LockFirstAccountClaim(t.Context())

	// then a transaction-scoped lock taken outside one would guard nothing
	require.ErrorIs(t, err, dao.ErrTxRequired)
}

func TestUserDAO_GetForUpdate_NeedsATransaction(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	_, err := repos.User.GetForUpdate(t.Context(), user.ID)

	// then a row lock taken outside a transaction would be released at once
	require.ErrorIs(t, err, dao.ErrTxRequired)
}

func TestUserDAO_GetForUpdate_ReadsTheAccountInsideTheTransaction(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("locked@example.com"))
	tx, err := repos.DB().BeginTx(t.Context(), nil)
	require.NoError(t, err)

	// when
	got, getErr := repos.User.GetForUpdate(t.Context(), user.ID, tx)
	_, missingErr := repos.User.GetForUpdate(t.Context(), uuid.New(), tx)
	rollbackErr := tx.Rollback()

	// then
	require.NoError(t, getErr)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "locked@example.com", got.Email)
	require.ErrorIs(t, missingErr, dao.ErrNotFound)
	require.NoError(t, rollbackErr)
}

func TestUserDAO_ListWithRoles_NewestFirstWithEachRole(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	oldest := daotest.CreateUser(t, repos, daotest.WithEmail("oldest@example.com"))
	middle := daotest.CreateUser(t, repos, daotest.WithEmail("middle@example.com"))
	newest := daotest.CreateUser(t, repos, daotest.WithEmail("newest@example.com"))
	_, err := repos.DB().ExecContext(t.Context(), `UPDATE users SET created_at = NOW() - INTERVAL '2 days' WHERE id = $1`, oldest.ID)
	require.NoError(t, err)
	_, err = repos.DB().ExecContext(t.Context(), `UPDATE users SET created_at = NOW() - INTERVAL '1 day' WHERE id = $1`, middle.ID)
	require.NoError(t, err)
	require.NoError(t, repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: middle.ID, Role: role.Support}))

	// when
	firstPage, firstErr := repos.User.ListWithRoles(t.Context(), spec.UserListing{Limit: 2})
	secondPage, secondErr := repos.User.ListWithRoles(t.Context(), spec.UserListing{Limit: 2, Offset: 2})
	total, totalErr := repos.User.Count(t.Context(), "")

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, totalErr)
	assert.Equal(t, []string{"newest@example.com", "middle@example.com"}, listedEmails(firstPage))
	assert.Equal(t, []string{"oldest@example.com"}, listedEmails(secondPage))
	assert.Equal(t, newest.ID, firstPage[0].ID)
	assert.Empty(t, firstPage[0].Role)
	assert.Equal(t, role.Support, firstPage[1].Role)
	assert.Equal(t, 3, total)
}

func TestUserDAO_ListWithRoles_SearchesEmailAndDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		search string
		want   []string
	}{
		{name: "a fragment of an address, in any case", search: "HARBOUR", want: []string{"ada@harbour.example"}},
		{name: "a fragment of a display name", search: "grace", want: []string{"grace@example.com"}},
		{name: "a wildcard character matches only itself", search: "_", want: []string{"under_score@example.com"}},
		{name: "a percent sign matches only itself", search: "%", want: []string{}},
		{name: "no match", search: "nobody", want: []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)
			daotest.CreateUser(t, repos, daotest.WithEmail("ada@harbour.example"), daotest.WithDisplayName("Ada"))
			daotest.CreateUser(t, repos, daotest.WithEmail("grace@example.com"), daotest.WithDisplayName("Grace Hopper"))
			daotest.CreateUser(t, repos, daotest.WithEmail("under_score@example.com"), daotest.WithDisplayName("Underscore"))

			// when
			listed, err := repos.User.ListWithRoles(t.Context(), spec.UserListing{Search: tc.search, Limit: 10})
			count, countErr := repos.User.Count(t.Context(), tc.search)

			// then
			require.NoError(t, err)
			require.NoError(t, countErr)
			assert.Equal(t, tc.want, listedEmails(listed))
			assert.Equal(t, len(tc.want), count)
		})
	}
}

func TestUserDAO_BanAndUnban(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	member := daotest.CreateUser(t, repos)

	// when
	banErr := repos.User.Ban(t.Context(), spec.UserBan{UserID: member.ID, BannedBy: staff.ID, Reason: "spam"})
	banned, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)
	unbanErr := repos.User.Unban(t.Context(), member.ID)
	cleared, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)

	// then
	require.NoError(t, banErr)
	require.NotNil(t, banned.BannedAt)
	assert.WithinDuration(t, time.Now(), *banned.BannedAt, time.Minute)
	require.NotNil(t, banned.BannedBy)
	assert.Equal(t, staff.ID, *banned.BannedBy)
	assert.Equal(t, "spam", banned.BanReason)
	require.NoError(t, unbanErr)
	assert.Nil(t, cleared.BannedAt)
	assert.Nil(t, cleared.BannedBy)
	assert.Empty(t, cleared.BanReason)
}

func TestUserDAO_LockAndUnlock(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	member := daotest.CreateUser(t, repos)

	// when
	lockErr := repos.User.Lock(t.Context(), spec.UserLock{UserID: member.ID, LockedBy: staff.ID, Reason: "review"})
	locked, lockedErr := repos.User.IsLocked(t.Context(), member.ID)
	lockedRow, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)
	unlockErr := repos.User.Unlock(t.Context(), member.ID)
	unlocked, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)

	// then
	require.NoError(t, lockErr)
	require.NoError(t, lockedErr)
	assert.True(t, locked)
	require.NotNil(t, lockedRow.LockedBy)
	assert.Equal(t, staff.ID, *lockedRow.LockedBy)
	assert.Equal(t, "review", lockedRow.LockReason)
	require.NoError(t, unlockErr)
	assert.Nil(t, unlocked.LockedAt)
	assert.Nil(t, unlocked.LockedBy)
	assert.Empty(t, unlocked.LockReason)
}

func TestUserDAO_BanWithoutAStaffActor(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	member := daotest.CreateUser(t, repos)

	// when
	err := repos.User.Ban(t.Context(), spec.UserBan{UserID: member.ID, Reason: "automated"})

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)
	assert.NotNil(t, got.BannedAt)
	assert.Nil(t, got.BannedBy)
}

func TestUserDAO_MarkEmailUnverified(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	member := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())

	// when
	err := repos.User.MarkEmailUnverified(t.Context(), member.ID)

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)
	assert.False(t, got.EmailVerified)
}

func TestUserRepository_UnverifyAccount_EndsEverySessionOfThatUserOnly(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	member := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	staffSession := daotest.CreateSession(t, repos, staff.ID)
	daotest.CreateSession(t, repos, member.ID)
	daotest.CreateSession(t, repos, member.ID)

	// when
	err := repos.User.UnverifyAccount(t.Context(), member.ID)

	// then an unverified account holds no session, and nobody else is signed out
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)
	assert.False(t, got.EmailVerified)
	var memberSessions int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, member.ID).Scan(&memberSessions))
	assert.Zero(t, memberSessions)
	_, err = repos.Session.Get(t.Context(), staffSession)
	require.NoError(t, err)
}

func TestUserRepository_UnverifyAccount_JoinsAnInboundTransaction(t *testing.T) {
	// given a transaction the caller rolls back
	repos := daotest.NewRepos(t)
	member := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	daotest.CreateSession(t, repos, member.ID)
	tx, err := repos.DB().BeginTx(t.Context(), nil)
	require.NoError(t, err)

	// when
	unverifyErr := repos.User.UnverifyAccount(t.Context(), member.ID, tx)
	require.NoError(t, tx.Rollback())

	// then neither the change nor the session purge survives
	require.NoError(t, unverifyErr)
	got, err := repos.User.GetByID(t.Context(), member.ID)
	require.NoError(t, err)
	assert.True(t, got.EmailVerified)
	var sessions int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, member.ID).Scan(&sessions))
	assert.Equal(t, 1, sessions)
}

func TestUserDAO_StaffWritesOnAMissingUserAreNotFound(t *testing.T) {
	missing := uuid.New()

	tests := []struct {
		name  string
		write func(ctx context.Context, repos *repository.Repositories) error
	}{
		{name: "Ban", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.Ban(ctx, spec.UserBan{UserID: missing})
		}},
		{name: "Unban", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.Unban(ctx, missing)
		}},
		{name: "Lock", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.Lock(ctx, spec.UserLock{UserID: missing})
		}},
		{name: "Unlock", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.Unlock(ctx, missing)
		}},
		{name: "MarkEmailUnverified", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.MarkEmailUnverified(ctx, missing)
		}},
		{name: "BanAccount", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.BanAccount(ctx, spec.UserBan{UserID: missing})
		}},
		{name: "UnverifyAccount", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.UnverifyAccount(ctx, missing)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)

			// when
			err := tc.write(t.Context(), repos)

			// then
			require.ErrorIs(t, err, dao.ErrNotFound)
		})
	}
}

func TestUserRepository_BanAccount_EndsEverySessionOfTheBannedUserOnly(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	member := daotest.CreateUser(t, repos)
	staffSession := daotest.CreateSession(t, repos, staff.ID)
	daotest.CreateSession(t, repos, member.ID)
	daotest.CreateSession(t, repos, member.ID)

	// when
	err := repos.User.BanAccount(t.Context(), spec.UserBan{UserID: member.ID, BannedBy: staff.ID, Reason: "spam"})

	// then
	require.NoError(t, err)
	banned, err := repos.User.IsBanned(t.Context(), member.ID)
	require.NoError(t, err)
	assert.True(t, banned)
	var memberSessions int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, member.ID).Scan(&memberSessions))
	assert.Zero(t, memberSessions)
	_, err = repos.Session.Get(t.Context(), staffSession)
	require.NoError(t, err)
}

func TestUserRepository_BanAccount_JoinsAnInboundTransaction(t *testing.T) {
	// given a transaction the caller rolls back
	repos := daotest.NewRepos(t)
	member := daotest.CreateUser(t, repos)
	daotest.CreateSession(t, repos, member.ID)
	tx, err := repos.DB().BeginTx(t.Context(), nil)
	require.NoError(t, err)

	// when
	banErr := repos.User.BanAccount(t.Context(), spec.UserBan{UserID: member.ID, Reason: "spam"}, tx)
	require.NoError(t, tx.Rollback())

	// then neither the ban nor the session purge survives
	require.NoError(t, banErr)
	banned, err := repos.User.IsBanned(t.Context(), member.ID)
	require.NoError(t, err)
	assert.False(t, banned)
	var sessions int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, member.ID).Scan(&sessions))
	assert.Equal(t, 1, sessions)
}
