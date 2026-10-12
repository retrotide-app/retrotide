package board

import (
	"fmt"
	"testing"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func autoLockHeader() *model.BoardHeader {
	header := boardAccess(host).Header
	header.TimerAutoLock = true
	header.TimerDurationMS = new(300000)
	header.TimerEndsAt = new(stamp)

	return &header
}

func autoLockDue() model.TimerDue {
	return model.TimerDue{BoardID: boardID, OrgID: orgID, EndsAt: stamp}
}

func timerLockWrite() spec.BoardTimerLockWrite {
	return spec.BoardTimerLockWrite{BoardWrite: spec.BoardWrite{OrgID: orgID, BoardID: boardID, Version: 7}, EndsAt: stamp}
}

func TestExpireTimer_LocksTheBoardAndMakesEveryoneRefetch(t *testing.T) {
	// given
	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return stamp
	}
	header := autoLockHeader()
	locked := header.Board
	locked.LockedAt = new(stamp)
	m.gate()
	m.boards.EXPECT().GetHeader(mock.Anything, boardID).Return(header, nil).Once()
	m.boards.EXPECT().LockBoardAtTimerEnd(mock.Anything, timerLockWrite()).Return(&model.Versioned[model.Board]{Value: locked, Version: 8}, nil).Once()
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()

	// when
	err := svc.expireTimer(t.Context(), autoLockDue())

	// then the lock narrows what everyone may do, so it comes back through a snapshot
	require.NoError(t, err)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.Equal(t, locked, m.events.published[0].access.Header.Board)
}

func TestExpireTimer_AStaleFiringWritesNothing(t *testing.T) {
	tests := []struct {
		name  string
		apply func(header *model.BoardHeader)
	}{
		{name: "a deleted board", apply: func(header *model.BoardHeader) {
			header.DeletedAt = new(stamp)
		}},
		{name: "a deleted organisation", apply: func(header *model.BoardHeader) {
			header.OrgDeletedAt = new(stamp)
		}},
		{name: "a closed board", apply: func(header *model.BoardHeader) {
			header.ClosedAt = new(stamp)
		}},
		{name: "an archived board", apply: func(header *model.BoardHeader) {
			header.ArchivedAt = new(stamp)
		}},
		{name: "a board already locked", apply: func(header *model.BoardHeader) {
			header.LockedAt = new(stamp)
		}},
		{name: "auto-lock turned off", apply: func(header *model.BoardHeader) {
			header.TimerAutoLock = false
		}},
		{name: "a stopped timer", apply: func(header *model.BoardHeader) {
			header.TimerDurationMS = nil
			header.TimerEndsAt = nil
		}},
		{name: "a timer that now ends later", apply: func(header *model.BoardHeader) {
			header.TimerEndsAt = new(stamp.Add(time.Minute))
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			svc.now = func() time.Time {
				return stamp
			}
			header := autoLockHeader()
			tc.apply(header)
			m.gate()
			m.boards.EXPECT().GetHeader(mock.Anything, boardID).Return(header, nil).Once()

			// when
			err := svc.expireTimer(t.Context(), autoLockDue())

			// then the strict mocks prove nothing was written
			require.NoError(t, err)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestExpireTimer_ABoardThatIsGoneIsIgnored(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.gate()
	m.boards.EXPECT().GetHeader(mock.Anything, boardID).Return(nil, errNotFound).Once()

	// when
	err := svc.expireTimer(t.Context(), autoLockDue())

	// then
	require.NoError(t, err)
	assert.Empty(t, m.events.published)
}

func TestExpireTimer_AFailedReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.gate()
	m.boards.EXPECT().GetHeader(mock.Anything, boardID).Return(nil, errDatabase).Once()

	// when
	err := svc.expireTimer(t.Context(), autoLockDue())

	// then
	require.ErrorIs(t, err, errDatabase)
}

func TestExpireTimer_ABusyGateIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Acquire(mock.Anything, boardID).Return(nil, live.ErrBusy).Once()

	// when
	err := svc.expireTimer(t.Context(), autoLockDue())

	// then the registry retries a firing that failed
	require.ErrorIs(t, err, live.ErrBusy)
}

func TestExpireTimer_ATimerMovedBeforeTheWriteIsANoOp(t *testing.T) {
	// given
	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return stamp
	}
	m.gate()
	m.boards.EXPECT().GetHeader(mock.Anything, boardID).Return(autoLockHeader(), nil).Once()
	m.boards.EXPECT().LockBoardAtTimerEnd(mock.Anything, timerLockWrite()).Return(nil, fmt.Errorf("lock: %w", dao.ErrTimerMoved)).Once()

	// when
	err := svc.expireTimer(t.Context(), autoLockDue())

	// then
	require.NoError(t, err)
	assert.Empty(t, m.events.published)
}

func TestExpireTimer_AFailedLockIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return stamp
	}
	m.gate()
	m.boards.EXPECT().GetHeader(mock.Anything, boardID).Return(autoLockHeader(), nil).Once()
	m.boards.EXPECT().LockBoardAtTimerEnd(mock.Anything, timerLockWrite()).Return(nil, errDatabase).Once()

	// when
	err := svc.expireTimer(t.Context(), autoLockDue())

	// then
	require.ErrorIs(t, err, errDatabase)
	assert.Empty(t, m.events.published)
}

func TestExpireTimer_AnEarlyFiringIsScheduledAgainForTheTimersEnd(t *testing.T) {
	// given a firing two seconds before the timer ends
	svc, m := newTestService(t)
	timers := fakeTimersFor(svc.timers)
	svc.now = func() time.Time {
		return stamp.Add(-2 * time.Second)
	}
	svc.timers.now = svc.now
	m.gate()
	m.boards.EXPECT().GetHeader(mock.Anything, boardID).Return(autoLockHeader(), nil).Once()

	// when
	err := svc.expireTimer(t.Context(), autoLockDue())

	// then the strict mocks prove nothing was written
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{2 * time.Second}, timers.live())
	assert.Empty(t, m.events.published)
}

func TestRestoreTimers_SchedulesEveryRunningAutoLockTimer(t *testing.T) {
	// given one timer that ended during a deploy and one still running
	svc, m := newTestService(t)
	timers := fakeTimersFor(svc.timers)
	m.boards.EXPECT().ListAutoLockTimers(mock.Anything).Return([]model.TimerDue{
		dueIn(boardID, -time.Minute),
		dueIn(otherBoardID, 3*time.Minute),
	}, nil).Once()

	// when
	err := svc.RestoreTimers(t.Context())

	// then the timer already past its end fires at once
	require.NoError(t, err)
	assert.ElementsMatch(t, []time.Duration{0, 3 * time.Minute}, timers.live())
}

func TestRestoreTimers_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.boards.EXPECT().ListAutoLockTimers(mock.Anything).Return(nil, errDatabase).Once()

	// when
	err := svc.RestoreTimers(t.Context())

	// then
	require.ErrorIs(t, err, errDatabase)
}

func TestStopTimers_StopsEveryScheduledTimer(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	timers := fakeTimersFor(svc.timers)
	svc.timers.Schedule(autoLockDue())

	// when
	err := svc.StopTimers(t.Context())
	svc.timers.Schedule(dueIn(otherBoardID, time.Minute))

	// then
	require.NoError(t, err)
	assert.Empty(t, timers.live())
}
