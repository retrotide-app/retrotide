package board

import (
	"fmt"
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func stageCommand(stage model.Stage) StageCommand {
	return StageCommand{Command: baseCommand(), Stage: stage}
}

func movedBoard(access *model.BoardAccess, stage model.Stage) model.Board {
	board := access.Header.Board
	board.Stage = stage
	board.CardsHidden = board.CardsHidden && stage == model.StageWrite

	return board
}

func TestChangeStage_EnteringVoteRevealsEveryCardWithTheShuffleInOneEvent(t *testing.T) {
	// given an anonymous board in its Write stage, whose cards were typed in order
	svc, m := newTestService(t)
	svc.shuffle = reverseShuffle
	access := boardAccess(host, hiddenCards, anonymous)
	m.entered(access)
	typed := []model.Card{hiddenCard(cardID, columnID, myID, "a0"), hiddenCard(otherID, columnID, theirID, "a1")}
	m.cards.EXPECT().List(mock.Anything, boardID).Return(typed, nil)
	keys := fracindex.Spread(2)
	write := spec.BoardStageWrite{
		BoardWrite: target(),
		From:       model.StageWrite,
		To:         model.StageVote,
		Positions:  []spec.PositionChange{{ID: otherID, Position: keys[0]}, {ID: cardID, Position: keys[1]}},
		Reveal:     true,
	}
	votes := []model.CardVote{{CardID: otherID, ParticipantID: myID, Count: 1, CardAuthorParticipantID: new(theirID)}}
	state := model.BoardState{Header: model.BoardHeader{Board: movedBoard(access, model.StageVote)}, Cards: typed, Votes: votes}
	m.boards.EXPECT().AdvanceStage(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageVote))

	// then the reveal and the new stage are one event at one version, and every seated viewer gets their votes back
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	published := m.events.published[0]
	assert.False(t, published.access.Header.CardsHidden, "the roster learns the board is shown with the event")
	assert.Equal(t, model.StageVote, published.access.Header.Stage)
	revealed, ok := published.event.Payload(projection.ClassOthers).(dto.CardsRevealedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventCardsRevealed, revealed.Type)
	assert.Equal(t, model.StageVote, revealed.Stage)
	require.Len(t, revealed.Cards, 2)
	require.Len(t, m.events.everyoneMe, 1)
	assert.Equal(t, int64(8), m.events.everyoneMe[0].version)
	assert.Equal(t, votes, m.events.everyoneMe[0].votes)
}

func TestChangeStage_EnteringVoteOnABoardThatShowsCardsAnnouncesOnlyTheStage(t *testing.T) {
	// given a named board whose cards were shown while people wrote
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	write := spec.BoardStageWrite{BoardWrite: target(), From: model.StageWrite, To: model.StageVote}
	state := model.BoardState{Header: model.BoardHeader{Board: movedBoard(access, model.StageVote)}}
	m.boards.EXPECT().AdvanceStage(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageVote))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.StageChangedEvent{Type: projection.EventStageChanged, Version: 8, Stage: model.StageVote}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.Empty(t, m.events.everyoneMe)
}

func TestChangeStage_EachLaterStepAnnouncesTheNewStageToEveryone(t *testing.T) {
	tests := []struct {
		from model.Stage
		to   model.Stage
	}{
		{from: model.StageVote, to: model.StageDiscuss},
		{from: model.StageDiscuss, to: model.StageActions},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s to %s", tc.from, tc.to), func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			access := boardAccess(host, atStage(tc.from))
			m.entered(access)
			write := spec.BoardStageWrite{BoardWrite: target(), From: tc.from, To: tc.to}
			state := model.BoardState{Header: model.BoardHeader{Board: movedBoard(access, tc.to)}}
			m.boards.EXPECT().AdvanceStage(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

			// when
			got, err := svc.ChangeStage(t.Context(), stageCommand(tc.to))

			// then
			require.NoError(t, err)
			assert.Equal(t, versionResponse(8), got)
			require.Len(t, m.events.published, 1)
			event := m.events.published[0].event
			assert.Equal(t, dto.StageChangedEvent{Type: projection.EventStageChanged, Version: 8, Stage: tc.to}, event.Payload(projection.ClassOthers))
			assert.Equal(t, projection.ClassOthers, event.ClassOf(projection.Viewer{ParticipantID: new(theirID)}))
		})
	}
}

func TestChangeStage_EnteringDiscussWithHiddenTotalsMakesEveryoneRefetch(t *testing.T) {
	// given totals hidden while people voted
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageVote), func(access *model.BoardAccess) {
		access.Header.VoteCountsHidden = true
	})
	m.entered(access)
	state := model.BoardState{Header: model.BoardHeader{Board: movedBoard(access, model.StageDiscuss)}}
	m.boards.EXPECT().AdvanceStage(mock.Anything, spec.BoardStageWrite{BoardWrite: target(), From: model.StageVote, To: model.StageDiscuss}).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageDiscuss))

	// then the totals come back only through a snapshot, which also carries the new stage
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.False(t, m.events.published[0].access.Header.TotalsHidden())
}

func TestChangeStage_MovingOnToEndRunsTheClose(t *testing.T) {
	// given an anonymous board in its Actions stage
	svc, m := newTestService(t)
	access := boardAccess(host, anonymous, atStage(model.StageActions))
	m.entered(access)
	m.boards.EXPECT().CloseBoard(mock.Anything, spec.BoardCloseWrite{BoardWrite: target(), Sever: true}).Return(&model.Versioned[model.Board]{Value: closedBoard(access), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, closeAudit()).Return(nil).Once()

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageEnd))

	// then the retro ends exactly as End retro ends it, severing included
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.Equal(t, model.StageEnd, m.events.published[0].access.Header.Stage)
}

