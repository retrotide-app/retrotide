package board

import (
	"testing"
	"time"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func timerCommand(action model.TimerAction, durationMS int) TimerCommand {
	return TimerCommand{Command: baseCommand(), Action: action, DurationMS: durationMS}
}

func TestNextTimer_EveryActionFromEveryState(t *testing.T) {
	now := stamp
	running := model.BoardTimer{DurationMS: new(300000), EndsAt: new(now.Add(2 * time.Minute))}
	paused := model.BoardTimer{DurationMS: new(300000), PausedRemainingMS: new(90000)}
	finished := model.BoardTimer{DurationMS: new(300000), EndsAt: new(now.Add(-time.Second))}
	nearlyAnHour := model.BoardTimer{DurationMS: new(3590000), EndsAt: new(now.Add(time.Minute))}
	finishedHour := model.BoardTimer{DurationMS: new(3600000), EndsAt: new(now.Add(-time.Second))}

	tests := []struct {
		name    string
		current model.BoardTimer
		action  model.TimerAction
		minutes int
		want    model.BoardTimer
		err     error
	}{
		{name: "start with no timer", action: model.TimerActionStart, minutes: 5, want: model.BoardTimer{DurationMS: new(300000), EndsAt: new(now.Add(5 * time.Minute))}},
		{name: "start over a running timer", current: running, action: model.TimerActionStart, minutes: 1, want: model.BoardTimer{DurationMS: new(60000), EndsAt: new(now.Add(time.Minute))}},
		{name: "pause a running timer", current: running, action: model.TimerActionPause, want: model.BoardTimer{DurationMS: new(300000), PausedRemainingMS: new(120000)}},
		{name: "pause with no timer", action: model.TimerActionPause, err: ErrTimerNotRunning},
		{name: "pause a paused timer", current: paused, action: model.TimerActionPause, err: ErrTimerNotRunning},
		{name: "pause a finished timer", current: finished, action: model.TimerActionPause, err: ErrTimerNotRunning},
		{name: "resume a paused timer", current: paused, action: model.TimerActionResume, want: model.BoardTimer{DurationMS: new(300000), EndsAt: new(now.Add(90 * time.Second))}},
		{name: "resume a running timer", current: running, action: model.TimerActionResume, err: ErrTimerNotPaused},
		{name: "add a minute to a running timer", current: running, action: model.TimerActionAddMinute, want: model.BoardTimer{DurationMS: new(360000), EndsAt: new(now.Add(3 * time.Minute))}},
		{name: "add a minute to a paused timer", current: paused, action: model.TimerActionAddMinute, want: model.BoardTimer{DurationMS: new(360000), PausedRemainingMS: new(150000)}},
		{name: "add a minute to a finished timer", current: finished, action: model.TimerActionAddMinute, want: model.BoardTimer{DurationMS: new(60000), EndsAt: new(now.Add(time.Minute))}},
		{name: "add a minute to a finished hour long timer", current: finishedHour, action: model.TimerActionAddMinute, want: model.BoardTimer{DurationMS: new(60000), EndsAt: new(now.Add(time.Minute))}},
		{name: "add a minute with no timer", action: model.TimerActionAddMinute, err: ErrTimerNotRunning},
		{name: "add a minute past an hour", current: nearlyAnHour, action: model.TimerActionAddMinute, err: ErrTimerTooLong},
		{name: "stop a running timer", current: running, action: model.TimerActionStop, want: model.BoardTimer{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := nextTimer(tc.current, timerCommand(tc.action, tc.minutes*60000), now)

			// then
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestControlTimer_StartsTheTimerAndTellsEveryoneWithTheServersClock(t *testing.T) {
	// given
	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return stamp
	}
	access := boardAccess(host)
	m.entered(access)
	timer := model.BoardTimer{DurationMS: new(300000), EndsAt: new(stamp.Add(5 * time.Minute))}
	started := access.Header.Board
	started.TimerDurationMS = timer.DurationMS
	started.TimerEndsAt = timer.EndsAt
	m.boards.EXPECT().SetBoardTimer(mock.Anything, spec.BoardTimerWrite{BoardWrite: target(), Timer: timer}).Return(&model.Versioned[model.Board]{Value: started, Version: 8}, nil)

	// when
	got, err := svc.ControlTimer(t.Context(), timerCommand(model.TimerActionStart, 300000))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.TimerChangedEvent{
		Type: projection.EventTimerChanged, Version: 8,
		Timer:     &dto.TimerView{DurationMS: 300000, EndsAt: new("2026-10-04T12:05:00.000Z")},
		ServerNow: "2026-10-04T12:00:00.000Z",
	}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.Equal(t, started, m.events.published[0].access.Header.Board)
}

func TestControlTimer_AStateThatDoesNotAllowTheActionWritesNothing(t *testing.T) {
	// given a board with no timer
	svc, m := newTestService(t)
	m.entered(boardAccess(host))

	// when
	got, err := svc.ControlTimer(t.Context(), timerCommand(model.TimerActionPause, 0))

	// then the strict mocks prove nothing was written
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrTimerNotRunning)
	assert.Empty(t, m.events.published)
}

func TestControlTimer_IsTheFacilitatorsAlone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.ControlTimer(t.Context(), timerCommand(model.TimerActionStart, 60000))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func autoLocking(access *model.BoardAccess) {
	access.Header.TimerAutoLock = true
}

func TestControlTimer_AnyoneCanEditLetsAParticipantRunTheTimer(t *testing.T) {
	// given
	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return stamp
	}
	access := boardAccess(writer, anyoneCanEdit)
	m.entered(access)
	timer := model.BoardTimer{DurationMS: new(60000), EndsAt: new(stamp.Add(time.Minute))}
	started := access.Header.Board
	started.TimerDurationMS = timer.DurationMS
	started.TimerEndsAt = timer.EndsAt
	m.boards.EXPECT().SetBoardTimer(mock.Anything, spec.BoardTimerWrite{BoardWrite: target(), Timer: timer}).Return(&model.Versioned[model.Board]{Value: started, Version: 8}, nil)

	// when
	got, err := svc.ControlTimer(t.Context(), timerCommand(model.TimerActionStart, 60000))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
}

func TestControlTimer_ALockedBoardStopsAParticipantEvenWhenAnyoneCanEdit(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(writer, anyoneCanEdit)
	access.Header.LockedAt = new(stamp)
	m.entered(access)

	// when
	got, err := svc.ControlTimer(t.Context(), timerCommand(model.TimerActionStart, 60000))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrReadOnly)
}

func TestControlTimer_StartingATimerOnAnAutoLockingBoardSchedulesTheLock(t *testing.T) {
	// given
	svc, m := newTestService(t)
	timers := fakeTimersFor(svc.timers)
	svc.now = func() time.Time {
		return stamp
	}
	access := boardAccess(host, autoLocking)
	m.entered(access)
	started := access.Header.Board
	started.TimerDurationMS = new(300000)
	started.TimerEndsAt = new(stamp.Add(5 * time.Minute))
	m.boards.EXPECT().SetBoardTimer(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: started, Version: 8}, nil)

	// when
	_, err := svc.ControlTimer(t.Context(), timerCommand(model.TimerActionStart, 300000))

	// then the lock fires when the timer ends
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{5 * time.Minute}, timers.live())
	assert.Contains(t, svc.timers.timers, boardID)
}

