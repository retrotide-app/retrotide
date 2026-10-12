package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*service, *repository.MockSettingsRepository) {
	t.Helper()

	repo := repository.NewMockSettingsRepository(t)
	svc := NewService(repo).(*service)

	return svc, repo
}

func batchOfKeys(want ...config.SiteSettingKey) any {
	return mock.MatchedBy(func(got []config.SiteSettingKey) bool {
		if len(got) != len(want) {
			return false
		}

		for _, key := range want {
			if !slices.Contains(got, key) {
				return false
			}
		}

		return true
	})
}

func primeValidCache(repo *repository.MockSettingsRepository) {
	m := validBaseSettings()
	m[config.SettingMaxBodySize.Key] = "104857600"
	m[config.SettingCacheInMemoryMaxMB.Key] = "256"

	repo.EXPECT().GetAll(mock.Anything).Return(m, nil).Maybe()
}

func validBaseSettings() map[config.SiteSettingKey]string {
	out := make(map[config.SiteSettingKey]string)
	for _, def := range config.AllSiteSettings {
		out[def.Key] = def.Default
	}

	return out
}

func TestGet_ReturnsTheRepositoryError(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	readErr := errors.New("db down")
	repo.EXPECT().Get(mock.Anything, config.SettingSiteName.Key).Return("", readErr)

	// when
	_, err := svc.Get(t.Context(), config.SettingSiteName)

	// then the caller decides what a failed read means, rather than being handed the default
	require.ErrorIs(t, err, readErr)
	assert.ErrorContains(t, err, string(config.SettingSiteName.Key))
}

func TestGet_ReturnsCachedValue(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, config.SettingSiteName.Key).Return("Cached Name", nil)

	// when
	got, err := svc.Get(t.Context(), config.SettingSiteName)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Cached Name", got)
}

func TestGetInt_ParsesCachedValue(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, config.SettingMaxBodySize.Key).Return("4096", nil)

	// when
	got, err := svc.GetInt(t.Context(), config.SettingMaxBodySize)

	// then
	require.NoError(t, err)
	assert.Equal(t, 4096, got)
}

func TestGetInt_RefusesAValueThatIsNotAWholeNumber(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, config.SettingMaxBodySize.Key).Return("not-a-number", nil)

	// when
	_, err := svc.GetInt(t.Context(), config.SettingMaxBodySize)

	// then
	require.ErrorIs(t, err, strconv.ErrSyntax)
	assert.ErrorContains(t, err, "max_body_size is not a whole number")
}

func TestGetInt_ReturnsTheRepositoryError(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	readErr := errors.New("db down")
	repo.EXPECT().Get(mock.Anything, config.SettingMaxBodySize.Key).Return("", readErr)

	// when
	_, err := svc.GetInt(t.Context(), config.SettingMaxBodySize)

	// then
	require.ErrorIs(t, err, readErr)
}

func TestGetBool_TrueWhenValueIsTrue(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, config.SettingMaintenanceMode.Key).Return("true", nil)

	// when
	got, err := svc.GetBool(t.Context(), config.SettingMaintenanceMode)

	// then
	require.NoError(t, err)
	assert.True(t, got)
}

func TestGetBool_ReturnsTheRepositoryError(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	readErr := errors.New("db down")
	repo.EXPECT().Get(mock.Anything, config.SettingMaintenanceMode.Key).Return("", readErr)

	// when
	_, err := svc.GetBool(t.Context(), config.SettingMaintenanceMode)

	// then a gate reading this can refuse the request instead of treating the switch as off
	require.ErrorIs(t, err, readErr)
}

func TestGetBool_FalseForOtherValues(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"literal false", "false"},
		{"empty string", ""},
		{"garbage", "yes"},
		{"capitalised true", "True"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, repo := newTestService(t)
			repo.EXPECT().Get(mock.Anything, config.SettingMaintenanceMode.Key).Return(tc.value, nil)

			// when
			got, err := svc.GetBool(t.Context(), config.SettingMaintenanceMode)

			// then
			require.NoError(t, err)
			assert.False(t, got)
		})
	}
}

