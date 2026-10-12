package health

import (
	"database/sql"
	"errors"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	healthgo "github.com/hellofresh/health-go/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	unreachablePostgresDSN = "postgres://retrotide@127.0.0.1:1/retrotide?sslmode=disable&connect_timeout=1"
)

func newTestService(t *testing.T, database *sql.DB, pinger Pinger) Service {
	t.Helper()

	settingsService := settings.NewMockService(t)
	settingsService.EXPECT().Get(mock.Anything, config.SettingSiteName).Return("RetroTide", nil)

	svc, err := NewService(database, pinger, settingsService, "test-version")
	require.NoError(t, err)

	return svc
}

func TestMeasure_UnavailableWhenPostgresFails(t *testing.T) {
	// given a database nothing listens for, and a healthy cache
	database, err := sql.Open("pgx", unreachablePostgresDSN)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, database.Close())
	})

	pinger := NewMockPinger(t)
	pinger.EXPECT().Ping(mock.Anything).Return(nil)

	svc := newTestService(t, database, pinger)

	// when
	check := svc.Measure(t.Context())

	// then the hard postgres check fails the whole measurement
	assert.Equal(t, healthgo.StatusUnavailable, check.Status)
	assert.Contains(t, check.Failures, "postgres")
	assert.NotContains(t, check.Failures, "valkey")
	assert.Equal(t, healthgo.Component{Name: "RetroTide", Version: "test-version"}, check.Component)
}

func TestNewService_ReturnsTheSiteNameReadError(t *testing.T) {
	// given
	readErr := errors.New("db down")
	settingsService := settings.NewMockService(t)
	settingsService.EXPECT().Get(mock.Anything, config.SettingSiteName).Return("", readErr)

	// when
	svc, err := NewService(nil, NewMockPinger(t), settingsService, "test-version")

	// then
	require.ErrorIs(t, err, readErr)
	assert.Nil(t, svc)
}
