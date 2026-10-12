package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"retrotide/internal/cache"
	"retrotide/internal/cache/engines"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	valkeymock "github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

const (
	settingSiteName config.SiteSettingKey = "site_name"
	settingLogLevel config.SiteSettingKey = "log_level"
	settingStaleKey config.SiteSettingKey = "stale_key"
)

func newCachedSettingsRepo(t *testing.T) (SettingsRepository, *dao.MockSettingsDAO, *valkeymock.Client) {
	t.Helper()

	client := valkeymock.NewClient(gomock.NewController(t))
	settingsDAO := dao.NewMockSettingsDAO(t)

	return NewSettingsRepo(settingsDAO, cache.NewManager(engines.NewValkeyWithClient(client))), settingsDAO, client
}

func captureSettingsCommands(client *valkeymock.Client, times int, reply func(cmd valkey.Completed) valkey.ValkeyResult) *[][]string {
	commands := new([][]string)

	client.EXPECT().
		Do(gomock.Any(), valkeymock.Match("FLUSHDB")).
		Return(valkeymock.Result(valkeymock.ValkeyString("OK"))).
		MaxTimes(1)

	client.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, cmd valkey.Completed) valkey.ValkeyResult {
			*commands = append(*commands, cmd.Commands())

			return reply(cmd)
		}).
		Times(times)

	return commands
}

func settingsMissThenStore(cmd valkey.Completed) valkey.ValkeyResult {
	if cmd.Commands()[0] == "GET" {
		return valkeymock.ErrorResult(valkey.Nil)
	}

	return valkeymock.Result(valkeymock.ValkeyString("OK"))
}

func TestSettingsGet_CachesTheDaoResult(t *testing.T) {
	// given a cold cache
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("RetroTide", nil)
	commands := captureSettingsCommands(client, 2, settingsMissThenStore)

	// when
	got, err := repo.Get(t.Context(), settingSiteName)

	// then the miss falls through to the dao, and the value is filled only if no write got there first, with no expiry
	require.NoError(t, err)
	assert.Equal(t, "RetroTide", got)
	assert.Equal(t, [][]string{
		{"GET", cache.Setting.Key(string(settingSiteName))},
		{"SET", cache.Setting.Key(string(settingSiteName)), "RetroTide", "NX"},
	}, *commands)
}

func TestSettingsSet_ReportsAFailedCacheWrite(t *testing.T) {
	// given a write that commits while the cache refuses the new value
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil}).Return(nil)
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("Harbour Retro", nil).Once()
	cacheErr := errors.New("valkey unreachable")
	commands := captureSettingsCommands(client, 2, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.ErrorResult(cacheErr)
	})

	// when
	err := repo.Set(t.Context(), spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil})

	// then the admin learns the cached copy was not refreshed, and the key is dropped as a last resort
	require.ErrorIs(t, err, ErrStaleSettingsCache)
	require.ErrorIs(t, err, cacheErr)
	assert.Equal(t, [][]string{
		{"SET", cache.Setting.Key(string(settingSiteName)), "Harbour Retro"},
		{"DEL", cache.Setting.Key(string(settingSiteName))},
	}, *commands)
}

func TestSettingsGet_ServesTheCacheWithoutTouchingTheDao(t *testing.T) {
	// given a warm cache, and a dao mock with no expectations so any call fails the test
	repo, _, client := newCachedSettingsRepo(t)
	captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString("Cached Name"))
	})

	// when
	got, err := repo.Get(t.Context(), settingSiteName)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Cached Name", got)
}

func TestSettingsGet_DaoErrorSkipsTheCacheWrite(t *testing.T) {
	// given
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("", errors.New("db down"))
	commands := captureSettingsCommands(client, 1, settingsMissThenStore)

	// when
	_, err := repo.Get(t.Context(), settingSiteName)

	// then a failed read must never be cached
	require.Error(t, err)
	require.Len(t, *commands, 1)
	assert.Equal(t, "GET", (*commands)[0][0])
}

