package health

import (
	"errors"
	"testing"

	"retrotide/internal/db/dbtest"

	healthgo "github.com/hellofresh/health-go/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPostgres_Measure_OKWithHealthyDependencies(t *testing.T) {
	// given
	database, _ := dbtest.NewEmptyDatabase(t)
	pinger := NewMockPinger(t)
	pinger.EXPECT().Ping(mock.Anything).Return(nil)

	svc := newTestService(t, database, pinger)

	// when
	check := svc.Measure(t.Context())

	// then
	assert.Equal(t, healthgo.StatusOK, check.Status)
	assert.Empty(t, check.Failures)
}

func TestPostgres_Measure_PartiallyAvailableWhenValkeyFails(t *testing.T) {
	// given a reachable database and a cache that cannot be pinged
	database, _ := dbtest.NewEmptyDatabase(t)
	pinger := NewMockPinger(t)
	pinger.EXPECT().Ping(mock.Anything).Return(errors.New("valkey unreachable"))

	svc := newTestService(t, database, pinger)

	// when
	check := svc.Measure(t.Context())

	// then the soft valkey check degrades the measurement without failing it
	assert.Equal(t, healthgo.StatusPartiallyAvailable, check.Status)
	assert.Equal(t, map[string]string{"valkey": "valkey unreachable"}, check.Failures)
}
