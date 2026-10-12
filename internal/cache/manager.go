package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"retrotide/internal/cache/engine"
	"retrotide/internal/logger"

	"github.com/prometheus/client_golang/prometheus"
)

type (
	Manager struct {
		engines  []engine.Engine
		active   atomic.Pointer[activation]
		switchMu sync.Mutex
	}

	activation struct {
		index      int
		generation uint64
	}
)

const (
	flushTimeout = 5 * time.Second
)

var (
	fallbackOrder = []engine.Kind{engine.KindValkey, engine.KindInMemory}

	cacheHits = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Number of cache lookups that returned a value.",
	})
	cacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Number of cache lookups that found no value.",
	})
)

func init() {
	prometheus.MustRegister(cacheHits, cacheMisses)
}

func New(reg prometheus.Registerer) (*Manager, error) {
	factory := NewEngineFactory(reg)

	candidates := make([]engine.Engine, 0, len(fallbackOrder))
	for _, kind := range fallbackOrder {
		created, err := factory.Create(kind)
		if err != nil {
			return nil, fmt.Errorf("cache: %w", err)
		}

		candidates = append(candidates, created)
	}

	return NewManager(candidates...), nil
}

func NewManager(candidates ...engine.Engine) *Manager {
	return &Manager{engines: candidates}
}

func (m *Manager) Engines() []engine.Engine {
	if m == nil {
		return nil
	}

	return m.engines
}

func (m *Manager) Del(ctx context.Context, keys ...string) error {
	if m == nil || len(keys) == 0 {
		return nil
	}

	var errs []error
	for _, candidate := range m.engines {
		err := candidate.Del(ctx, keys...)
		if err != nil {
			errs = append(errs, fmt.Errorf("cache engine %s: %w", candidate.Kind(), err))
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) Ping(ctx context.Context) error {
	if m == nil {
		return nil
	}

	var errs []error
	for _, candidate := range m.engines {
		err := candidate.Ping(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("cache engine %s: %w", candidate.Kind(), err))
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}

	var errs []error
	for _, candidate := range m.engines {
		err := candidate.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("cache engine %s: %w", candidate.Kind(), err))
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) getBytes(ctx context.Context, key string) ([]byte, error) {
	active := m.current(ctx)
	if active == nil {
		return nil, ErrMiss
	}

	data, err := active.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrMiss) {
			cacheMisses.Inc()
		}

		return nil, err
	}

	cacheHits.Inc()

	return data, nil
}

func (m *Manager) setBytes(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	active := m.current(ctx)
	if active == nil {
		return nil
	}

	return active.Set(ctx, key, data, ttl)
}

func (m *Manager) setIfAbsentBytes(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	active := m.current(ctx)
	if active == nil {
		return nil
	}

	return active.SetIfAbsent(ctx, key, data, ttl)
}

func (m *Manager) setManyBytes(ctx context.Context, entries map[string][]byte, ttl time.Duration) error {
	active := m.current(ctx)
	if active == nil || len(entries) == 0 {
		return nil
	}

	return active.SetMany(ctx, entries, ttl)
}

func (m *Manager) current(ctx context.Context) engine.Engine {
	if m == nil {
		return nil
	}

	for index, candidate := range m.engines {
		if !candidate.Enabled() {
			continue
		}

		if err := m.activate(ctx, index, candidate); err != nil {
			logger.Ctx(ctx).Warn().Err(err).Msg("cache engine not used because it could not be flushed")
			continue
		}

		return candidate
	}

	return nil
}

func (m *Manager) activate(ctx context.Context, index int, candidate engine.Engine) error {
	generation := candidate.Generation()

	if current := m.active.Load(); current != nil && current.index == index && current.generation == generation {
		return nil
	}

	m.switchMu.Lock()
	defer m.switchMu.Unlock()

	if current := m.active.Load(); current != nil && current.index == index && current.generation == generation {
		return nil
	}

	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), flushTimeout)
	defer cancel()

	if err := candidate.Flush(flushCtx); err != nil {
		return fmt.Errorf("flush cache engine %s: %w", candidate.Kind(), err)
	}

	m.active.Store(&activation{index: index, generation: generation})

	return nil
}
