package board

import (
	"fmt"
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

func settingsCommand(patch spec.BoardSettingsPatch) SettingsCommand {
	return SettingsCommand{Command: baseCommand(), Settings: patch}
}

func TestChangeSettings_WritesThePatchAndAnnouncesTheNewSettings(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	patch := spec.BoardSettingsPatch{VoteCountsHidden: new(true), MaxVotes: new(3), SortMode: new(model.SortModeVotes)}
	changed := access.Header.Board
	changed.VoteCountsHidden = true
	changed.MaxVotes = 3
	changed.SortMode = model.SortModeVotes
	changed.Version = 8
	m.boards.EXPECT().SetBoardSettings(mock.Anything, spec.BoardSettingsWrite{BoardWrite: target(), Patch: patch}).Return(&model.Versioned[model.Board]{Value: changed, Version: 8}, nil)

	// when
	got, err := svc.ChangeSettings(t.Context(), settingsCommand(patch))

	// then the roster learns the new settings with the event
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.SettingsChangedEvent{
		Type: projection.EventSettingsChanged, Version: 8,
		Settings: projection.Settings(changed),
	}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.Equal(t, changed, m.events.published[0].access.Header.Board)
}

func TestChangeSettings_ShowingHiddenTotalsAgainMakesEveryoneRefetch(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, hiddenTotals)
	m.entered(access)
	shown := access.Header.Board
	shown.VoteCountsHidden = false
	m.boards.EXPECT().SetBoardSettings(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: shown, Version: 8}, nil)

	// when
	_, err := svc.ChangeSettings(t.Context(), settingsCommand(spec.BoardSettingsPatch{VoteCountsHidden: new(false)}))

	// then the totals come back through a snapshot, never through an event
	require.NoError(t, err)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
}

func TestChangeSettings_CarriesTheLocksAndTheTimerSound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	patch := spec.BoardSettingsPatch{CardsLocked: new(true), MovesLocked: new(true), TimerPlaySound: new(false)}
	changed := access.Header.Board
	changed.CardsLocked = true
	changed.MovesLocked = true
	m.boards.EXPECT().SetBoardSettings(mock.Anything, spec.BoardSettingsWrite{BoardWrite: target(), Patch: patch}).Return(&model.Versioned[model.Board]{Value: changed, Version: 8}, nil)

	// when
	got, err := svc.ChangeSettings(t.Context(), settingsCommand(patch))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	settings := m.events.published[0].event.Payload(projection.ClassOthers).(dto.SettingsChangedEvent).Settings
	assert.True(t, settings.CardsLocked)
	assert.True(t, settings.MovesLocked)
	assert.False(t, settings.BoardLocked)
	assert.False(t, settings.TimerPlaySound)
}

func TestChangeSettings_LockingTheBoardMakesEveryoneRefetchTheirCapabilities(t *testing.T) {
	for _, locking := range []bool{true, false} {
		t.Run(fmt.Sprintf("locked=%t", locking), func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			access := boardAccess(host)
			if !locking {
				access.Header.LockedAt = new(stamp)
			}
			m.entered(access)
			changed := access.Header.Board
			changed.LockedAt = nil
			if locking {
				changed.LockedAt = new(stamp)
			}
			m.boards.EXPECT().SetBoardSettings(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: changed, Version: 8}, nil)
			m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()

			// when
			_, err := svc.ChangeSettings(t.Context(), settingsCommand(spec.BoardSettingsPatch{BoardLocked: new(locking)}))

			// then a lock narrows or widens what everyone may do, so it comes back through a snapshot
			require.NoError(t, err)
			require.Len(t, m.events.published, 1)
			assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
		})
	}
}

func TestChangeSettings_CarriesAutoLockSortEnforcementAndAnyoneCanEdit(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	patch := spec.BoardSettingsPatch{TimerAutoLock: new(true), SortEnforced: new(true), AnyoneCanEdit: new(true)}
	changed := access.Header.Board
	changed.TimerAutoLock = true
	changed.SortEnforced = true
	changed.AnyoneCanEdit = true
	m.boards.EXPECT().SetBoardSettings(mock.Anything, spec.BoardSettingsWrite{BoardWrite: target(), Patch: patch}).Return(&model.Versioned[model.Board]{Value: changed, Version: 8}, nil)

	// when
	got, err := svc.ChangeSettings(t.Context(), settingsCommand(patch))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	settings := m.events.published[0].event.Payload(projection.ClassOthers).(dto.SettingsChangedEvent).Settings
	assert.True(t, settings.TimerAutoLock)
	assert.True(t, settings.SortEnforced)
	assert.True(t, settings.AnyoneCanEdit)
}