func TestControlTimer_StoppingOrPausingTheTimerCancelsTheLock(t *testing.T) {
	tests := []struct {
		name    string
		action  model.TimerAction
		written func(board model.Board) model.Board
	}{
		{name: "stop", action: model.TimerActionStop, written: func(board model.Board) model.Board {
			board.TimerDurationMS = nil
			board.TimerEndsAt = nil

			return board
		}},
		{name: "pause", action: model.TimerActionPause, written: func(board model.Board) model.Board {
			board.TimerEndsAt = nil
			board.TimerPausedRemainingMS = new(120000)

			return board
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a running timer whose lock is scheduled
			svc, m := newTestService(t)
			timers := fakeTimersFor(svc.timers)
			svc.now = func() time.Time {
				return stamp
			}
			access := boardAccess(host, autoLocking)
			access.Header.TimerDurationMS = new(300000)
			access.Header.TimerEndsAt = new(stamp.Add(2 * time.Minute))
			svc.timers.Schedule(model.TimerDue{BoardID: boardID, OrgID: orgID, EndsAt: *access.Header.TimerEndsAt})
			m.entered(access)
			m.boards.EXPECT().SetBoardTimer(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: tc.written(access.Header.Board), Version: 8}, nil)

			// when
			_, err := svc.ControlTimer(t.Context(), timerCommand(tc.action, 0))

			// then
			require.NoError(t, err)
			assert.Empty(t, timers.live())
			assert.NotContains(t, svc.timers.timers, boardID)
		})
	}
}

func TestControlTimer_RefusesABadRequestBeforeTakingTheGate(t *testing.T) {
	tests := []struct {
		name    string
		command TimerCommand
		want    error
	}{
		{name: "an unknown action", command: timerCommand("rewind", 0), want: ErrInvalidTimerAction},
		{name: "a start with no duration", command: timerCommand(model.TimerActionStart, 0), want: ErrInvalidTimerDuration},
		{name: "a start under a second", command: timerCommand(model.TimerActionStart, 999), want: ErrInvalidTimerDuration},
		{name: "a start over an hour", command: timerCommand(model.TimerActionStart, 3600001), want: ErrInvalidTimerDuration},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.ControlTimer(t.Context(), tc.command)

			// then the strict mocks prove the gate was never asked for
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}