func TestSettingsSet_WritesTheCommittedValueThrough(t *testing.T) {
	// given
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	updatedBy := uuid.New()
	settingsDAO.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: updatedBy}).Return(nil)
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("Harbour Retro", nil).Twice()
	commands := captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString("OK"))
	})

	// when
	err := repo.Set(t.Context(), spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: updatedBy})

	// then every reader sees the new value at once, and no reader can refill the key with the old one
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"SET", cache.Setting.Key(string(settingSiteName)), "Harbour Retro"}}, *commands)
}

func TestSettingsSet_FollowsAConcurrentWriteUntilTheValueSettles(t *testing.T) {
	// given another write that commits between this write's cache update and its check
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil}).Return(nil)
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("Harbour Retro", nil).Once()
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("Tidal Retro", nil).Times(3)
	commands := captureSettingsCommands(client, 2, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString("OK"))
	})

	// when
	err := repo.Set(t.Context(), spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil})

	// then the cache ends on the value the database holds last
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"SET", cache.Setting.Key(string(settingSiteName)), "Harbour Retro"},
		{"SET", cache.Setting.Key(string(settingSiteName)), "Tidal Retro"},
	}, *commands)
}

func TestSettingsSet_DropsTheKeyWhenTheValueNeverSettles(t *testing.T) {
	// given a value that changes again after every cache update
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: settingSiteName, Value: "one", UpdatedBy: uuid.Nil}).Return(nil)
	for _, value := range []string{"one", "two", "three", "four", "five", "six"} {
		settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return(value, nil).Once()
	}

	commands := captureSettingsCommands(client, 4, func(cmd valkey.Completed) valkey.ValkeyResult {
		if cmd.Commands()[0] == "DEL" {
			return valkeymock.Result(valkeymock.ValkeyInt64(1))
		}

		return valkeymock.Result(valkeymock.ValkeyString("OK"))
	})

	// when
	err := repo.Set(t.Context(), spec.SettingsUpdate{Key: settingSiteName, Value: "one", UpdatedBy: uuid.Nil})

	// then the write reports the stale risk and leaves the key for the next read to fill from the database
	require.ErrorIs(t, err, ErrStaleSettingsCache)
	require.ErrorIs(t, err, errSettingKeptChanging)
	assert.Equal(t, []string{"DEL", cache.Setting.Key(string(settingSiteName))}, (*commands)[3])
}

func TestSettingsSet_DropsTheKeyWhenTheCommittedValueCannotBeRead(t *testing.T) {
	// given
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil}).Return(nil)
	readErr := errors.New("db down")
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("", readErr).Once()
	commands := captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyInt64(1))
	})

	// when
	err := repo.Set(t.Context(), spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil})

	// then the old cached copy is dropped rather than left to be served
	require.ErrorIs(t, err, ErrStaleSettingsCache)
	require.ErrorIs(t, err, readErr)
	assert.Equal(t, [][]string{{"DEL", cache.Setting.Key(string(settingSiteName))}}, *commands)
}

func TestSettingsSet_DaoErrorSkipsInvalidation(t *testing.T) {
	// given
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Set(mock.Anything, spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil}).Return(errors.New("db down"))
	client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	err := repo.Set(t.Context(), spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro", UpdatedBy: uuid.Nil})

	// then a failed write must leave the cached value alone
	require.Error(t, err)
}

