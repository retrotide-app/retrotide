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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserDAO_CreateAndGetByID(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	created, err := repos.User.Create(t.Context(), spec.NewUser{
		Email:        "ada@example.com",
		PasswordHash: new("bcrypt-hash"),
		DisplayName:  "Ada",
	})

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, got)
	assert.Equal(t, "ada@example.com", got.Email)
	assert.Equal(t, "Ada", got.DisplayName)
	require.NotNil(t, got.PasswordHash)
	assert.Equal(t, "bcrypt-hash", *got.PasswordHash)
	assert.True(t, got.HasPassword())
	assert.False(t, got.EmailVerified)
	assert.True(t, got.EmailNotifications)
	assert.Nil(t, got.BannedAt)
	assert.Nil(t, got.LockedAt)
	assert.Nil(t, got.LastSeenIP)
}

func TestUserDAO_Create_WithoutPasswordStoresNoHash(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	created := daotest.CreateUser(t, repos, daotest.WithoutPassword(), daotest.WithVerifiedEmail())

	// then
	got, err := repos.User.GetByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Nil(t, got.PasswordHash)
	assert.False(t, got.HasPassword())
	assert.True(t, got.EmailVerified)
}

func TestUserDAO_Create_ConflictsOnAnEmailThatDiffersOnlyInCase(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos, daotest.WithEmail("Ada@Example.com"))

	// when
	_, err := repos.User.Create(t.Context(), spec.NewUser{Email: "ada@example.COM", PasswordHash: new("hash")})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Contains(t, err.Error(), "users_email_key")
}

func TestUserDAO_GetByID_NotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	_, err := repos.User.GetByID(t.Context(), uuid.New())

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserDAO_GetByEmail_IgnoresCase(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	created := daotest.CreateUser(t, repos, daotest.WithEmail("grace@example.com"))

	// when
	got, err := repos.User.GetByEmail(t.Context(), "GRACE@example.com")

	// then
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "grace@example.com", got.Email)
}

func TestUserDAO_GetByEmail_NotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos)

	// when
	_, err := repos.User.GetByEmail(t.Context(), "nobody@example.com")

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserDAO_GetByEmail_NeverMatchesAPendingChangeOfAddress(t *testing.T) {
	// given a user whose change to a new address is still waiting for its link
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("old@example.com"), daotest.WithVerifiedEmail())
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{
		TokenHash:    uuid.NewString(),
		UserID:       user.ID,
		PendingEmail: new("new@example.com"),
		ExpiresAt:    time.Now().Add(time.Hour),
	}))

	// when
	_, err := repos.User.GetByEmail(t.Context(), "new@example.com")

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserDAO_EmailInUse(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos, daotest.WithEmail("taken@example.com"))
	other := daotest.CreateUser(t, repos, daotest.WithEmail("other@example.com"))
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{
		TokenHash:    uuid.NewString(),
		UserID:       other.ID,
		PendingEmail: new("pending@example.com"),
		ExpiresAt:    time.Now().Add(time.Hour),
	}))

	cases := []struct {
		name   string
		filter spec.UserEmailFilter
		want   bool
	}{
		{"another account's address, in any case", spec.UserEmailFilter{Email: "TAKEN@example.com", ExcludeUserID: other.ID}, true},
		{"the account's own address", spec.UserEmailFilter{Email: "taken@example.com", ExcludeUserID: owner.ID}, false},
		{"an address nobody holds", spec.UserEmailFilter{Email: "free@example.com", ExcludeUserID: uuid.Nil}, false},
		{"another account's pending change of address", spec.UserEmailFilter{Email: "pending@example.com", ExcludeUserID: uuid.Nil}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			inUse, err := repos.User.EmailInUse(t.Context(), tc.filter)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, inUse)
		})
	}
}

func TestUserDAO_SetPasswordHash(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithoutPassword())

	// when
	err := repos.User.SetPasswordHash(t.Context(), spec.UserPasswordHashUpdate{UserID: user.ID, PasswordHash: "new-hash"})

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.PasswordHash)
	assert.Equal(t, "new-hash", *got.PasswordHash)
	assert.False(t, got.UpdatedAt.Before(user.UpdatedAt))
}

