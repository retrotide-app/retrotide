package db

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterStatsCollector_ReportsADuplicateRegistration(t *testing.T) {
	// given
	database := new(sql.DB)
	require.NoError(t, RegisterStatsCollector(database))
	t.Cleanup(func() {
		assert.True(t, prometheus.Unregister(newStatsCollector(database)))
	})

	// when
	err := RegisterStatsCollector(database)

	// then
	_, isDuplicate := errors.AsType[prometheus.AlreadyRegisteredError](err)
	assert.True(t, isDuplicate)
}