func TestChangeStage_AnEndWhoseAuditFailedStillAnswersItsVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageActions))
	m.entered(access)
	m.boards.EXPECT().CloseBoard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: closedBoard(access), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, closeAudit()).Return(errDatabase).Once()

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageEnd))

	// then
	assert.Equal(t, versionResponse(8), got)
	require.ErrorIs(t, err, ErrNotAudited)
}

func TestChangeStage_TheStageTheBoardIsAlreadyInAnswersItsVersionAndWritesNothing(t *testing.T) {
	// given a click that arrived after someone else moved the board on
	svc, m := newTestService(t)
	m.entered(boardAccess(host, atStage(model.StageDiscuss)))

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageDiscuss))

	// then the strict board mock proves nothing was written or announced
	require.NoError(t, err)
	assert.Equal(t, versionResponse(7), got)
	assert.Empty(t, m.events.published)
}

func TestChangeStage_GoingBackOneStepAnnouncesTheEarlierStageAndRehidesNothing(t *testing.T) {
	tests := []struct {
		from model.Stage
		to   model.Stage
	}{
		{from: model.StageVote, to: model.StageWrite},
		{from: model.StageDiscuss, to: model.StageVote},
		{from: model.StageActions, to: model.StageDiscuss},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s back to %s", tc.from, tc.to), func(t *testing.T) {
			// given a board whose cards were revealed on the way
			svc, m := newTestService(t)
			access := boardAccess(host, atStage(tc.from))
			m.entered(access)
			write := spec.BoardStageWrite{BoardWrite: target(), From: tc.from, To: tc.to}
			state := model.BoardState{Header: model.BoardHeader{Board: movedBoard(access, tc.to)}}
			m.boards.EXPECT().AdvanceStage(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

			// when
			got, err := svc.ChangeStage(t.Context(), stageCommand(tc.to))

			// then the write asks for no reveal, and the cards stay shown
			require.NoError(t, err)
			assert.Equal(t, versionResponse(8), got)
			require.Len(t, m.events.published, 1)
			assert.False(t, m.events.published[0].access.Header.CardsHidden)
			assert.Equal(t, dto.StageChangedEvent{Type: projection.EventStageChanged, Version: 8, Stage: tc.to}, m.events.published[0].event.Payload(projection.ClassOthers))
			assert.Empty(t, m.events.everyoneMe)
		})
	}
}

func TestChangeStage_GoingBackToVoteWithHiddenTotalsMakesEveryoneRefetch(t *testing.T) {
	// given totals were shown in Discuss and the setting still hides them while people vote
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageDiscuss), func(access *model.BoardAccess) {
		access.Header.VoteCountsHidden = true
	})
	m.entered(access)
	state := model.BoardState{Header: model.BoardHeader{Board: movedBoard(access, model.StageVote)}}
	m.boards.EXPECT().AdvanceStage(mock.Anything, spec.BoardStageWrite{BoardWrite: target(), From: model.StageDiscuss, To: model.StageVote}).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageVote))

	// then the totals go back into hiding through a snapshot
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.True(t, m.events.published[0].access.Header.TotalsHidden())
}

func TestChangeStage_RefusesAnyStepButTheNextOrThePreviousOne(t *testing.T) {
	tests := []struct {
		name string
		from model.Stage
		to   model.Stage
	}{
		{name: "skipping Vote", from: model.StageWrite, to: model.StageDiscuss},
		{name: "skipping to the end", from: model.StageVote, to: model.StageEnd},
		{name: "going back two stages", from: model.StageDiscuss, to: model.StageWrite},
		{name: "going back to Write from Actions", from: model.StageActions, to: model.StageWrite},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(host, atStage(tc.from)))

			// when
			got, err := svc.ChangeStage(t.Context(), stageCommand(tc.to))

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, ErrStageNotNext)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestChangeStage_RefusesAnUnknownStageBeforeTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand("review"))

	// then the strict mocks prove the gate was never asked for
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrInvalidStage)
}

func TestChangeStage_IsRefusedToAParticipantAndOnAClosedBoard(t *testing.T) {
	closed := func(access *model.BoardAccess) {
		access.Header.ClosedAt = new(stamp)
		access.Header.Stage = model.StageEnd
	}

	tests := []struct {
		name   string
		access *model.BoardAccess
		want   error
	}{
		{name: "a participant", access: boardAccess(writer), want: policy.ErrNotAllowed},
		{name: "an ended retro", access: boardAccess(host, closed), want: policy.ErrReadOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)

			// when
			got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageVote))

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestChangeStage_AStageThatMovedUnderTheWriteIsRefusedAsNotNext(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, atStage(model.StageVote)))
	m.boards.EXPECT().AdvanceStage(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("move the board on: %w", dao.ErrStageMoved))

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageDiscuss))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrStageNotNext)
	assert.Empty(t, m.events.published)
}

func TestChangeStage_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, atStage(model.StageVote)))
	m.boards.EXPECT().AdvanceStage(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageDiscuss))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, errDatabase)
	assert.Empty(t, m.events.published)
}

func TestChangeStage_ARetriedStepReplaysItsVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host, atStage(model.StageDiscuss)), 6)

	// when
	got, err := svc.ChangeStage(t.Context(), stageCommand(model.StageDiscuss))

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(6, nil), got)
}
