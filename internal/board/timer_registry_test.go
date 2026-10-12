package board

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	fakeTimers struct {
		mu      sync.Mutex
		pending []*fakeTimer
	}

	fakeTimer struct {
		delay   time.Duration
		run     func()
		stopped bool
	}

	recordedFirings struct {
		mu    sync.Mutex
		dues  []model.TimerDue
		err   error
		block chan struct{}
	}
)

var (
	errLockFailed = errors.New("the lock could not be written")
	otherBoardID  = uuid.MustParse("1a2b3c4d-0000-4000-8000-00000000000b")
)

func fakeTimersFor(registry *timerRegistry) *fakeTimers {
	timers := &fakeTimers{}
	registry.afterFunc = timers.afterFunc
	registry.now = func() time.Time {
		return stamp
	}

	return timers
}

func (f *fakeTimers) afterFunc(d time.Duration, run func()) func() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	timer := &fakeTimer{delay: d, run: run}
	f.pending = append(f.pending, timer)

	return func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()

		wasPending := !timer.stopped
		timer.stopped = true

		return wasPending
	}
}

func (f *fakeTimers) at(i int) *fakeTimer {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.pending[i]
}

func (f *fakeTimers) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.pending)
}

func (f *fakeTimers) live() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()

	var delays []time.Duration

	for _, timer := range f.pending {
		if !timer.stopped {
			delays = append(delays, timer.delay)
		}
	}

	return delays
}

func (r *recordedFirings) fire(ctx context.Context, due model.TimerDue) error {
	r.mu.Lock()
	r.dues = append(r.dues, due)
	err := r.err
	block := r.block
	r.mu.Unlock()

	if block != nil {
		<-block
	}

	return err
}

func (r *recordedFirings) fired() []model.TimerDue {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]model.TimerDue(nil), r.dues...)
}

func newTestRegistry() (*timerRegistry, *fakeTimers, *recordedFirings) {
	firings := &recordedFirings{}
	registry := newTimerRegistry(firings.fire)
	timers := fakeTimersFor(registry)

	return registry, timers, firings
}

func dueIn(boardID uuid.UUID, after time.Duration) model.TimerDue {
	return model.TimerDue{BoardID: boardID, OrgID: orgID, EndsAt: stamp.Add(after)}
}

func TestTimerRegistry_ScheduleWaitsUntilTheTimerEnds(t *testing.T) {
	// given
	registry, timers, firings := newTestRegistry()

	// when
	registry.Schedule(dueIn(boardID, 5*time.Minute))

	// then
	assert.Equal(t, []time.Duration{5 * time.Minute}, timers.live())
	assert.Empty(t, firings.fired())
}

func TestTimerRegistry_ATimerAlreadyPastItsEndFiresAtOnce(t *testing.T) {
	// given
	registry, timers, _ := newTestRegistry()

	// when
	registry.Schedule(dueIn(boardID, -time.Minute))

	// then
	assert.Equal(t, []time.Duration{0}, timers.live())
}

func TestTimerRegistry_FiringHandsTheDueTimerToTheCallbackAndForgetsIt(t *testing.T) {
	// given
	registry, timers, firings := newTestRegistry()
	due := dueIn(boardID, time.Minute)
	registry.Schedule(due)

	// when
	timers.at(0).run()

	// then
	assert.Equal(t, []model.TimerDue{due}, firings.fired())
	assert.Empty(t, registry.timers)
}

func TestTimerRegistry_SchedulingABoardAgainReplacesItsTimer(t *testing.T) {
	// given
	registry, timers, firings := newTestRegistry()
	registry.Schedule(dueIn(boardID, time.Minute))
	moved := dueIn(boardID, 3*time.Minute)

	// when
	registry.Schedule(moved)
	timers.at(0).run()

	// then the replaced timer was stopped, and a late run of it fires nothing
	assert.True(t, timers.at(0).stopped)
	assert.Equal(t, []time.Duration{3 * time.Minute}, timers.live())
	assert.Empty(t, firings.fired())

	// when
	timers.at(1).run()

	// then
	assert.Equal(t, []model.TimerDue{moved}, firings.fired())
}

func TestTimerRegistry_EachBoardKeepsItsOwnTimer(t *testing.T) {
	// given
	registry, timers, _ := newTestRegistry()

	// when
	registry.Schedule(dueIn(boardID, time.Minute))
	registry.Schedule(dueIn(otherBoardID, 2*time.Minute))

	// then
	assert.ElementsMatch(t, []time.Duration{time.Minute, 2 * time.Minute}, timers.live())
}