func TestGetAll_ReturnsDefaultsWhenCacheEmpty(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().GetAll(mock.Anything).Return(map[config.SiteSettingKey]string{}, nil)

	// when
	got, err := svc.GetAll(t.Context())

	// then
	require.NoError(t, err)
	assert.Len(t, got, len(config.AllSiteSettings))
	for _, def := range config.AllSiteSettings {
		assert.Equal(t, def.Default, got[def.Key])
	}
}

func TestGetAll_OverlaysCachedValues(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().GetAll(mock.Anything).Return(map[config.SiteSettingKey]string{
		config.SettingSiteName.Key:        "Overlay",
		config.SettingMaintenanceMode.Key: "true",
	}, nil)

	// when
	got, err := svc.GetAll(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, "Overlay", got[config.SettingSiteName.Key])
	assert.Equal(t, "true", got[config.SettingMaintenanceMode.Key])
	assert.Equal(t, config.SettingBaseURL.Default, got[config.SettingBaseURL.Key])
}

func TestGetAll_LeavesOutStoredSettingsItDoesNotKnow(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().GetAll(mock.Anything).Return(map[config.SiteSettingKey]string{
		config.SettingSiteName.Key:     "Harbour",
		"setting_from_a_newer_release": "tuned by an admin",
	}, nil)

	// when
	got, err := svc.GetAll(t.Context())

	// then
	require.NoError(t, err)
	assert.Len(t, got, len(config.AllSiteSettings))
	assert.Equal(t, "Harbour", got[config.SettingSiteName.Key])
	assert.NotContains(t, got, config.SiteSettingKey("setting_from_a_newer_release"))
}

func TestGetAll_ReturnsTheRepositoryError(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	readErr := errors.New("db down")
	repo.EXPECT().GetAll(mock.Anything).Return(nil, readErr)

	// when
	got, err := svc.GetAll(t.Context())

	// then
	require.ErrorIs(t, err, readErr)
	assert.Nil(t, got)
}

func TestRefresh_RepoGetAllError(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().GetAll(mock.Anything).Return(nil, errors.New("db down"))

	// when
	err := svc.Refresh(t.Context())

	// then
	require.Error(t, err)
	assert.EqualError(t, err, "db down")
}

func TestRefresh_SeedsMissingDefaults(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	existing := map[config.SiteSettingKey]string{}
	for _, def := range config.AllSiteSettings {
		existing[def.Key] = def.Default
	}
	delete(existing, config.SettingSiteName.Key)
	delete(existing, config.SettingBaseURL.Key)

	repo.EXPECT().GetAll(mock.Anything).Return(existing, nil)
	repo.EXPECT().Reconcile(mock.Anything, mock.MatchedBy(func(s spec.SettingsReconcile) bool {
		if len(s.Missing) != 2 || s.UpdatedBy != uuid.Nil {
			return false
		}

		_, okName := s.Missing[config.SettingSiteName.Key]
		_, okURL := s.Missing[config.SettingBaseURL.Key]

		return okName && okURL
	})).Return(nil)

	// when
	err := svc.Refresh(t.Context())

	// then
	require.NoError(t, err)
}

func TestRefresh_SeedErrorBubbles(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	repo.EXPECT().GetAll(mock.Anything).Return(map[config.SiteSettingKey]string{}, nil)
	repo.EXPECT().Reconcile(mock.Anything, mock.Anything).Return(errors.New("seed failed"))

	// when
	err := svc.Refresh(t.Context())

	// then
	require.Error(t, err)
	assert.EqualError(t, err, "seed failed")
}

