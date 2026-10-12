package session

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
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
	testDurationDays = 30
	testToken        = "3f1c9a7e2b4d6f8091a3c5e7f9b1d3e5a7c9e1f3b5d7f9a1c3e5b7d9f1a3c5e7"
)

func newTestManager(t *testing.T) (*Manager, *repository.MockSessionRepository, *settings.MockService) {
	t.Helper()

	repo := repository.NewMockSessionRepository(t)
	settingsSvc := settings.NewMockService(t)

	return NewManager(repo, settingsSvc), repo, settingsSvc
}

func TestIssue_DrawsA32ByteHexTokenThatExpiresAfterTheConfiguredDays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		mgr, _, settingsSvc := newTestManager(t)
		settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil)

		// when
		token, expiresAt, err := mgr.issue(t.Context(), true)

		// then
		require.NoError(t, err)
		raw, decodeErr := hex.DecodeString(token)
		require.NoError(t, decodeErr)
		assert.Len(t, raw, 32)
		assert.Equal(t, time.Now().Add(testDurationDays*24*time.Hour), expiresAt)
	})
}

func TestIssue_ExpiresAfterADayWithoutReadingTheSettingWhenNotRemembered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		mgr, _, _ := newTestManager(t)

		// when
		token, expiresAt, err := mgr.issue(t.Context(), false)

		// then
		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.Equal(t, time.Now().Add(TransientLifetime), expiresAt)
	})
}

func TestIssue_DrawsAFreshTokenEachTime(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil).Twice()

	// when
	first, _, firstErr := mgr.issue(t.Context(), true)
	second, _, secondErr := mgr.issue(t.Context(), true)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.NotEqual(t, first, second)
}

func TestIssue_ReturnsTheSettingsError(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(0, errSettingsDown)

	// when
	token, _, err := mgr.issue(t.Context(), true)

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Empty(t, token)
}

func TestCreate_StoresTheIssuedTokenForTheUser(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		mgr, repo, settingsSvc := newTestManager(t)
		userID := uuid.New()
		settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil)

		var stored spec.NewSession
		repo.EXPECT().Create(mock.Anything, mock.AnythingOfType("spec.NewSession")).RunAndReturn(func(_ context.Context, s spec.NewSession, _ ...*sql.Tx) error {
			stored = s

			return nil
		})

		// when
		token, err := mgr.Create(t.Context(), spec.SessionGrant{UserID: userID, Remember: true})

		// then
		require.NoError(t, err)
		assert.Equal(t, spec.NewSession{Token: token, UserID: userID, ExpiresAt: time.Now().Add(testDurationDays * 24 * time.Hour)}, stored)
	})
}

func TestCreate_StoresAOneDaySessionWhenNotRemembered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		mgr, repo, _ := newTestManager(t)
		userID := uuid.New()

		var stored spec.NewSession
		repo.EXPECT().Create(mock.Anything, mock.AnythingOfType("spec.NewSession")).RunAndReturn(func(_ context.Context, s spec.NewSession, _ ...*sql.Tx) error {
			stored = s

			return nil
		})

		// when
		token, err := mgr.Create(t.Context(), spec.SessionGrant{UserID: userID})

		// then
		require.NoError(t, err)
		assert.Equal(t, spec.NewSession{Token: token, UserID: userID, ExpiresAt: time.Now().Add(TransientLifetime)}, stored)
	})
}

func TestCreate_ReturnsTheRepositoryError(t *testing.T) {
	// given
	mgr, repo, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testDurationDays, nil)
	repo.EXPECT().Create(mock.Anything, mock.AnythingOfType("spec.NewSession")).Return(errDatabaseDown)

	// when
	token, err := mgr.Create(t.Context(), spec.SessionGrant{UserID: uuid.New(), Remember: true})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Empty(t, token)
}

func TestCreate_WritesNothingWhenTheDurationIsUnreadable(t *testing.T) {
	// given
	mgr, _, settingsSvc := newTestManager(t)
	settingsSvc.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(0, errSettingsDown)

	// when
	token, err := mgr.Create(t.Context(), spec.SessionGrant{UserID: uuid.New(), Remember: true})

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.Empty(t, token)
}

func TestValidate_ReturnsTheSessionsUser(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	userID := uuid.New()
	repo.EXPECT().Get(mock.Anything, testToken).Return(&model.Session{UserID: userID}, nil)

	// when
	got, err := mgr.Validate(t.Context(), testToken)

	// then
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestValidate_ReportsAnUnknownOrExpiredSessionAsInvalid(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Get(mock.Anything, testToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))

	// when
	got, err := mgr.Validate(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, ErrInvalidSession)
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, uuid.Nil, got)
}

func TestValidate_NeverReportsADatabaseFailureAsInvalid(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Get(mock.Anything, testToken).Return(nil, errDatabaseDown)

	// when
	got, err := mgr.Validate(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrInvalidSession)
	assert.Equal(t, uuid.Nil, got)
}

func TestValidatePrincipal_ReturnsTheSessionsUserAsAUserPrincipal(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	userID := uuid.New()
	repo.EXPECT().Get(mock.Anything, testToken).Return(&model.Session{UserID: userID}, nil)

	// when
	got, err := mgr.ValidatePrincipal(t.Context(), testToken)

	// then
	require.NoError(t, err)
	assert.Equal(t, model.Principal{Kind: model.PrincipalUser, ID: userID}, got)
}

