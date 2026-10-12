package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	boardParticipant = uuid.MustParse("6e7f8091-a2b3-4c4d-a5e6-4c5d6e7f8091")
	participantRoute = "/participants/" + boardParticipant.String()
)

func TestBoardFacilitationController_EveryRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: boardPath("/reveal"), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath("/hide"), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath("/timer"), Body: dto.TimerRequest{BoardCommandRequest: boardOpBody, Action: model.TimerActionStart, DurationMS: 300000}},
		{Method: http.MethodPost, Path: boardPath("/stage"), Body: dto.StageRequest{BoardCommandRequest: boardOpBody, Stage: model.StageVote}},
		{Method: http.MethodPost, Path: boardPath("/close"), Body: boardOpBody},
		{Method: http.MethodPut, Path: boardPath(participantRoute + "/role"), Body: dto.ParticipantRoleRequest{BoardCommandRequest: boardOpBody, Role: model.BoardRoleFacilitator}},
		{Method: http.MethodDelete, Path: boardPath(participantRoute), Body: boardOpBody},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newBoardHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardFacilitationController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	tests := []boardCommandCase{
		{
			name:   "reveal the cards",
			method: http.MethodPost,
			path:   boardPath("/reveal"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RevealCards(mock.Anything, boardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "hide the cards",
			method: http.MethodPost,
			path:   boardPath("/hide"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().HideCards(mock.Anything, boardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "start the timer",
			method: http.MethodPost,
			path:   boardPath("/timer"),
			body:   dto.TimerRequest{BoardCommandRequest: boardOpBody, Action: model.TimerActionStart, DurationMS: 300000},
			expect: func(bh *boardHarness) {
				timer := board.TimerCommand{Command: boardCommandOf, Action: model.TimerActionStart, DurationMS: 300000}
				bh.boards.EXPECT().ControlTimer(mock.Anything, timer).Return(boardVersioned, nil)
			},
		},
		{
			name:   "add a minute to the timer",
			method: http.MethodPost,
			path:   boardPath("/timer"),
			body:   dto.TimerRequest{BoardCommandRequest: boardOpBody, Action: model.TimerActionAddMinute},
			expect: func(bh *boardHarness) {
				timer := board.TimerCommand{Command: boardCommandOf, Action: model.TimerActionAddMinute}
				bh.boards.EXPECT().ControlTimer(mock.Anything, timer).Return(boardVersioned, nil)
			},
		},
		{
			name:   "move on to the next stage",
			method: http.MethodPost,
			path:   boardPath("/stage"),
			body:   dto.StageRequest{BoardCommandRequest: boardOpBody, Stage: model.StageDiscuss},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ChangeStage(mock.Anything, board.StageCommand{Command: boardCommandOf, Stage: model.StageDiscuss}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "end the retro",
			method: http.MethodPost,
			path:   boardPath("/close"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Close(mock.Anything, boardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "promote a participant",
			method: http.MethodPut,
			path:   boardPath(participantRoute + "/role"),
			body:   dto.ParticipantRoleRequest{BoardCommandRequest: boardOpBody, Role: model.BoardRoleFacilitator},
			expect: func(bh *boardHarness) {
				assign := board.ParticipantRoleCommand{Command: boardCommandOf, ParticipantID: boardParticipant, Role: model.BoardRoleFacilitator}
				bh.boards.EXPECT().ChangeParticipantRole(mock.Anything, assign).Return(boardVersioned, nil)
			},
		},
		{
			name:   "remove a participant",
			method: http.MethodDelete,
			path:   boardPath(participantRoute),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RemoveParticipant(mock.Anything, board.ParticipantCommand{Command: boardCommandOf, ParticipantID: boardParticipant}).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardFacilitationController_SettingsCarryTheLocksAndTheTimerSound(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	request := dto.BoardSettingsRequest{BoardCommandRequest: boardOpBody, CardsLocked: new(true), MovesLocked: new(false), BoardLocked: new(true), TimerPlaySound: new(false)}
	bh.boards.EXPECT().ChangeSettings(mock.Anything, mock.MatchedBy(func(command board.SettingsCommand) bool {
		patch := command.Settings

		return command.Command == boardCommandOf && *patch.CardsLocked && !*patch.MovesLocked && *patch.BoardLocked && !*patch.TimerPlaySound && patch.VotingOpen == nil
	})).Return(boardVersioned, nil)

	// when
	status, body := bh.record(http.MethodPut, boardPath("/settings"), request)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":7}`, body)
}

func TestBoardFacilitationController_AParticipantPathIDThatIsNotAUUIDIsRefused(t *testing.T) {
	// given
	bh := newBoardHarness(t)

	// when
	status, body := bh.record(http.MethodDelete, boardPath("/participants/not-a-uuid"), boardOpBody)

	// then the strict board mock proves the service is never asked
	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"invalid participantID"}`, body)
}

func TestBoardFacilitationController_AnEndedRetroWhoseAuditFailedStillAnswersItsVersion(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	bh.boards.EXPECT().Close(mock.Anything, boardCommandOf).Return(&dto.BoardCommandResponse{Version: 8}, fmt.Errorf("%w: audit down", board.ErrNotAudited))

	// when
	status, body := bh.record(http.MethodPost, boardPath("/close"), boardOpBody)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":8}`, body)
}

func TestBoardFacilitationController_AStageMoveThatEndedTheRetroWhoseAuditFailedStillAnswersItsVersion(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	command := board.StageCommand{Command: boardCommandOf, Stage: model.StageEnd}
	bh.boards.EXPECT().ChangeStage(mock.Anything, command).Return(&dto.BoardCommandResponse{Version: 8}, fmt.Errorf("%w: audit down", board.ErrNotAudited))

	// when
	status, body := bh.record(http.MethodPost, boardPath("/stage"), dto.StageRequest{BoardCommandRequest: boardOpBody, Stage: model.StageEnd})

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":8}`, body)
}

func TestBoardFacilitationController_MapsEveryStageRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: board.ErrInvalidStage, status: http.StatusBadRequest, body: `{"error":"a board stage must be write, vote, discuss, actions or end","code":"invalid_stage"}`},
		{err: board.ErrStageNotNext, status: http.StatusConflict, body: `{"error":"a board moves one stage at a time, so it can only go to the stage straight after its current one or back to the one before it","code":"stage_not_next"}`},
		{err: policy.ErrCardsOutOfStage, status: http.StatusConflict, body: `{"error":"cards can be added, edited, moved and deleted only in the Write stage","code":"wrong_stage"}`},
		{err: policy.ErrVotesOutOfStage, status: http.StatusConflict, body: `{"error":"votes can be cast or taken back only in the Vote stage","code":"wrong_stage"}`},
		{err: policy.ErrCommentsOutOfStage, status: http.StatusConflict, body: `{"error":"comments can be written, edited and deleted only in the Discuss stage","code":"wrong_stage"}`},
		{err: policy.ErrMergesOutOfStage, status: http.StatusConflict, body: `{"error":"cards can be merged or unmerged only in the Discuss stage","code":"wrong_stage"}`},
		{err: policy.ErrHideOutOfStage, status: http.StatusConflict, body: `{"error":"cards can be hidden only in the Write stage","code":"wrong_stage"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			bh.boards.EXPECT().ChangeStage(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("move the board on: %w", tc.err))

			// when
			status, body := bh.record(http.MethodPost, boardPath("/stage"), dto.StageRequest{BoardCommandRequest: boardOpBody, Stage: model.StageVote})

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardFacilitationController_MapsEveryFacilitationErrorToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: board.ErrInvalidTimerAction, status: http.StatusBadRequest, body: `{"error":"the timer action must be start, pause, resume, add_minute or stop","code":"invalid_timer_action"}`},
		{err: board.ErrInvalidTimerDuration, status: http.StatusBadRequest, body: `{"error":"a timer runs for between 1 second and 60 minutes","code":"invalid_timer_duration"}`},
		{err: board.ErrTimerTooLong, status: http.StatusConflict, body: `{"error":"a timer can run for at most 60 minutes","code":"timer_too_long"}`},
		{err: board.ErrTimerNotRunning, status: http.StatusConflict, body: `{"error":"the timer is not running","code":"timer_not_running"}`},
		{err: board.ErrTimerNotPaused, status: http.StatusConflict, body: `{"error":"the timer is not paused","code":"timer_not_paused"}`},
		{err: board.ErrInvalidRole, status: http.StatusBadRequest, body: `{"error":"a board role must be facilitator or participant","code":"invalid_role"}`},
		{err: board.ErrParticipantNotFound, status: http.StatusNotFound, body: `{"error":"participant not found"}`},
		{err: board.ErrCannotPromote, status: http.StatusConflict, body: `{"error":"only a member of the board's team or organisation can co-facilitate","code":"cannot_promote"}`},
		{err: board.ErrLeadFacilitates, status: http.StatusConflict, body: `{"error":"team leads and organisation admins always facilitate their boards","code":"lead_facilitates"}`},
		{err: board.ErrRemoveSelf, status: http.StatusConflict, body: `{"error":"you cannot remove yourself from the board","code":"remove_self"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			bh.boards.EXPECT().ControlTimer(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("control the timer: %w", tc.err))

			// when
			status, body := bh.record(http.MethodPost, boardPath("/timer"), dto.TimerRequest{BoardCommandRequest: boardOpBody, Action: model.TimerActionPause})

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardFacilitationController_AnUnexpectedFailureAnswersTheRoutesOwnMessage(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		expect func(bh *boardHarness)
		want   string
	}{
		{
			name:   "reveal",
			method: http.MethodPost,
			path:   boardPath("/reveal"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RevealCards(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to reveal the cards"}`,
		},
		{
			name:   "hide",
			method: http.MethodPost,
			path:   boardPath("/hide"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().HideCards(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to hide the cards"}`,
		},
		{
			name:   "timer",
			method: http.MethodPost,
			path:   boardPath("/timer"),
			body:   dto.TimerRequest{BoardCommandRequest: boardOpBody, Action: model.TimerActionStop},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ControlTimer(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to change the timer"}`,
		},
		{
			name:   "stage",
			method: http.MethodPost,
			path:   boardPath("/stage"),
			body:   dto.StageRequest{BoardCommandRequest: boardOpBody, Stage: model.StageVote},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ChangeStage(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to move the board to its next stage"}`,
		},
		{
			name:   "close",
			method: http.MethodPost,
			path:   boardPath("/close"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Close(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to end the retro"}`,
		},
		{
			name:   "role",
			method: http.MethodPut,
			path:   boardPath(participantRoute + "/role"),
			body:   dto.ParticipantRoleRequest{BoardCommandRequest: boardOpBody, Role: model.BoardRoleParticipant},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ChangeParticipantRole(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to change the participant's role"}`,
		},
		{
			name:   "removal",
			method: http.MethodDelete,
			path:   boardPath(participantRoute),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RemoveParticipant(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to remove the participant"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.JSONEq(t, tc.want, body)
		})
	}
}
