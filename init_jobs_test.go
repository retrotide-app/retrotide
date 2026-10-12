package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScheduleJob_RunsAtOnceThenOnEveryTickUntilStopped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		stop := make(chan struct{})
		wg := new(sync.WaitGroup)
		var runs atomic.Int64

		// when
		scheduleJob(stop, wg, "count runs", "counted runs", time.Hour, func(context.Context) (int64, error) {
			return runs.Add(1), nil
		})
		synctest.Wait()
		afterStart := runs.Load()

		time.Sleep(2*time.Hour + time.Minute)
		synctest.Wait()
		afterTwoTicks := runs.Load()

		close(stop)
		wg.Wait()
		time.Sleep(3 * time.Hour)

		// then
		assert.Equal(t, int64(1), afterStart)
		assert.Equal(t, int64(3), afterTwoTicks)
		assert.Equal(t, int64(3), runs.Load())
	})
}

func TestScheduleJob_KeepsTickingAfterAFailedRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a sweep whose first run fails
		stop := make(chan struct{})
		wg := new(sync.WaitGroup)
		var runs atomic.Int64

		// when
		scheduleJob(stop, wg, "flaky sweep", "swept", time.Hour, func(context.Context) (int64, error) {
			if runs.Add(1) == 1 {
				return 0, errors.New("database down")
			}

			return 1, nil
		})
		time.Sleep(time.Hour + time.Minute)
		synctest.Wait()

		close(stop)
		wg.Wait()

		// then the failure is logged and the next tick runs the sweep again
		assert.Equal(t, int64(2), runs.Load())
	})
}