func TestTimerRegistry_CancelStopsTheTimerAndALateRunFiresNothing(t *testing.T) {
	// given
	registry, timers, firings := newTestRegistry()
	registry.Schedule(dueIn(boardID, time.Minute))

	// when
	registry.Cancel(boardID)
	timers.at(0).run()

	// then
	assert.True(t, timers.at(0).stopped)
	assert.Empty(t, firings.fired())
	assert.Empty(t, registry.timers)
}

func TestTimerRegistry_CancellingABoardWithNoTimerDoesNothing(t *testing.T) {
	// given
	registry, timers, _ := newTestRegistry()

	// when
	registry.Cancel(boardID)

	// then
	assert.Zero(t, timers.count())
}

func TestTimerRegistry_AFailedFiringIsRetriedAfterTheRetryDelay(t *testing.T) {
	// given
	registry, timers, firings := newTestRegistry()
	firings.err = errLockFailed
	due := dueIn(boardID, time.Minute)
	registry.Schedule(due)

	// when
	timers.at(0).run()

	// then
	require.Equal(t, 2, timers.count())
	assert.Equal(t, timerRetryDelay, timers.at(1).delay)

	// when the retry runs and succeeds
	firings.err = nil
	timers.at(1).run()

	// then
	assert.Equal(t, []model.TimerDue{due, due}, firings.fired())
	assert.Empty(t, registry.timers)
}

func TestTimerRegistry_AFailedFiringIsNotRetriedOnceTheBoardIsCancelledOrRescheduled(t *testing.T) {
	tests := []struct {
		name   string
		during func(registry *timerRegistry)
		want   []time.Duration
	}{
		{
			name: "cancelled while firing",
			during: func(registry *timerRegistry) {
				registry.Cancel(boardID)
			},
		},
		{
			name: "rescheduled while firing",
			during: func(registry *timerRegistry) {
				registry.Schedule(dueIn(boardID, 4*time.Minute))
			},
			want: []time.Duration{4 * time.Minute},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			firings := &recordedFirings{}
			registry := newTimerRegistry(nil)
			timers := fakeTimersFor(registry)
			registry.fire = func(ctx context.Context, due model.TimerDue) error {
				tc.during(registry)

				return firings.fire(ctx, due)
			}
			firings.err = errLockFailed
			registry.Schedule(dueIn(boardID, time.Minute))

			// when
			timers.at(0).run()

			// then
			assert.Equal(t, tc.want, timers.live())
		})
	}
}

func TestTimerRegistry_StopStopsEveryTimerAndRefusesNewOnes(t *testing.T) {
	// given
	registry, timers, firings := newTestRegistry()
	registry.Schedule(dueIn(boardID, time.Minute))
	registry.Schedule(dueIn(otherBoardID, time.Minute))

	// when
	err := registry.Stop(t.Context())
	registry.Schedule(dueIn(boardID, 2*time.Minute))
	timers.at(0).run()

	// then
	require.NoError(t, err)
	assert.Empty(t, timers.live())
	assert.Equal(t, 2, timers.count())
	assert.Empty(t, firings.fired())
	assert.ErrorIs(t, registry.background.Err(), context.Canceled)
}

func TestTimerRegistry_StopWaitsForAFiringInFlight(t *testing.T) {
	// given a firing that holds on until it is released
	registry, timers, firings := newTestRegistry()
	firings.block = make(chan struct{})
	firings.err = errLockFailed
	registry.Schedule(dueIn(boardID, time.Minute))
	running := make(chan struct{})

	go func() {
		timers.at(0).run()
		close(running)
	}()

	require.Eventually(t, func() bool {
		return len(firings.fired()) == 1
	}, time.Second, time.Millisecond)

	stopped := make(chan error, 1)

	// when
	go func() {
		stopped <- registry.Stop(t.Context())
	}()

	// then Stop is still waiting while the firing runs
	select {
	case <-stopped:
		t.Fatal("Stop returned before the firing finished")
	case <-time.After(20 * time.Millisecond):
	}

	// when
	close(firings.block)

	// then the failed firing is not retried after Stop
	require.NoError(t, <-stopped)
	<-running
	assert.Equal(t, 1, timers.count())
}

func TestTimerRegistry_StopGivesUpWhenItsContextEnds(t *testing.T) {
	// given
	registry, timers, firings := newTestRegistry()
	firings.block = make(chan struct{})
	registry.Schedule(dueIn(boardID, time.Minute))
	running := make(chan struct{})

	go func() {
		timers.at(0).run()
		close(running)
	}()

	require.Eventually(t, func() bool {
		return len(firings.fired()) == 1
	}, time.Second, time.Millisecond)

	cause := errors.New("shutdown deadline")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)

	// when
	err := registry.Stop(ctx)

	// then
	require.ErrorIs(t, err, cause)
	close(firings.block)
	<-running
}
