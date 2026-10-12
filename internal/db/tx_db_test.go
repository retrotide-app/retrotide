package db_test

import (
	"database/sql"
	"errors"
	"testing"

	"retrotide/internal/db"
	"retrotide/internal/db/dbtest"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	insertSetting       = `INSERT INTO site_settings (key, value) VALUES ($1, $2)`
	readOnlyTransaction = "25006"
)

func newMigratedDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, _ := dbtest.NewEmptyDatabase(t)
	require.NoError(t, db.Migrate(database))

	return database
}

func countSettings(t *testing.T, handle db.DBTX, key string) int {
	t.Helper()

	var count int
	require.NoError(t, handle.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM site_settings WHERE key = $1`, key).Scan(&count))

	return count
}

func TestPostgres_WithTx_CommitsWhenFnSucceeds(t *testing.T) {
	// given
	database := newMigratedDatabase(t)

	// when
	err := db.WithTx(t.Context(), database, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), insertSetting, "theme", "dark")

		return err
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, countSettings(t, database, "theme"))
}

func TestPostgres_WithTx_RollsBackWhenFnFails(t *testing.T) {
	// given
	database := newMigratedDatabase(t)
	failure := errors.New("fn failed")

	// when
	err := db.WithTx(t.Context(), database, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(t.Context(), insertSetting, "theme", "dark"); err != nil {
			return err
		}

		return failure
	})

	// then
	require.ErrorIs(t, err, failure)
	assert.Zero(t, countSettings(t, database, "theme"))
}

func TestPostgres_WithReadTx_RunsAtRepeatableRead(t *testing.T) {
	// given
	database := newMigratedDatabase(t)
	var isolation string

	// when
	err := db.WithReadTx(t.Context(), database, nil, func(tx *sql.Tx) error {
		return tx.QueryRowContext(t.Context(), `SHOW transaction_isolation`).Scan(&isolation)
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, "repeatable read", isolation)
}

func TestPostgres_WithReadTx_RefusesWrites(t *testing.T) {
	// given
	database := newMigratedDatabase(t)

	// when
	err := db.WithReadTx(t.Context(), database, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), insertSetting, "theme", "dark")

		return err
	})

	// then
	pgErr, isPgErr := errors.AsType[*pgconn.PgError](err)
	require.True(t, isPgErr, "expected a postgres error, got %v", err)
	assert.Equal(t, readOnlyTransaction, pgErr.Code)
	assert.Zero(t, countSettings(t, database, "theme"))
}

func TestPostgres_WithReadTx_KeepsItsSnapshot(t *testing.T) {
	// given
	database := newMigratedDatabase(t)
	var before, after int

	// when
	err := db.WithReadTx(t.Context(), database, nil, func(tx *sql.Tx) error {
		before = countSettings(t, tx, "theme")

		if _, err := database.ExecContext(t.Context(), insertSetting, "theme", "dark"); err != nil {
			return err
		}

		after = countSettings(t, tx, "theme")

		return nil
	})

	// then
	require.NoError(t, err)
	assert.Zero(t, before)
	assert.Zero(t, after)
	assert.Equal(t, 1, countSettings(t, database, "theme"))
}
