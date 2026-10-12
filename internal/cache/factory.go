package cache

import (
	"fmt"

	"retrotide/internal/cache/engine"
	"retrotide/internal/cache/engines"
	"retrotide/internal/cache/engines/observability"

	"github.com/prometheus/client_golang/prometheus"
)

type (
	EngineFactory struct {
		constructors map[engine.Kind]func() (engine.Engine, error)
	}
)

func NewEngineFactory(reg prometheus.Registerer) *EngineFactory {
	return &EngineFactory{constructors: map[engine.Kind]func() (engine.Engine, error){
		engine.KindValkey: func() (engine.Engine, error) {
			valkey := engines.NewValkey()
			if err := observability.RegisterStats(reg, valkey.Client); err != nil {
				return nil, fmt.Errorf("register the valkey stats: %w", err)
			}

			return valkey, nil
		},
		engine.KindInMemory: func() (engine.Engine, error) {
			return engines.NewInMemory(0), nil
		},
	}}
}

func (f *EngineFactory) Create(kind engine.Kind) (engine.Engine, error) {
	construct, ok := f.constructors[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownEngine, kind)
	}

	created, err := construct()
	if err != nil {
		return nil, fmt.Errorf("create the %s cache engine: %w", kind, err)
	}

	return created, nil
}
