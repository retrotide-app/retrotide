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

func TestGuestDevice_CreateAndGet(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	token := uuid.NewString()
	expiresAt := time.Now().Add(90 * 24 * time.Hour)

	// when
	created, err := repos.GuestDevice.Create(t.Context(), spec.NewGuestDevice{Token: token, ExpiresAt: expiresAt})

	// then
	require.NoError(t, err)
	got, err := repos.GuestDevice.Get(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.WithinDuration(t, expiresAt, got.ExpiresAt, time.Millisecond)
	assert.WithinDuration(t, time.Now(), got.LastSeenAt, time.Minute)
}

func TestGuestDevice_StoresOnlyTheTokenHash(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	token := uuid.NewString()
	expected := sha256.Sum256([]byte(token))

	// when
	created, err := repos.GuestDevice.Create(t.Context(), spec.NewGuestDevice{Token: token, ExpiresAt: time.Now().Add(time.Hour)})
	require.NoError(t, err)

	// then a database leak hands over no usable guest identity
	var stored string
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT token_hash FROM guest_devices WHERE id = $1`, created.ID).Scan(&stored))
	assert.NotEqual(t, token, stored)
	assert.Equal(t, hex.EncodeToString(expected[:]), stored)
}

func TestGuestDevice_AnUnknownOrExpiredDeviceIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	expired := uuid.NewString()
	_, err := repos.GuestDevice.Create(t.Context(), spec.NewGuestDevice{Token: expired, ExpiresAt: time.Now().Add(-time.Minute)})
	require.NoError(t, err)

	// when
	_, unknownErr := repos.GuestDevice.Get(t.Context(), "never-issued")
	_, expiredErr := repos.GuestDevice.Get(t.Context(), expired)

	// then
	require.ErrorIs(t, unknownErr, dao.ErrNotFound)
	require.ErrorIs(t, expiredErr, dao.ErrNotFound)
}

func TestGuestDevice_TouchMovesLastSeen(t *testing.T) {
	// given a device last seen two hours ago
	repos := daotest.NewRepos(t)
	device, token := daotest.CreateGuest(t, repos)
	_, err := repos.DB().ExecContext(t.Context(), `UPDATE guest_devices SET last_seen_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, device.ID)
	require.NoError(t, err)

	// when
	require.NoError(t, repos.GuestDevice.Touch(t.Context(), device.ID))

	// then
	got, err := repos.GuestDevice.Get(t.Context(), token)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), got.LastSeenAt, time.Minute)
}

func TestGuestDevice_DeleteForgetsTheToken(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	_, byToken := daotest.CreateGuest(t, repos)
	byID, byIDToken := daotest.CreateGuest(t, repos)

	// when
	require.NoError(t, repos.GuestDevice.Delete(t.Context(), byToken))
	require.NoError(t, repos.GuestDevice.DeleteByID(t.Context(), byID.ID))

	// then
	_, tokenErr := repos.GuestDevice.Get(t.Context(), byToken)
	_, idErr := repos.GuestDevice.Get(t.Context(), byIDToken)
	require.ErrorIs(t, tokenErr, dao.ErrNotFound)
	require.ErrorIs(t, idErr, dao.ErrNotFound)
}

func TestGuestDevice_DeletingADeviceKeepsItsSeatsButSeversThem(t *testing.T) {
	// given a guest seated on a board
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	board := daotest.CreateBoard(t, repos, workspace.Team, owner.ID)
	device, _ := daotest.CreateGuest(t, repos)
	seat := daotest.CreateGuestParticipant(t, repos, board.Header, device.ID)

	// when
	require.NoError(t, repos.GuestDevice.DeleteByID(t.Context(), device.ID))

	// then the cards stay on the board, and nothing links them to anyone
	participants, err := repos.Board.ListParticipants(t.Context(), board.Header.ID)
	require.NoError(t, err)
	var kept bool
	for _, participant := range participants {
		if participant.ID == seat.ID {
			kept = true
			assert.Nil(t, participant.GuestDeviceID)
			assert.Nil(t, participant.UserID)
		}
	}
	assert.True(t, kept)
}

func TestGuestDevice_CleanExpiredSweepsExpiredAndLongUnseenDevices(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	fresh, freshToken := daotest.CreateGuest(t, repos)
	_, err := repos.GuestDevice.Create(t.Context(), spec.NewGuestDevice{Token: uuid.NewString(), ExpiresAt: time.Now().Add(-time.Minute)})
	require.NoError(t, err)
	unseen, _ := daotest.CreateGuest(t, repos)
	_, err = repos.DB().ExecContext(t.Context(), `UPDATE guest_devices SET last_seen_at = NOW() - INTERVAL '91 days' WHERE id = $1`, unseen.ID)
	require.NoError(t, err)

	// when
	removed, err := repos.GuestDevice.CleanExpired(t.Context(), time.Now().Add(-90*24*time.Hour))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)
	kept, err := repos.GuestDevice.Get(t.Context(), freshToken)
	require.NoError(t, err)
	assert.Equal(t, fresh.ID, kept.ID)
}
