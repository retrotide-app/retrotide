package dao_test

import (
	"testing"

	"retrotide/internal/db"
	"retrotide/internal/db/dbtest"

	"github.com/stretchr/testify/require"
)

const (
	beforeFirstMigration = 0
)

func TestMigrations_DownAndUpRoundTrip(t *testing.T) {
	// given
	conn, _ := dbtest.NewEmptyDatabase(t)
	require.NoError(t, db.Migrate(conn))

	// when
	require.NoError(t, db.MigrateDownTo(conn, beforeFirstMigration))

	// then
	require.NoError(t, db.Migrate(conn))
}
