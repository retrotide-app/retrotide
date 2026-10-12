package board

import (
	"context"
	"sync"
	"time"

	"retrotide/internal/logger"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	timerRegistry struct {
		mu          sync.Mutex
		timers      map[uuid.UUID]*scheduledTimer
		closed      bool
		firing      sync.WaitGroup
		fire        func(ctx context.Context, due model.TimerDue) error
		afterFunc   func(d time.Duration, f func()) func() bool
		now         func() time.Time
		background  context.Context
		stop        context.CancelCauseFunc
		retryDelay  time.Duration
		fireTimeout time.Duration
	}

	scheduledTimer struct {
		due  model.TimerDue
		stop func() bool
	}
)

const (
	timerRetryDelay  = 10 * time.Second
	timerFireTimeout = 10 * time.Second
)

func newTimerRegistry(fire func(ctx context.Context, due model.TimerDue) error) *timerRegistry {
	background, stop := context.WithCancelCause(context.Background())

	return &timerRegistry{
		timers: map[uuid.UUID]*scheduledTimer{},
		fire:   fire,
		afterFunc: func(d time.Duration, f func()) func() bool {
			return time.AfterFunc(d, f).Stop
		},
		now:         time.Now,
		background:  background,
		stop:        stop,
		retryDelay:  timerRetryDelay,
		fireTimeout: timerFireTimeout,
	}
}

func (r *timerRegistry) Schedule(due model.TimerDue) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	r.schedule(due, max(0, due.EndsAt.Sub(r.now())))
}

func (r *timerRegistry) Cancel(boardID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.timers[boardID]
	if !ok {
		return
	}

	entry.stop()
	delete(r.timers, boardID)
}

func (r *timerRegistry) Stop(ctx context.Context) error {
	r.mu.Lock()
	r.closed = true

	for boardID, entry := range r.timers {
		entry.stop()
		delete(r.timers, boardID)
	}

	r.mu.Unlock()

	r.stop(context.Canceled)

	done := make(chan struct{})

	go func() {
		r.firing.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (r *timerRegistry) schedule(due model.TimerDue, delay time.Duration) {
	if previous, ok := r.timers[due.BoardID]; ok {
		previous.stop()
	}

	entry := &scheduledTimer{due: due}
	entry.stop = r.afterFunc(delay, func() {
		r.run(entry)
	})

	r.timers[due.BoardID] = entry
}

func (r *timerRegistry) run(entry *scheduledTimer) {
	boardID := entry.due.BoardID

	r.mu.Lock()

	if r.closed || r.timers[boardID] != entry {
		r.mu.Unlock()

		return
	}

	r.firing.Add(1)
	r.mu.Unlock()

	defer r.firing.Done()

	ctx, cancel := context.WithTimeout(r.background, r.fireTimeout)
	defer cancel()

	err := r.fire(ctx, entry.due)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.timers[boardID] != entry {
		return
	}

	delete(r.timers, boardID)

	if err == nil || r.closed {
		return
	}

	logger.Ctx(ctx).Warn().Err(err).Str("board_id", boardID.String()).Dur("retry_in", r.retryDelay).Msg("the board's timer could not lock it, retrying")

	r.schedule(entry.due, r.retryDelay)
}
