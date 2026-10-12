package cache

import (
	"errors"
	"testing"

	"retrotide/internal/cache/engine"
	"retrotide/internal/cache/engines"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineFactoryCreatesTheEngineForEachKind(t *testing.T) {
	tests := []struct {
		kind engine.Kind
		want engine.Engine
	}{
		{kind: engine.KindValkey, want: &engines.Valkey{}},
		{kind: engine.KindInMemory, want: &engines.InMemory{}},
	}

	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			// when
			got, err := NewEngineFactory(prometheus.NewRegistry()).Create(tc.kind)

			// then
			require.NoError(t, err)
			assert.IsType(t, tc.want, got)
			assert.Equal(t, tc.kind, got.Kind())
		})
	}
}

func TestEngineFactoryRefusesAnUnknownKind(t *testing.T) {
	// when
	_, err := NewEngineFactory(prometheus.NewRegistry()).Create("memcached")

	// then
	require.ErrorIs(t, err, ErrUnknownEngine)
}

func TestEngineFactoryReportsAFailedValkeyStatsRegistration(t *testing.T) {
	// given a registry that already holds the valkey stats
	factory := NewEngineFactory(prometheus.NewRegistry())

	_, err := factory.Create(engine.KindValkey)
	require.NoError(t, err)

	// when
	_, err = factory.Create(engine.KindValkey)

	// then
	_, isDuplicate := errors.AsType[prometheus.AlreadyRegisteredError](err)
	assert.True(t, isDuplicate)
}
