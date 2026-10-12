package live

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGates_AdmitOneHolderAtATimePerBoard(t *testing.T) {
	// given
	g := newGates()
	gatedBoardID := uuid.New()
	const callers = 64

	var (
		wg       sync.WaitGroup
		inFlight atomic.Int32
		peak     atomic.Int32
		entered  int
	)

	// when
	for range callers {
		wg.Go(func() {
			release, err := g.Acquire(t.Context(), gatedBoardID)
			if !assert.NoError(t, err) {
				return
			}
			defer release()

			now := inFlight.Add(1)
			for {
				seen := peak.Load()
				if now <= seen || peak.CompareAndSwap(seen, now) {
					break
				}
			}

			entered++
			time.Sleep(50 * time.Microsecond)
			inFlight.Add(-1)
		})
	}
	wg.Wait()

	// then
	assert.Equal(t, int32(1), peak.Load())
	assert.Equal(t, callers, entered)
	assert.Empty(t, g.byID)
}

func TestGates_DoNotBlockAnotherBoard(t *testing.T) {
	// given
	g := newGates()
	release, err := g.Acquire(t.Context(), uuid.New())
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	// when
	other, err := g.Acquire(ctx, uuid.New())

	// then
	require.NoError(t, err)
	other()
	assert.Len(t, g.byID, 1)
}

func TestGates_HandTheSlotToAWaiterOnRelease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		g := newGates()
		gatedBoardID := uuid.New()
		release, err := g.Acquire(t.Context(), gatedBoardID)
		require.NoError(t, err)

		acquired := make(chan error, 1)
		go func() {
			next, err := g.Acquire(t.Context(), gatedBoardID)
			if err == nil {
				next()
			}
			acquired <- err
		}()
		synctest.Wait()

		// when
		release()

		// then
		require.NoError(t, <-acquired)
		assert.Empty(t, g.byID)
	})
}

func TestGates_ReturnTheContextsCauseWhenTheWaitEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		g := newGates()
		gatedBoardID := uuid.New()
		release, err := g.Acquire(t.Context(), gatedBoardID)
		require.NoError(t, err)
		defer release()

		ctx, cancel := context.WithCancelCause(t.Context())
		go cancel(ErrBusy)

		// when
		next, err := g.Acquire(ctx, gatedBoardID)

		// then
		require.ErrorIs(t, err, ErrBusy)
		assert.Nil(t, next)
		require.Contains(t, g.byID, gatedBoardID)
		assert.Equal(t, 1, g.byID[gatedBoardID].refs)
	})
}

func TestLive_Acquire_GivesUpWithErrBusyAfterTheGateWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		svc := New(Deps{})
		gatedBoardID := uuid.New()
		release, err := svc.Acquire(t.Context(), gatedBoardID)
		require.NoError(t, err)
		defer release()
		started := time.Now()

		// when
		next, err := svc.Acquire(t.Context(), gatedBoardID)

		// then
		require.ErrorIs(t, err, ErrBusy)
		assert.Nil(t, next)
		assert.Equal(t, GateWait, time.Since(started))
	})
}

func TestLive_Acquire_ReturnsCanceledWhenTheCallerGoesAway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		svc := New(Deps{})
		gatedBoardID := uuid.New()
		release, err := svc.Acquire(t.Context(), gatedBoardID)
		require.NoError(t, err)
		defer release()

		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(time.Second, cancel)

		// when
		next, err := svc.Acquire(ctx, gatedBoardID)

		// then
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, ErrBusy)
		assert.Nil(t, next)
	})
}

func TestLive_Acquire_ServesTheWaitersOfABoardOneAtATime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		svc := New(Deps{})
		gatedBoardID := uuid.New()
		const callers = 12
		var (
			wg       sync.WaitGroup
			inFlight atomic.Int32
			busy     atomic.Int32
			served   atomic.Int32
		)

		// when
		for range callers {
			wg.Go(func() {
				release, err := svc.Acquire(t.Context(), gatedBoardID)
				if err != nil {
					assert.ErrorIs(t, err, ErrBusy)
					busy.Add(1)
					return
				}
				defer release()

				assert.Equal(t, int32(1), inFlight.Add(1))
				time.Sleep(2 * time.Second)
				inFlight.Add(-1)
				served.Add(1)
			})
		}
		wg.Wait()

		// then
		assert.Equal(t, int32(3), served.Load())
		assert.Equal(t, int32(callers-3), busy.Load())
	})
}
