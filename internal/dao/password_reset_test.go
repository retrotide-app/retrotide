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

func TestTokenPasswordReset_ConsumesOnce(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	tokenHash := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Create(t.Context(), spec.NewPasswordReset{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	consumed, err := repos.PasswordReset.Consume(t.Context(), tokenHash)
	_, again := repos.PasswordReset.Consume(t.Context(), tokenHash)

	// then
	require.NoError(t, err)
	assert.Equal(t, user.ID, consumed.UserID)
	require.NotNil(t, consumed.UsedAt)
	require.ErrorIs(t, again, dao.ErrNotFound)
}

func TestTokenPasswordReset_ExpiredTokenIsNotConsumed(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	tokenHash := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Create(t.Context(), spec.NewPasswordReset{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(-time.Minute)}))

	// when
	_, err := repos.PasswordReset.Consume(t.Context(), tokenHash)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTokenPasswordReset_UnknownTokenIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	_, err := repos.PasswordReset.Consume(t.Context(), "never-issued")

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTokenPasswordReset_IssueReplacesEveryUnusedToken(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	earlier := uuid.NewString()
	later := uuid.NewString()
	foreign := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: earlier, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: foreign, UserID: other.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	err := repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: later, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)})

	// then only the latest link for that account works, and other accounts keep theirs
	require.NoError(t, err)
	_, earlierErr := repos.PasswordReset.Consume(t.Context(), earlier)
	require.ErrorIs(t, earlierErr, dao.ErrNotFound)
	_, laterErr := repos.PasswordReset.Consume(t.Context(), later)
	require.NoError(t, laterErr)
	_, foreignErr := repos.PasswordReset.Consume(t.Context(), foreign)
	require.NoError(t, foreignErr)
}
