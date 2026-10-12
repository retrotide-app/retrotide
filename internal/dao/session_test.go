package dao_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionDAO_CreateAndGet(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	token := uuid.NewString()
	expiresAt := time.Now().Add(time.Hour)

	// when
	err := repos.Session.Create(t.Context(), spec.NewSession{Token: token, UserID: user.ID, ExpiresAt: expiresAt})

	// then
	require.NoError(t, err)
	got, err := repos.Session.Get(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.UserID)
	assert.WithinDuration(t, expiresAt, got.ExpiresAt, time.Millisecond)
	assert.WithinDuration(t, time.Now(), got.CreatedAt, time.Minute)
}

func TestSessionDAO_StoresOnlyTheTokenHash(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	token := uuid.NewString()
	expected := sha256.Sum256([]byte(token))

	// when
	require.NoError(t, repos.Session.Create(t.Context(), spec.NewSession{Token: token, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// then
	var stored string
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT token FROM sessions WHERE user_id = $1`, user.ID).Scan(&stored))
	assert.NotEqual(t, token, stored)
	assert.Equal(t, hex.EncodeToString(expected[:]), stored)
}

func TestSessionDAO_Get_UnknownTokenIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	_, err := repos.Session.Get(t.Context(), "never-issued")

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestSessionDAO_Get_ExpiredSessionIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	token := uuid.NewString()
	require.NoError(t, repos.Session.Create(t.Context(), spec.NewSession{Token: token, UserID: user.ID, ExpiresAt: time.Now().Add(-time.Minute)}))

	// when
	_, err := repos.Session.Get(t.Context(), token)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestSessionDAO_Create_ForAnUnknownUserFails(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Session.Create(t.Context(), spec.NewSession{Token: uuid.NewString(), UserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)})

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sessions_user_id_fkey")
}

func TestSessionDAO_Delete(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	token := daotest.CreateSession(t, repos, user.ID)
	kept := daotest.CreateSession(t, repos, user.ID)

	// when
	err := repos.Session.Delete(t.Context(), token)

	// then
	require.NoError(t, err)
	_, err = repos.Session.Get(t.Context(), token)
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, err = repos.Session.Get(t.Context(), kept)
	require.NoError(t, err)
}

func TestSessionDAO_Delete_UnknownTokenIsNotAnError(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Session.Delete(t.Context(), "already-signed-out")

	// then
	require.NoError(t, err)
}

func TestSessionDAO_DeleteAllForUser_LeavesOtherUsers(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	first := daotest.CreateSession(t, repos, user.ID)
	second := daotest.CreateSession(t, repos, user.ID)
	foreign := daotest.CreateSession(t, repos, other.ID)

	// when
	err := repos.Session.DeleteAllForUser(t.Context(), user.ID)

	// then
	require.NoError(t, err)
	_, firstErr := repos.Session.Get(t.Context(), first)
	_, secondErr := repos.Session.Get(t.Context(), second)
	_, foreignErr := repos.Session.Get(t.Context(), foreign)
	require.ErrorIs(t, firstErr, dao.ErrNotFound)
	require.ErrorIs(t, secondErr, dao.ErrNotFound)
	require.NoError(t, foreignErr)
}

func TestSessionDAO_DeleteAllForUserExcept_KeepsThePresentedSession(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	current := daotest.CreateSession(t, repos, user.ID)
	stale := daotest.CreateSession(t, repos, user.ID)
	foreign := daotest.CreateSession(t, repos, other.ID)

	// when
	err := repos.Session.DeleteAllForUserExcept(t.Context(), spec.SessionDeletionExcept{UserID: user.ID, KeepToken: current})

	// then
	require.NoError(t, err)
	_, currentErr := repos.Session.Get(t.Context(), current)
	_, staleErr := repos.Session.Get(t.Context(), stale)
	_, foreignErr := repos.Session.Get(t.Context(), foreign)
	require.NoError(t, currentErr)
	require.ErrorIs(t, staleErr, dao.ErrNotFound)
	require.NoError(t, foreignErr)
}

func TestSessionDAO_CleanExpired_RemovesOnlyExpiredSessions(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	fresh := uuid.NewString()
	require.NoError(t, repos.Session.Create(t.Context(), spec.NewSession{Token: fresh, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))
	for range 2 {
		require.NoError(t, repos.Session.Create(t.Context(), spec.NewSession{Token: uuid.NewString(), UserID: user.ID, ExpiresAt: time.Now().Add(-time.Hour)}))
	}

	// when
	removed, err := repos.Session.CleanExpired(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)
	_, err = repos.Session.Get(t.Context(), fresh)
	require.NoError(t, err)
}

func TestSessionDAO_CleanExpired_NothingToRemove(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	daotest.CreateSession(t, repos, user.ID)

	// when
	removed, err := repos.Session.CleanExpired(t.Context())

	// then
	require.NoError(t, err)
	assert.Zero(t, removed)
}