func TestRefresh_KeepsSettingsANewerReleaseAdded(t *testing.T) {
	// given the database of a newer release, which the previous binary is rolled back onto
	svc, repo := newTestService(t)
	existing := validBaseSettings()
	existing["setting_from_a_newer_release"] = "tuned by an admin"

	repo.EXPECT().GetAll(mock.Anything).Return(existing, nil)

	// when
	err := svc.Refresh(t.Context())

	// then the unknown row is left alone, so rolling forward again finds the admin's value
	require.NoError(t, err)
	repo.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestRefresh_PopulatesCacheFromRepo(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	existing := validBaseSettings()
	existing[config.SettingSiteName.Key] = "Loaded Site"
	existing[config.SettingMaintenanceMode.Key] = "true"

	repo.EXPECT().GetAll(mock.Anything).Return(existing, nil)
	repo.EXPECT().Get(mock.Anything, config.SettingMaintenanceMode.Key).Return("true", nil)

	// when
	err := svc.Refresh(t.Context())

	// then
	require.NoError(t, err)

	maintenance, err := svc.GetBool(t.Context(), config.SettingMaintenanceMode)
	require.NoError(t, err)
	assert.True(t, maintenance)
}

func TestSet_HappyPathUpdatesCacheAndNotifies(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	listener := NewMockListener(t)
	listener.EXPECT().OnSettingChanged(config.SettingSiteName.Key, "New Name").Once()
	svc.Subscribe(listener)
	updatedBy := uuid.New()

	repo.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: config.SettingSiteName.Key, Value: "New Name", UpdatedBy: updatedBy}).Return(nil)

	// when
	err := svc.Set(t.Context(), config.SettingSiteName, "New Name", updatedBy)

	// then
	require.NoError(t, err)
	listener.AssertExpectations(t)
}

func TestSet_ValidationFailureSkipsRepo(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	updatedBy := uuid.New()

	// when
	err := svc.Set(t.Context(), config.SettingMaxBodySize, "abc", updatedBy)

	// then
	require.ErrorIs(t, err, ErrInvalidValue)
	assert.Contains(t, err.Error(), "max_body_size must be a whole number")
}

func TestSet_RepoErrorBubblesAndSkipsCache(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	listener := NewMockListener(t)
	svc.Subscribe(listener)
	updatedBy := uuid.New()

	repo.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: config.SettingSiteName.Key, Value: "Attempt", UpdatedBy: updatedBy}).Return(errors.New("db down"))

	// when
	err := svc.Set(t.Context(), config.SettingSiteName, "Attempt", updatedBy)

	// then
	require.Error(t, err)
	assert.EqualError(t, err, "db down")
	listener.AssertNotCalled(t, "OnSettingChanged", mock.Anything, mock.Anything)
}

func TestSet_ReportsAStaleCacheButStillAppliesTheChange(t *testing.T) {
	// given a write that commits but cannot drop its cached copy
	svc, repo := newTestService(t)
	primeValidCache(repo)
	listener := NewMockListener(t)
	listener.EXPECT().OnSettingChanged(config.SettingSiteName.Key, "New Name").Once()
	svc.Subscribe(listener)
	updatedBy := uuid.New()
	staleErr := fmt.Errorf("%w: %w", repository.ErrStaleSettingsCache, errors.New("valkey unreachable"))

	repo.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: config.SettingSiteName.Key, Value: "New Name", UpdatedBy: updatedBy}).Return(staleErr)

	// when
	err := svc.Set(t.Context(), config.SettingSiteName, "New Name", updatedBy)

	// then the listeners still apply the committed value, and the admin is told the cache may lag
	require.ErrorIs(t, err, repository.ErrStaleSettingsCache)
	listener.AssertExpectations(t)
}

func TestSetMultiple_ReportsAStaleCacheButStillAppliesTheChange(t *testing.T) {
	// given a write that commits but cannot drop its cached copies
	svc, repo := newTestService(t)
	primeValidCache(repo)
	plain := NewMockListener(t)
	plain.EXPECT().OnSettingChanged(config.SettingSiteName.Key, "X").Once()
	batch := NewMockBatchListener(t)
	batch.EXPECT().OnSettingsBatchChanged(batchOfKeys(config.SettingSiteName.Key)).Once()
	svc.Subscribe(plain)
	svc.SubscribeBatch(batch)
	updatedBy := uuid.New()
	values := map[config.SiteSettingKey]string{
		config.SettingSiteName.Key: "X",
	}
	staleErr := fmt.Errorf("%w: %w", repository.ErrStaleSettingsCache, errors.New("valkey unreachable"))

	repo.EXPECT().SetMultiple(mock.Anything, spec.SettingsBulkUpdate{Values: values, UpdatedBy: updatedBy}).Return(staleErr)

	// when
	err := svc.SetMultiple(t.Context(), values, updatedBy)

	// then
	require.ErrorIs(t, err, repository.ErrStaleSettingsCache)
	plain.AssertExpectations(t)
	batch.AssertExpectations(t)
}

