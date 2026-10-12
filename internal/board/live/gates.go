package live

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

type (
	gate struct {
		slot chan struct{}
		refs int
	}

	gates struct {
		mu   sync.Mutex
		byID map[uuid.UUID]*gate
	}
)

func newGates() *gates {
	return &gates{byID: make(map[uuid.UUID]*gate)}
}

func (g *gates) Acquire(ctx context.Context, boardID uuid.UUID) (func(), error) {
	entry := g.retain(boardID)

	select {
	case entry.slot <- struct{}{}:
	case <-ctx.Done():
		g.drop(boardID, entry)
		return nil, context.Cause(ctx)
	}

	return func() {
		<-entry.slot
		g.drop(boardID, entry)
	}, nil
}

func (g *gates) retain(boardID uuid.UUID) *gate {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry, ok := g.byID[boardID]
	if !ok {
		entry = &gate{slot: make(chan struct{}, 1)}
		g.byID[boardID] = entry
	}

	entry.refs++

	return entry
}

func (g *gates) drop(boardID uuid.UUID, entry *gate) {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry.refs--
	if entry.refs == 0 {
		delete(g.byID, boardID)
	}
}