func TestChangeSettings_TurningAutoLockOnWhileATimerRunsSchedulesTheLock(t *testing.T) {
	// given
	svc, m := newTestService(t)
	timers := fakeTimersFor(svc.timers)
	svc.now = func() time.Time {
		return stamp
	}
	access := boardAccess(host)
	access.Header.TimerDurationMS = new(300000)
	access.Header.TimerEndsAt = new(stamp.Add(4 * time.Minute))
	m.entered(access)
	changed := access.Header.Board
	changed.TimerAutoLock = true
	m.boards.EXPECT().SetBoardSettings(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: changed, Version: 8}, nil)

	// when
	_, err := svc.ChangeSettings(t.Context(), settingsCommand(spec.BoardSettingsPatch{TimerAutoLock: new(true)}))

	// then
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{4 * time.Minute}, timers.live())
}

func TestChangeSettings_UnlockingABoardWhoseTimerHasEndedDoesNotLockItAgain(t *testing.T) {
	// given a board the timer locked when its time was up
	svc, m := newTestService(t)
	timers := fakeTimersFor(svc.timers)
	svc.now = func() time.Time {
		return stamp
	}
	access := boardAccess(host, autoLocking)
	access.Header.TimerDurationMS = new(300000)
	access.Header.TimerEndsAt = new(stamp.Add(-time.Minute))
	access.Header.LockedAt = new(stamp.Add(-time.Minute))
	m.entered(access)
	unlocked := access.Header.Board
	unlocked.LockedAt = nil
	m.boards.EXPECT().SetBoardSettings(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: unlocked, Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()

	// when
	_, err := svc.ChangeSettings(t.Context(), settingsCommand(spec.BoardSettingsPatch{BoardLocked: new(false)}))

	// then only a timer still running schedules a lock
	require.NoError(t, err)
	assert.Empty(t, timers.live())
	assert.NotContains(t, svc.timers.timers, boardID)
}

func TestChangeSettings_TurningAutoLockOffOrLockingTheBoardCancelsTheLock(t *testing.T) {
	tests := []struct {
		name   string
		patch  spec.BoardSettingsPatch
		change func(board *model.Board)
		reeval bool
	}{
		{name: "auto-lock off", patch: spec.BoardSettingsPatch{TimerAutoLock: new(false)}, change: func(board *model.Board) {
			board.TimerAutoLock = false
		}},
		{name: "board locked by hand", patch: spec.BoardSettingsPatch{BoardLocked: new(true)}, change: func(board *model.Board) {
			board.LockedAt = new(stamp)
		}, reeval: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a running timer whose lock is scheduled
			svc, m := newTestService(t)
			timers := fakeTimersFor(svc.timers)
			access := boardAccess(host, autoLocking)
			access.Header.TimerDurationMS = new(300000)
			access.Header.TimerEndsAt = new(stamp.Add(4 * time.Minute))
			svc.timers.Schedule(model.TimerDue{BoardID: boardID, OrgID: orgID, EndsAt: *access.Header.TimerEndsAt})
			m.entered(access)
			changed := access.Header.Board
			tc.change(&changed)
			m.boards.EXPECT().SetBoardSettings(mock.Anything, spec.BoardSettingsWrite{BoardWrite: target(), Patch: tc.patch}).Return(&model.Versioned[model.Board]{Value: changed, Version: 8}, nil)
			if tc.reeval {
				m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()
			}

			// when
			_, err := svc.ChangeSettings(t.Context(), settingsCommand(tc.patch))

			// then
			require.NoError(t, err)
			assert.Empty(t, timers.live())
			assert.NotContains(t, svc.timers.timers, boardID)
		})
	}
}

func TestChangeSettings_IsTheFacilitatorsAlone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.ChangeSettings(t.Context(), settingsCommand(spec.BoardSettingsPatch{VotingOpen: new(false)}))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestChangeSettings_RefusesValuesOutOfRangeBeforeTakingTheGate(t *testing.T) {
	tests := []struct {
		name  string
		patch spec.BoardSettingsPatch
		want  error
	}{
		{name: "a negative budget", patch: spec.BoardSettingsPatch{MaxVotes: new(-1)}, want: ErrInvalidMaxVotes},
		{name: "a hundred votes", patch: spec.BoardSettingsPatch{MaxVotes: new(100)}, want: ErrInvalidMaxVotes},
		{name: "a hundred votes per card", patch: spec.BoardSettingsPatch{MaxVotesPerCard: new(100)}, want: ErrInvalidMaxVotes},
		{name: "an unknown sort order", patch: spec.BoardSettingsPatch{SortMode: new(model.SortMode("random"))}, want: ErrInvalidSortMode},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.ChangeSettings(t.Context(), settingsCommand(tc.patch))

			// then the strict mocks prove the gate was never asked for
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}
