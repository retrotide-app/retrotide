package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenEmailVerification_ConsumesOnce(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Create(t.Context(), spec.NewEmailVerification{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	consumed, err := repos.EmailVerification.Consume(t.Context(), tokenHash)
	_, again := repos.EmailVerification.Consume(t.Context(), tokenHash)

	// then
	require.NoError(t, err)
	assert.Equal(t, tokenHash, consumed.TokenHash)
	assert.Equal(t, user.ID, consumed.UserID)
	assert.Nil(t, consumed.PendingEmail)
	assert.False(t, consumed.ChangesEmail())
	require.NotNil(t, consumed.UsedAt)
	assert.WithinDuration(t, time.Now(), *consumed.UsedAt, time.Minute)
	require.ErrorIs(t, again, dao.ErrNotFound)
}

func TestTokenEmailVerification_CarriesThePendingEmail(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Create(t.Context(), spec.NewEmailVerification{
		TokenHash:    tokenHash,
		UserID:       user.ID,
		PendingEmail: new("next@example.com"),
		ExpiresAt:    time.Now().Add(time.Hour),
	}))

	// when
	consumed, err := repos.EmailVerification.Consume(t.Context(), tokenHash)

	// then
	require.NoError(t, err)
	assert.True(t, consumed.ChangesEmail())
	require.NotNil(t, consumed.PendingEmail)
	assert.Equal(t, "next@example.com", *consumed.PendingEmail)
}

func TestTokenEmailVerification_ExpiredTokenIsNotConsumed(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Create(t.Context(), spec.NewEmailVerification{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(-time.Minute)}))

	// when
	_, err := repos.EmailVerification.Consume(t.Context(), tokenHash)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	var usedAt *time.Time
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT used_at FROM email_verification_tokens WHERE token_hash = $1`, tokenHash).Scan(&usedAt))
	assert.Nil(t, usedAt)
}

func TestTokenEmailVerification_UnknownTokenIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	_, err := repos.EmailVerification.Consume(t.Context(), "never-issued")

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTokenEmailVerification_IssueReplacesEveryUnusedToken(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	earlier := uuid.NewString()
	later := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{TokenHash: earlier, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	err := repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{TokenHash: later, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)})

	// then only the latest link works
	require.NoError(t, err)
	_, earlierErr := repos.EmailVerification.Consume(t.Context(), earlier)
	require.ErrorIs(t, earlierErr, dao.ErrNotFound)
	_, laterErr := repos.EmailVerification.Consume(t.Context(), later)
	require.NoError(t, laterErr)
}

func TestTokenEmailVerification_DeleteUnusedForUserKeepsUsedTokensAndOtherUsers(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	used := uuid.NewString()
	unused := uuid.NewString()
	foreign := uuid.NewString()
	for _, token := range []spec.NewEmailVerification{
		{TokenHash: used, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)},
		{TokenHash: unused, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)},
		{TokenHash: foreign, UserID: other.ID, ExpiresAt: time.Now().Add(time.Hour)},
	} {
		require.NoError(t, repos.EmailVerification.Create(t.Context(), token))
	}
	_, err := repos.EmailVerification.Consume(t.Context(), used)
	require.NoError(t, err)

	// when
	err = repos.EmailVerification.DeleteUnusedForUser(t.Context(), user.ID)

	// then
	require.NoError(t, err)
	for tokenHash, wantKept := range map[string]bool{used: true, unused: false, foreign: true} {
		var kept bool
		require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT EXISTS (SELECT 1 FROM email_verification_tokens WHERE token_hash = $1)`, tokenHash).Scan(&kept))
		assert.Equal(t, wantKept, kept, tokenHash)
	}
}

func TestTokenEmailVerification_DeletePendingEmailChangesForUserLeavesEveryOtherToken(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	pending := uuid.NewString()
	usedPending := uuid.NewString()
	registration := uuid.NewString()
	foreignPending := uuid.NewString()
	for _, token := range []spec.NewEmailVerification{
		{TokenHash: pending, UserID: user.ID, PendingEmail: new("next@example.com"), ExpiresAt: time.Now().Add(time.Hour)},
		{TokenHash: usedPending, UserID: user.ID, PendingEmail: new("earlier@example.com"), ExpiresAt: time.Now().Add(time.Hour)},
		{TokenHash: registration, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)},
		{TokenHash: foreignPending, UserID: other.ID, PendingEmail: new("elsewhere@example.com"), ExpiresAt: time.Now().Add(time.Hour)},
	} {
		require.NoError(t, repos.EmailVerification.Create(t.Context(), token))
	}
	_, err := repos.EmailVerification.Consume(t.Context(), usedPending)
	require.NoError(t, err)

	// when
	err = repos.EmailVerification.DeletePendingEmailChangesForUser(t.Context(), user.ID)

	// then only the user's unused change of address is gone
	require.NoError(t, err)
	for tokenHash, wantKept := range map[string]bool{pending: false, usedPending: true, registration: true, foreignPending: true} {
		var kept bool
		require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT EXISTS (SELECT 1 FROM email_verification_tokens WHERE token_hash = $1)`, tokenHash).Scan(&kept))
		assert.Equal(t, wantKept, kept, tokenHash)
	}
}