func TestUserDAO_SetDisplayName(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithDisplayName("Before"))

	// when
	err := repos.User.SetDisplayName(t.Context(), spec.UserDisplayNameUpdate{UserID: user.ID, DisplayName: "After"})

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "After", got.DisplayName)
}

func TestUserDAO_MarkEmailVerified(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	err := repos.User.MarkEmailVerified(t.Context(), user.ID)

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	assert.True(t, got.EmailVerified)
}

func TestUserDAO_SetVerifiedEmail_SwapsTheAddressAndVerifiesIt(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("before@example.com"))

	// when
	err := repos.User.SetVerifiedEmail(t.Context(), spec.UserVerifiedEmailUpdate{UserID: user.ID, Email: "after@example.com"})

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "after@example.com", got.Email)
	assert.True(t, got.EmailVerified)
}

func TestUserDAO_SetVerifiedEmail_ConflictsWithAnotherAccountsAddress(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("mine@example.com"))
	daotest.CreateUser(t, repos, daotest.WithEmail("theirs@example.com"))

	// when
	err := repos.User.SetVerifiedEmail(t.Context(), spec.UserVerifiedEmailUpdate{UserID: user.ID, Email: "Theirs@example.com"})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	got, getErr := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "mine@example.com", got.Email)
	assert.False(t, got.EmailVerified)
}

func TestUserDAO_WritesToAMissingUserAreNotFound(t *testing.T) {
	missing := uuid.New()

	tests := []struct {
		name  string
		write func(ctx context.Context, repos *repository.Repositories) error
	}{
		{name: "SetPasswordHash", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.SetPasswordHash(ctx, spec.UserPasswordHashUpdate{UserID: missing, PasswordHash: "hash"})
		}},
		{name: "SetDisplayName", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.SetDisplayName(ctx, spec.UserDisplayNameUpdate{UserID: missing, DisplayName: "Nobody"})
		}},
		{name: "MarkEmailVerified", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.MarkEmailVerified(ctx, missing)
		}},
		{name: "SetVerifiedEmail", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.SetVerifiedEmail(ctx, spec.UserVerifiedEmailUpdate{UserID: missing, Email: "nobody@example.com"})
		}},
		{name: "UpdateLastSeenIP", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.UpdateLastSeenIP(ctx, spec.UserLastSeenIPUpdate{UserID: missing, IP: "198.51.100.7"})
		}},
		{name: "Delete", write: func(ctx context.Context, repos *repository.Repositories) error {
			return repos.User.Delete(ctx, missing)
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

func TestUserDAO_ReadsOfAMissingUserAreNotFound(t *testing.T) {
	missing := uuid.New()

	tests := []struct {
		name string
		read func(ctx context.Context, repos *repository.Repositories) (bool, error)
	}{
		{name: "IsBanned", read: func(ctx context.Context, repos *repository.Repositories) (bool, error) {
			return repos.User.IsBanned(ctx, missing)
		}},
		{name: "IsLocked", read: func(ctx context.Context, repos *repository.Repositories) (bool, error) {
			return repos.User.IsLocked(ctx, missing)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)

			// when
			_, err := tc.read(t.Context(), repos)

			// then
			require.ErrorIs(t, err, dao.ErrNotFound)
		})
	}
}

func TestUserDAO_IsBannedAndIsLocked(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	banned := daotest.CreateUser(t, repos)
	locked := daotest.CreateUser(t, repos)
	clean := daotest.CreateUser(t, repos)
	_, err := repos.DB().ExecContext(t.Context(), `UPDATE users SET banned_at = NOW(), banned_by = $2, ban_reason = 'spam' WHERE id = $1`, banned.ID, staff.ID)
	require.NoError(t, err)
	_, err = repos.DB().ExecContext(t.Context(), `UPDATE users SET locked_at = NOW(), locked_by = $2, lock_reason = 'review' WHERE id = $1`, locked.ID, staff.ID)
	require.NoError(t, err)

	cases := []struct {
		name       string
		user       *model.User
		wantBanned bool
		wantLocked bool
	}{
		{"a banned account", banned, true, false},
		{"a locked account", locked, false, true},
		{"an account in good standing", clean, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			isBanned, banErr := repos.User.IsBanned(t.Context(), tc.user.ID)
			isLocked, lockErr := repos.User.IsLocked(t.Context(), tc.user.ID)

			// then
			require.NoError(t, banErr)
			require.NoError(t, lockErr)
			assert.Equal(t, tc.wantBanned, isBanned)
			assert.Equal(t, tc.wantLocked, isLocked)
		})
	}
}