func TestSettingsSetMultiple_WritesEveryCommittedValueThrough(t *testing.T) {
	// given
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	values := map[config.SiteSettingKey]string{settingSiteName: "Harbour Retro", settingLogLevel: "debug"}
	settingsDAO.EXPECT().SetMultiple(mock.Anything, spec.SettingsBulkUpdate{Values: values, UpdatedBy: uuid.Nil}).Return(nil)
	settingsDAO.EXPECT().Get(mock.Anything, settingLogLevel).Return("debug", nil).Twice()
	settingsDAO.EXPECT().Get(mock.Anything, settingSiteName).Return("Harbour Retro", nil).Twice()
	commands := captureSettingsCommands(client, 2, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString("OK"))
	})

	// when
	err := repo.SetMultiple(t.Context(), spec.SettingsBulkUpdate{Values: values, UpdatedBy: uuid.Nil})

	// then every written key gets its committed value, in key order
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"SET", cache.Setting.Key(string(settingLogLevel)), "debug"},
		{"SET", cache.Setting.Key(string(settingSiteName)), "Harbour Retro"},
	}, *commands)
}

func TestSettingsDelete_InvalidatesTheKey(t *testing.T) {
	// given
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	settingsDAO.EXPECT().Delete(mock.Anything, settingStaleKey).Return(nil)
	commands := captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyInt64(1))
	})

	// when
	err := repo.Delete(t.Context(), settingStaleKey)

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"DEL", cache.Setting.Key(string(settingStaleKey))}}, *commands)
}

func TestSettingsWrites_RefuseAnInboundTransaction(t *testing.T) {
	tests := []struct {
		name  string
		write func(ctx context.Context, repo SettingsRepository, tx *sql.Tx) error
	}{
		{
			name: "Set",
			write: func(ctx context.Context, repo SettingsRepository, tx *sql.Tx) error {
				return repo.Set(ctx, spec.SettingsUpdate{Key: settingSiteName, Value: "Harbour Retro"}, tx)
			},
		},
		{
			name: "SetMultiple",
			write: func(ctx context.Context, repo SettingsRepository, tx *sql.Tx) error {
				return repo.SetMultiple(ctx, spec.SettingsBulkUpdate{Values: map[config.SiteSettingKey]string{settingLogLevel: "debug"}}, tx)
			},
		},
		{
			name: "Delete",
			write: func(ctx context.Context, repo SettingsRepository, tx *sql.Tx) error {
				return repo.Delete(ctx, settingStaleKey, tx)
			},
		},
		{
			name: "Reconcile",
			write: func(ctx context.Context, repo SettingsRepository, tx *sql.Tx) error {
				return repo.Reconcile(ctx, spec.SettingsReconcile{Missing: map[config.SiteSettingKey]string{settingSiteName: "RetroTide"}}, tx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given a caller-supplied transaction, and a dao mock with no expectations so any call fails the test
			repo, _, client := newCachedSettingsRepo(t)
			client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

			// when
			err := tt.write(t.Context(), repo, new(sql.Tx))

			// then the write is refused before it touches the dao or the cache
			require.ErrorIs(t, err, db.ErrInboundTx)
		})
	}
}

func TestSettingsReconcile_SeedsTheMissingSettingsAndInvalidatesThem(t *testing.T) {
	// given
	repo, settingsDAO, client := newCachedSettingsRepo(t)
	missing := map[config.SiteSettingKey]string{settingSiteName: "RetroTide", settingLogLevel: "info"}
	settingsDAO.EXPECT().SetMultiple(mock.Anything, spec.SettingsBulkUpdate{Values: missing, UpdatedBy: uuid.Nil}).Return(nil)
	commands := captureSettingsCommands(client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyInt64(2))
	})

	// when
	err := repo.Reconcile(t.Context(), spec.SettingsReconcile{Missing: missing, UpdatedBy: uuid.Nil})

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"DEL", cache.Setting.Key(string(settingLogLevel)), cache.Setting.Key(string(settingSiteName))}}, *commands)
}

func TestSettingsReconcile_NothingToDoSkipsInvalidation(t *testing.T) {
	// given a reconcile that finds the stored settings already correct
	repo, _, client := newCachedSettingsRepo(t)
	client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	err := repo.Reconcile(t.Context(), spec.SettingsReconcile{})

	// then
	require.NoError(t, err)
}
