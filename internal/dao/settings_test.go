package dao_test

import (
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsDAO_SetAndGet(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	updatedBy := uuid.New()

	// when
	err := repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "site_name", Value: "RetroTide", UpdatedBy: updatedBy})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.Get(t.Context(), "site_name")
	require.NoError(t, err)
	assert.Equal(t, "RetroTide", got)
}

func TestSettingsDAO_Set_WithNilUser(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "anon_key", Value: "anon_value", UpdatedBy: uuid.Nil})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.Get(t.Context(), "anon_key")
	require.NoError(t, err)
	assert.Equal(t, "anon_value", got)
}

func TestSettingsDAO_Set_Upsert(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	updatedBy := uuid.New()
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "theme", Value: "light", UpdatedBy: updatedBy}))

	// when
	err := repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "theme", Value: "dark", UpdatedBy: updatedBy})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.Get(t.Context(), "theme")
	require.NoError(t, err)
	assert.Equal(t, "dark", got)
}

func TestSettingsDAO_Get_NotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	_, err := repos.Settings.Get(t.Context(), "missing_key")

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Contains(t, err.Error(), "missing_key")
}

func TestSettingsDAO_GetAll_Empty(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	got, err := repos.Settings.GetAll(t.Context())

	// then
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSettingsDAO_GetAll(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	updatedBy := uuid.New()
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "alpha", Value: "1", UpdatedBy: updatedBy}))
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "beta", Value: "2", UpdatedBy: updatedBy}))
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "gamma", Value: "3", UpdatedBy: uuid.Nil}))

	// when
	got, err := repos.Settings.GetAll(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, map[config.SiteSettingKey]string{
		"alpha": "1",
		"beta":  "2",
		"gamma": "3",
	}, got)
}

func TestSettingsDAO_SetMultiple(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	updatedBy := uuid.New()
	settings := map[config.SiteSettingKey]string{
		"colour":   "blue",
		"language": "en-GB",
		"timezone": "UTC",
	}

	// when
	err := repos.Settings.SetMultiple(t.Context(), spec.SettingsBulkUpdate{Values: settings, UpdatedBy: updatedBy})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.GetAll(t.Context())
	require.NoError(t, err)
	assert.Equal(t, settings, got)
}

func TestSettingsDAO_SetMultiple_Empty(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	updatedBy := uuid.New()

	// when
	err := repos.Settings.SetMultiple(t.Context(), spec.SettingsBulkUpdate{Values: map[config.SiteSettingKey]string{}, UpdatedBy: updatedBy})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.GetAll(t.Context())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSettingsDAO_SetMultiple_Upsert(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	updatedBy := uuid.New()
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "colour", Value: "red", UpdatedBy: updatedBy}))
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "extra", Value: "keep", UpdatedBy: updatedBy}))

	// when
	err := repos.Settings.SetMultiple(t.Context(), spec.SettingsBulkUpdate{
		Values: map[config.SiteSettingKey]string{
			"colour": "green",
			"size":   "large",
		},
		UpdatedBy: updatedBy,
	})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.GetAll(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[config.SiteSettingKey]string{
		"colour": "green",
		"extra":  "keep",
		"size":   "large",
	}, got)
}

func TestSettingsDAO_SetMultiple_NilUser(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Settings.SetMultiple(t.Context(), spec.SettingsBulkUpdate{
		Values: map[config.SiteSettingKey]string{
			"a": "1",
			"b": "2",
		},
		UpdatedBy: uuid.Nil,
	})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.GetAll(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[config.SiteSettingKey]string{"a": "1", "b": "2"}, got)
}

func TestSettingsDAO_Delete(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	updatedBy := uuid.New()
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "to_delete", Value: "value", UpdatedBy: updatedBy}))
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "to_keep", Value: "value", UpdatedBy: updatedBy}))

	// when
	err := repos.Settings.Delete(t.Context(), "to_delete")

	// then
	require.NoError(t, err)
	_, getErr := repos.Settings.Get(t.Context(), "to_delete")
	assert.ErrorIs(t, getErr, dao.ErrNotFound)
	kept, err := repos.Settings.Get(t.Context(), "to_keep")
	require.NoError(t, err)
	assert.Equal(t, "value", kept)
}

func TestSettingsDAO_Delete_NotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.Settings.Delete(t.Context(), "never_existed")

	// then
	assert.NoError(t, err)
}

func TestSettingsRepository_ReconcileSeedsWithoutTouchingOtherRows(t *testing.T) {
	// given a row this binary does not know, as a newer release leaves behind on a rollback
	repos := daotest.NewRepos(t)
	require.NoError(t, repos.Settings.Set(t.Context(), spec.SettingsUpdate{Key: "setting_from_a_newer_release", Value: "tuned", UpdatedBy: uuid.New()}))

	// when
	err := repos.Settings.Reconcile(t.Context(), spec.SettingsReconcile{
		Missing:   map[config.SiteSettingKey]string{config.SettingSiteName.Key: config.SettingSiteName.Default},
		UpdatedBy: uuid.Nil,
	})

	// then
	require.NoError(t, err)
	got, err := repos.Settings.GetAll(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[config.SiteSettingKey]string{
		config.SettingSiteName.Key:     config.SettingSiteName.Default,
		"setting_from_a_newer_release": "tuned",
	}, got)
}
