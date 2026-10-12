package guest

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testDeviceDays = 90
	testToken      = "4e6a8c0e2b4d6f8a1c3e5b7d9f2a4c6e8b1d3f5a7c9e2b4d6f8a1c3e5b7d9f2a"
)

var (
	errStoreDown = errors.New("guest store down")
)

func newTestManager(t *testing.T) (*Manager, *repository.MockGuestDeviceRepository, *settings.MockService) {
	t.Helper()

	repo := repository.NewMockGuestDeviceRepository(t)
	settingsSvc := settings.NewMockService(t)

	return NewManager(repo, settingsSvc), repo, settingsSvc
}

func TestCreate_StoresADeviceThatExpiresAfterTheConfiguredDays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		mgr, repo, settingsSvc := newTestManager(t)
		settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingGuestDeviceDays).Return(testDeviceDays, nil)
		deviceID := uuid.New()
		var stored spec.NewGuestDevice
		repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewGuestDevice, _ ...*sql.Tx) (*model.GuestDevice, error) {
			stored = s

			return &model.GuestDevice{ID: deviceID}, nil
		})

		// when
		issued, err := mgr.Create(t.Context())

		// then the browser gets the token, and the store is handed it to hash
		require.NoError(t, err)
		assert.Equal(t, deviceID, issued.DeviceID)
		raw, decodeErr := hex.DecodeString(issued.Token)
		require.NoError(t, decodeErr)
		assert.Len(t, raw, 32)
		assert.Equal(t, issued.Token, stored.Token)
		assert.Equal(t, time.Now().Add(testDeviceDays*24*time.Hour), stored.ExpiresAt)
	})
}

func TestCreate_ReportsALifetimeItCannotRead(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingGuestDeviceDays).Return(0, errStoreDown)

	// when
	_, err := mgr.Create(t.Context())

	// then nothing is written, which the strict repository mock proves
	require.ErrorIs(t, err, errStoreDown)
}

func TestValidate_AnswersTheDeviceWithoutTouchingItWithinTheHour(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		mgr, repo, _ := newTestManager(t)
		deviceID := uuid.New()
		repo.EXPECT().Get(mock.Anything, testToken).Return(&model.GuestDevice{ID: deviceID, LastSeenAt: time.Now().Add(-59 * time.Minute)}, nil)

		// when
		got, err := mgr.Validate(t.Context(), testToken)

		// then
		require.NoError(t, err)
		assert.Equal(t, deviceID, got)
	})
}

func TestValidate_TouchesADeviceUnseenForAnHour(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		mgr, repo, _ := newTestManager(t)
		deviceID := uuid.New()
		repo.EXPECT().Get(mock.Anything, testToken).Return(&model.GuestDevice{ID: deviceID, LastSeenAt: time.Now().Add(-time.Hour)}, nil)
		repo.EXPECT().Touch(mock.Anything, deviceID).Return(nil)

		// when
		got, err := mgr.Validate(t.Context(), testToken)

		// then
		require.NoError(t, err)
		assert.Equal(t, deviceID, got)
	})
}

func TestValidate_ReportsAFailedTouch(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	deviceID := uuid.New()
	repo.EXPECT().Get(mock.Anything, testToken).Return(&model.GuestDevice{ID: deviceID}, nil)
	repo.EXPECT().Touch(mock.Anything, deviceID).Return(errStoreDown)

	// when
	_, err := mgr.Validate(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errStoreDown)
	assert.NotErrorIs(t, err, ErrInvalidGuest)
}

func TestValidate_TellsAGoneDeviceFromAStoreThatCannotAnswer(t *testing.T) {
	tests := []struct {
		name        string
		stored      error
		wantInvalid bool
	}{
		{name: "an unknown or expired device", stored: fmt.Errorf("get guest device: %w", dao.ErrNotFound), wantInvalid: true},
		{name: "a store that cannot answer", stored: errStoreDown, wantInvalid: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mgr, repo, _ := newTestManager(t)
			repo.EXPECT().Get(mock.Anything, testToken).Return(nil, tc.stored)

			// when
			_, err := mgr.Validate(t.Context(), testToken)
			_, principalErr := mgr.ValidatePrincipal(t.Context(), testToken)

			// then only a gone device reads as invalid, so a blip is never a silent downgrade
			require.Error(t, err)
			assert.Equal(t, tc.wantInvalid, errors.Is(err, ErrInvalidGuest))
			assert.Equal(t, tc.wantInvalid, errors.Is(principalErr, model.ErrInvalidPrincipal))
		})
	}
}

func TestValidatePrincipal_AnswersAGuestPrincipal(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	deviceID := uuid.New()
	repo.EXPECT().Get(mock.Anything, testToken).Return(&model.GuestDevice{ID: deviceID, LastSeenAt: time.Now()}, nil)

	// when
	principal, err := mgr.ValidatePrincipal(t.Context(), testToken)

	// then
	require.NoError(t, err)
	assert.Equal(t, model.Principal{Kind: model.PrincipalGuest, ID: deviceID}, principal)
}

func TestRetire_DeletesTheDeviceThenDisconnectsItsSockets(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	disconnector := NewMockDisconnector(t)
	mgr.SetDisconnector(disconnector)
	deviceID := uuid.New()
	deleted := false
	repo.EXPECT().DeleteByID(mock.Anything, deviceID).RunAndReturn(func(_ context.Context, _ uuid.UUID, _ ...*sql.Tx) error {
		deleted = true

		return nil
	})
	disconnector.EXPECT().DisconnectUser(deviceID).RunAndReturn(func(uuid.UUID) int {
		assert.True(t, deleted, "the sockets drop only once the device can no longer revalidate")

		return 2
	})

	// when
	err := mgr.Retire(t.Context(), deviceID)

	// then
	require.NoError(t, err)
}

func TestRetire_LeavesTheSocketsWhenTheDeleteFails(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	mgr.SetDisconnector(NewMockDisconnector(t))
	deviceID := uuid.New()
	repo.EXPECT().DeleteByID(mock.Anything, deviceID).Return(errStoreDown)

	// when
	err := mgr.Retire(t.Context(), deviceID)

	// then the strict disconnector proves no socket was dropped
	require.ErrorIs(t, err, errStoreDown)
}

func TestCookie_UsesTheHostPrefixAndTheDeviceLifetimeUnderHTTPS(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("https://retrotide.test", nil)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingGuestDeviceDays).Return(testDeviceDays, nil)

	// when
	cookie, err := mgr.Cookie(t.Context(), testToken)

	// then
	require.NoError(t, err)
	assert.Equal(t, "__Host-rt_guest", cookie.Name)
	assert.Equal(t, testToken, cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, "/", cookie.Path)
	assert.Empty(t, cookie.Domain)
	assert.Equal(t, testDeviceDays*24*60*60, cookie.MaxAge)
}

func TestToken_ReadsOnlyTheBareNameOverHTTP(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("http://localhost:4323", nil)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/b/x", nil)
	req.AddCookie(&http.Cookie{Name: "rt_guest", Value: testToken})

	// when
	token, err := mgr.Token(req)

	// then
	require.NoError(t, err)
	assert.Equal(t, testToken, token)
}