func TestValidatePrincipal_ReportsAnUnknownSessionAsAnInvalidPrincipal(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Get(mock.Anything, testToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))

	// when
	got, err := mgr.ValidatePrincipal(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, model.ErrInvalidPrincipal)
	require.ErrorIs(t, err, ErrInvalidSession)
	assert.Equal(t, model.Principal{}, got)
}

func TestValidatePrincipal_NeverReportsADatabaseFailureAsAnInvalidPrincipal(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Get(mock.Anything, testToken).Return(nil, errDatabaseDown)

	// when
	got, err := mgr.ValidatePrincipal(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, model.ErrInvalidPrincipal)
	assert.Equal(t, model.Principal{}, got)
}

func TestDelete_RevokesTheSession(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Delete(mock.Anything, testToken).Return(nil)

	// when
	err := mgr.Delete(t.Context(), testToken)

	// then
	require.NoError(t, err)
}

func TestDelete_RevokesNothingWithoutAToken(t *testing.T) {
	// given
	mgr, _, _ := newTestManager(t)

	// when
	err := mgr.Delete(t.Context(), "")

	// then
	require.NoError(t, err)
}

func TestDelete_ReturnsTheRepositoryError(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Delete(mock.Anything, testToken).Return(errDatabaseDown)

	// when
	err := mgr.Delete(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestDeleteAllForUser_RevokesEverySessionThenDisconnects(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	disconnector := NewMockDisconnector(t)
	mgr.SetDisconnector(disconnector)
	userID := uuid.New()
	repo.EXPECT().DeleteAllForUser(mock.Anything, userID).Return(nil)
	disconnector.EXPECT().DisconnectUser(userID).Return(2)

	// when
	err := mgr.DeleteAllForUser(t.Context(), userID)

	// then
	require.NoError(t, err)
}

func TestDeleteAllForUser_KeepsSocketsWhenTheRevocationFails(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	mgr.SetDisconnector(NewMockDisconnector(t))
	userID := uuid.New()
	repo.EXPECT().DeleteAllForUser(mock.Anything, userID).Return(errDatabaseDown)

	// when
	err := mgr.DeleteAllForUser(t.Context(), userID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestDeleteAllForUser_WorksBeforeADisconnectorIsSet(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	userID := uuid.New()
	repo.EXPECT().DeleteAllForUser(mock.Anything, userID).Return(nil)

	// when
	err := mgr.DeleteAllForUser(t.Context(), userID)

	// then
	require.NoError(t, err)
}

func TestDeleteAllForUserExcept_KeepsTheCurrentSessionThenDisconnects(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	disconnector := NewMockDisconnector(t)
	mgr.SetDisconnector(disconnector)
	deletion := spec.SessionDeletionExcept{UserID: uuid.New(), KeepToken: testToken}
	repo.EXPECT().DeleteAllForUserExcept(mock.Anything, deletion).Return(nil)
	disconnector.EXPECT().DisconnectUser(deletion.UserID).Return(1)

	// when
	err := mgr.DeleteAllForUserExcept(t.Context(), deletion)

	// then
	require.NoError(t, err)
}

func TestDeleteAllForUserExcept_RevokesEverySessionWithoutATokenToKeep(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	disconnector := NewMockDisconnector(t)
	mgr.SetDisconnector(disconnector)
	userID := uuid.New()
	repo.EXPECT().DeleteAllForUser(mock.Anything, userID).Return(nil)
	disconnector.EXPECT().DisconnectUser(userID).Return(0)

	// when
	err := mgr.DeleteAllForUserExcept(t.Context(), spec.SessionDeletionExcept{UserID: userID})

	// then
	require.NoError(t, err)
}

func TestDeleteAllForUserExcept_KeepsSocketsWhenTheRevocationFails(t *testing.T) {
	// given
	mgr, repo, _ := newTestManager(t)
	mgr.SetDisconnector(NewMockDisconnector(t))
	deletion := spec.SessionDeletionExcept{UserID: uuid.New(), KeepToken: testToken}
	repo.EXPECT().DeleteAllForUserExcept(mock.Anything, deletion).Return(errDatabaseDown)

	// when
	err := mgr.DeleteAllForUserExcept(t.Context(), deletion)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestDisconnect_ClosesTheUsersSockets(t *testing.T) {
	// given
	mgr, _, _ := newTestManager(t)
	disconnector := NewMockDisconnector(t)
	mgr.SetDisconnector(disconnector)
	userID := uuid.New()
	disconnector.EXPECT().DisconnectUser(userID).Return(3)

	// when
	mgr.Disconnect(userID)

	// then
	disconnector.AssertCalled(t, "DisconnectUser", userID)
}

func TestDisconnect_DoesNothingBeforeADisconnectorIsSet(t *testing.T) {
	// given
	mgr, _, _ := newTestManager(t)

	// when
	disconnect := func() {
		mgr.Disconnect(uuid.New())
	}

	// then
	assert.NotPanics(t, disconnect)
}