func TestUserDAO_UpdateLastSeenIP(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	err := repos.User.UpdateLastSeenIP(t.Context(), spec.UserLastSeenIPUpdate{UserID: user.ID, IP: "198.51.100.7"})

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LastSeenIP)
	assert.Equal(t, "198.51.100.7", *got.LastSeenIP)
	require.NotNil(t, got.LastSeenIPAt)
	assert.WithinDuration(t, time.Now(), *got.LastSeenIPAt, time.Minute)
}

func TestUserDAO_ClearLastSeenIPsBefore_NullsOnlyStaleAddresses(t *testing.T) {
	// given one address recorded 40 days ago and one recorded now
	repos := daotest.NewRepos(t)
	stale := daotest.CreateUser(t, repos)
	fresh := daotest.CreateUser(t, repos)
	require.NoError(t, repos.User.UpdateLastSeenIP(t.Context(), spec.UserLastSeenIPUpdate{UserID: stale.ID, IP: "198.51.100.7"}))
	require.NoError(t, repos.User.UpdateLastSeenIP(t.Context(), spec.UserLastSeenIPUpdate{UserID: fresh.ID, IP: "203.0.113.9"}))
	_, err := repos.DB().ExecContext(t.Context(), `UPDATE users SET last_seen_ip_at = NOW() - INTERVAL '40 days' WHERE id = $1`, stale.ID)
	require.NoError(t, err)

	// when
	cleared, err := repos.User.ClearLastSeenIPsBefore(t.Context(), time.Now().Add(-30*24*time.Hour))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleared)
	staleGot, err := repos.User.GetByID(t.Context(), stale.ID)
	require.NoError(t, err)
	assert.Nil(t, staleGot.LastSeenIP)
	assert.Nil(t, staleGot.LastSeenIPAt)
	freshGot, err := repos.User.GetByID(t.Context(), fresh.ID)
	require.NoError(t, err)
	require.NotNil(t, freshGot.LastSeenIP)
	assert.Equal(t, "203.0.113.9", *freshGot.LastSeenIP)
}

func TestUserDAO_Delete_CascadesToTheAccountsOwnRows(t *testing.T) {
	// given an account with a session, both kinds of token and a notification
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	token := daotest.CreateSession(t, repos, user.ID)
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{TokenHash: "verify-hash", UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: "reset-hash", UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))
	_, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: user.ID, Type: model.NotificationOrgInvite})
	require.NoError(t, err)

	// when
	err = repos.User.Delete(t.Context(), user.ID)

	// then
	require.NoError(t, err)
	_, err = repos.User.GetByID(t.Context(), user.ID)
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, err = repos.Session.Get(t.Context(), token)
	require.ErrorIs(t, err, dao.ErrNotFound)
	for _, table := range []string{"sessions", "email_verification_tokens", "password_reset_tokens", "notifications"} {
		var remaining int
		require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM `+table+` WHERE user_id = $1`, user.ID).Scan(&remaining))
		assert.Zero(t, remaining, table)
	}
}

func TestUserDAO_Delete_DetachesRowsOtherAccountsKeep(t *testing.T) {
	// given a staff member who banned someone and notified someone else
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	banned := daotest.CreateUser(t, repos)
	recipient := daotest.CreateUser(t, repos)
	_, err := repos.DB().ExecContext(t.Context(), `UPDATE users SET banned_at = NOW(), banned_by = $2 WHERE id = $1`, banned.ID, staff.ID)
	require.NoError(t, err)
	notification, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationOrgInvite, ActorID: staff.ID})
	require.NoError(t, err)

	// when
	require.NoError(t, repos.User.Delete(t.Context(), staff.ID))

	// then the ban stands without its author, and the notification stays without its actor
	bannedGot, err := repos.User.GetByID(t.Context(), banned.ID)
	require.NoError(t, err)
	assert.NotNil(t, bannedGot.BannedAt)
	assert.Nil(t, bannedGot.BannedBy)
	listed, err := repos.Notification.ListByUser(t.Context(), spec.NotificationListing{UserID: recipient.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, notification.ID, listed[0].ID)
	assert.Nil(t, listed[0].ActorID)
}
