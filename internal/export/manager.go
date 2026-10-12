package export

import (
	"sync"

	"retrotide/internal/export/engine"
)

type (
	EngineManager struct {
		factory *EngineFactory

		mu      sync.Mutex
		engines map[engine.Format]engine.Engine
	}
)

func NewEngineManager() *EngineManager {
	return &EngineManager{factory: NewEngineFactory(), engines: map[engine.Format]engine.Engine{}}
}

func (m *EngineManager) Engine(format engine.Format) (engine.Engine, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.engines[format]; ok {
		return existing, nil
	}

	created, err := m.factory.Create(format)
	if err != nil {
		return nil, err
	}

	m.engines[format] = created

	return created, nil
}