func TestSet_ReturnsTheRepositoryReadError(t *testing.T) {
	// given a repository that cannot read the stored settings
	svc, repo := newTestService(t)
	listener := NewMockListener(t)
	svc.Subscribe(listener)
	readErr := errors.New("db down")
	repo.EXPECT().GetAll(mock.Anything).Return(nil, readErr)

	// when
	err := svc.Set(t.Context(), config.SettingSiteName, "New Name", uuid.New())

	// then the write is refused rather than validated against the defaults
	require.ErrorIs(t, err, readErr)
	listener.AssertNotCalled(t, "OnSettingChanged", mock.Anything, mock.Anything)
}

func TestSetMultiple_HappyPathNotifiesEachAndBatch(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	plain := NewMockListener(t)
	plain.EXPECT().OnSettingChanged(config.SettingSiteName.Key, "Multi Name").Once()
	plain.EXPECT().OnSettingChanged(config.SettingMaintenanceMode.Key, "true").Once()

	listener := NewMockBatchListener(t)
	listener.EXPECT().OnSettingsBatchChanged(batchOfKeys(config.SettingSiteName.Key, config.SettingMaintenanceMode.Key)).Once()

	svc.Subscribe(plain)
	svc.SubscribeBatch(listener)
	updatedBy := uuid.New()

	values := map[config.SiteSettingKey]string{
		config.SettingSiteName.Key:        "Multi Name",
		config.SettingMaintenanceMode.Key: "true",
	}

	repo.EXPECT().SetMultiple(mock.Anything, spec.SettingsBulkUpdate{Values: values, UpdatedBy: updatedBy}).Return(nil)

	// when
	err := svc.SetMultiple(t.Context(), values, updatedBy)

	// then
	require.NoError(t, err)
	plain.AssertExpectations(t)
	listener.AssertExpectations(t)
	listener.AssertNumberOfCalls(t, "OnSettingsBatchChanged", 1)
}

func TestSetMultiple_UnknownKeyRejected(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	updatedBy := uuid.New()
	values := map[config.SiteSettingKey]string{
		config.SiteSettingKey("not_a_real_key"): "v",
	}

	// when
	err := svc.SetMultiple(t.Context(), values, updatedBy)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown setting")
}

func TestSetMultiple_ValidationFailureSkipsRepo(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	updatedBy := uuid.New()
	values := map[config.SiteSettingKey]string{
		config.SettingMaxBodySize.Key: "abc",
	}

	// when
	err := svc.SetMultiple(t.Context(), values, updatedBy)

	// then
	require.ErrorIs(t, err, ErrInvalidValue)
	assert.Contains(t, err.Error(), "max_body_size must be a whole number")
}

func TestSetMultiple_RepoErrorBubbles(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	plain := NewMockListener(t)
	listener := NewMockBatchListener(t)
	svc.Subscribe(plain)
	svc.SubscribeBatch(listener)
	updatedBy := uuid.New()
	values := map[config.SiteSettingKey]string{
		config.SettingSiteName.Key: "X",
	}

	repo.EXPECT().SetMultiple(mock.Anything, spec.SettingsBulkUpdate{Values: values, UpdatedBy: updatedBy}).Return(errors.New("boom"))

	// when
	err := svc.SetMultiple(t.Context(), values, updatedBy)

	// then
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
	plain.AssertNotCalled(t, "OnSettingChanged", mock.Anything, mock.Anything)
	listener.AssertNotCalled(t, "OnSettingsBatchChanged", mock.Anything)
}

func TestSetMultiple_ReturnsTheRepositoryReadError(t *testing.T) {
	// given a repository that cannot read the stored settings
	svc, repo := newTestService(t)
	batch := NewMockBatchListener(t)
	svc.SubscribeBatch(batch)
	readErr := errors.New("db down")
	repo.EXPECT().GetAll(mock.Anything).Return(nil, readErr)
	values := map[config.SiteSettingKey]string{
		config.SettingSiteName.Key: "X",
	}

	// when
	err := svc.SetMultiple(t.Context(), values, uuid.New())

	// then the write is refused rather than validated against the defaults
	require.ErrorIs(t, err, readErr)
	batch.AssertNotCalled(t, "OnSettingsBatchChanged", mock.Anything)
}

func TestSubscribe_MultipleListenersAllNotified(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	l1 := NewMockListener(t)
	l2 := NewMockListener(t)
	l1.EXPECT().OnSettingChanged(config.SettingSiteName.Key, "Hello").Once()
	l2.EXPECT().OnSettingChanged(config.SettingSiteName.Key, "Hello").Once()
	svc.Subscribe(l1)
	svc.Subscribe(l2)
	updatedBy := uuid.New()

	repo.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: config.SettingSiteName.Key, Value: "Hello", UpdatedBy: updatedBy}).Return(nil)

	// when
	err := svc.Set(t.Context(), config.SettingSiteName, "Hello", updatedBy)

	// then
	require.NoError(t, err)
	l1.AssertExpectations(t)
	l2.AssertExpectations(t)
}

func TestSubscribe_NonBatchListenerDoesNotReceiveBatch(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	plain := NewMockListener(t)
	plain.EXPECT().OnSettingChanged(config.SettingSiteName.Key, "Batched").Once()

	batch := NewMockBatchListener(t)
	batch.EXPECT().OnSettingsBatchChanged(batchOfKeys(config.SettingSiteName.Key)).Once()

	svc.Subscribe(plain)
	svc.SubscribeBatch(batch)
	updatedBy := uuid.New()

	values := map[config.SiteSettingKey]string{
		config.SettingSiteName.Key: "Batched",
	}

	repo.EXPECT().SetMultiple(mock.Anything, spec.SettingsBulkUpdate{Values: values, UpdatedBy: updatedBy}).Return(nil)

	// when
	err := svc.SetMultiple(t.Context(), values, updatedBy)

	// then
	require.NoError(t, err)
	plain.AssertExpectations(t)
	plain.AssertNumberOfCalls(t, "OnSettingChanged", 1)
	batch.AssertExpectations(t)
	batch.AssertNumberOfCalls(t, "OnSettingsBatchChanged", 1)
}

func TestSet_RejectsValueThatDoesNotMatchSettingType(t *testing.T) {
	tests := []struct {
		name    string
		setting *config.SiteSettingDef
		value   string
		wantErr string
	}{
		{
			name:    "text in a numeric setting",
			setting: config.SettingCacheInMemoryMaxMB,
			value:   "abc",
			wantErr: "must be a whole number",
		},
		{
			name:    "cleared numeric setting",
			setting: config.SettingCacheInMemoryMaxMB,
			value:   "",
			wantErr: "must be a whole number",
		},
		{
			name:    "yes in a boolean setting",
			setting: config.SettingMaintenanceMode,
			value:   "yes",
			wantErr: "must be true or false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			svc, repo := newTestService(t)
			primeValidCache(repo)
			updatedBy := uuid.New()

			// when
			err := svc.Set(t.Context(), tt.setting, tt.value, updatedBy)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSet_RegisteredValidatorBlocksWrite(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	updatedBy := uuid.New()

	svc.RegisterValidator(config.SettingValkeyURL, func(_ context.Context, _ string) error {
		return errors.New("cannot reach valkey")
	})

	// when
	err := svc.Set(t.Context(), config.SettingValkeyURL, "redis://nothing-here:6379", updatedBy)

	// then
	require.ErrorIs(t, err, ErrInvalidValue)
	assert.EqualError(t, err, "invalid setting: valkey_url: cannot reach valkey")
}

func TestSet_ValidatorSkippedWhenValueUnchanged(t *testing.T) {
	// given
	svc, repo := newTestService(t)
	primeValidCache(repo)
	updatedBy := uuid.New()

	ran := false
	svc.RegisterValidator(config.SettingValkeyURL, func(_ context.Context, _ string) error {
		ran = true

		return errors.New("should not run")
	})

	repo.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: config.SettingValkeyURL.Key, Value: config.SettingValkeyURL.Default, UpdatedBy: updatedBy}).Return(nil)

	// when
	err := svc.Set(t.Context(), config.SettingValkeyURL, config.SettingValkeyURL.Default, updatedBy)

	// then
	require.NoError(t, err)
	assert.False(t, ran)
}
