package board

import (
	"slices"
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func facilitatedColumnCommand() ColumnCommand {
	return ColumnCommand{Command: baseCommand(), ColumnID: columnID}
}

func columnFacilitationState(access *model.BoardAccess, change func(board *model.Board)) model.BoardState {
	board := access.Header.Board
	change(&board)

	return model.BoardState{Header: model.BoardHeader{Board: board}}
}

func spotlitBoard(spotlight model.BoardSpotlight) accessOption {
	return func(access *model.BoardAccess) {
		access.Header.Spotlight = spotlight
	}
}

func firstColumnRevealed(access *model.BoardAccess) {
	access.Header.RevealedColumnIDs = []uuid.UUID{columnID}
}

func TestRevealColumn_ShowsThatColumnsCardsToEveryoneAtOneVersionAndRestoresEachViewersVotes(t *testing.T) {
	// given a named board in its writing phase, whose cards are hidden
	svc, m := newTestService(t)
	access := boardAccess(host, hiddenCards)
	m.entered(access)
	m.column()
	card := hiddenCard(cardID, columnID, theirID, "a0")
	votes := []model.CardVote{{CardID: cardID, ParticipantID: myID, Count: 1, CardAuthorParticipantID: new(theirID), CardColumnID: columnID}}
	state := columnFacilitationState(access, func(board *model.Board) {
		board.RevealedColumnIDs = []uuid.UUID{columnID}
	})
	state.Cards = []model.Card{card, hiddenCard(otherID, secondColumnID, theirID, "a0")}
	state.Votes = votes
	write := spec.BoardFacilitationWrite{
		BoardWrite:        target(),
		RevealedColumnIDs: []uuid.UUID{columnID},
		RevealedColumnID:  new(columnID),
	}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.RevealColumn(t.Context(), facilitatedColumnCommand())

	// then one event at the write's version carries only that column's cards, never marked as anyone's own
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	published := m.onlyEvent(t)
	assert.Equal(t, int64(8), published.Version())
	assert.Equal(t, []uuid.UUID{columnID}, m.events.published[0].access.Header.RevealedColumnIDs, "the roster learns the column is revealed with the event")
	revealed, ok := published.Payload(projection.ClassOthers).(dto.ColumnRevealedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventColumnRevealed, revealed.Type)
	assert.Equal(t, columnID, revealed.ColumnID)
	require.Len(t, revealed.Cards, 1)
	assert.Equal(t, cardID, revealed.Cards[0].ID)
	assert.Equal(t, secret, revealed.Cards[0].Body)
	assert.False(t, revealed.Cards[0].IsMine)
	require.Len(t, m.events.everyoneMe, 1)
	assert.Equal(t, everyoneMe{board: state.Header.Board, votes: votes, version: 8}, m.events.everyoneMe[0])
}

func TestRevealColumn_ShufflesOnlyThatColumnsKeysOnAnAnonymousBoard(t *testing.T) {
	// given an anonymous board whose column holds cards typed in order
	svc, m := newTestService(t)
	svc.shuffle = reverseShuffle
	access := boardAccess(host, hiddenCards, anonymous)
	m.entered(access)
	m.column()
	typed := []model.Card{
		hiddenCard(cardID, columnID, myID, "a0"),
		hiddenCard(otherID, columnID, theirID, "a1"),
		hiddenCard(thirdCardID, columnID, myID, "a2"),
	}
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(typed, nil)
	keys := fracindex.Spread(3)
	shuffled := []spec.PositionChange{
		{ID: thirdCardID, Position: keys[0]},
		{ID: otherID, Position: keys[1]},
		{ID: cardID, Position: keys[2]},
	}
	state := columnFacilitationState(access, func(board *model.Board) {
		board.RevealedColumnIDs = []uuid.UUID{columnID}
	})
	write := spec.BoardFacilitationWrite{
		BoardWrite:        target(),
		RevealedColumnIDs: []uuid.UUID{columnID},
		RevealedColumnID:  new(columnID),
		Positions:         shuffled,
	}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.RevealColumn(t.Context(), facilitatedColumnCommand())

	// then the strict card mock proves no other column's cards were read or moved
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestRevealColumn_KeepsTheColumnsAlreadyRevealed(t *testing.T) {
	// given the second column revealed earlier
	svc, m := newTestService(t)
	access := boardAccess(host, hiddenCards, secondColumnRevealed)
	m.entered(access)
	m.column()
	state := columnFacilitationState(access, func(board *model.Board) {
		board.RevealedColumnIDs = []uuid.UUID{secondColumnID, columnID}
	})
	m.boards.EXPECT().SetFacilitationState(mock.Anything, mock.MatchedBy(func(s spec.BoardFacilitationWrite) bool {
		return slices.Equal(s.RevealedColumnIDs, []uuid.UUID{secondColumnID, columnID})
	})).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	_, err := svc.RevealColumn(t.Context(), facilitatedColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{secondColumnID}, access.Header.RevealedColumnIDs, "the resolved access is never mutated")
}

func TestRevealColumn_AColumnAlreadyShownIsLeftAsItIs(t *testing.T) {
	tests := []struct {
		name    string
		options []accessOption
	}{
		{name: "the board shows its cards", options: nil},
		{name: "the column is already revealed", options: []accessOption{hiddenCards, firstColumnRevealed}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(host, tc.options...))
			m.column()

			// when
			got, err := svc.RevealColumn(t.Context(), facilitatedColumnCommand())

			// then the strict mocks prove nothing was written or announced
			require.NoError(t, err)
			assert.Equal(t, versionResponse(7), got)
			assert.Empty(t, m.events.published)
			assert.Empty(t, m.events.everyoneMe)
		})
	}
}

func TestRevealColumn_AHiddenColumnCannotBeRevealed(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards, columnHidden))
	m.column()

	// when
	got, err := svc.RevealColumn(t.Context(), facilitatedColumnCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrColumnHidden)
	assert.Empty(t, m.events.published)
}

func TestColumnVisibility_IsTheFacilitatorsAlone(t *testing.T) {
	commands := map[string]func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error){
		"reveal": func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error) {
			return svc.RevealColumn(t.Context(), facilitatedColumnCommand())
		},
		"hide": func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error) {
			return svc.HideColumn(t.Context(), facilitatedColumnCommand())
		},
		"show": func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error) {
			return svc.ShowColumn(t.Context(), facilitatedColumnCommand())
		},
	}

	for name, run := range commands {
		t.Run(name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, hiddenCards))

			// when
			got, err := run(t, svc)

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, policy.ErrNotAllowed)
		})
	}
}

func TestRevealColumn_AnUnknownColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(nil, errNotFound).Once()

	// when
	got, err := svc.RevealColumn(t.Context(), facilitatedColumnCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrColumnNotFound)
}

func TestRevealColumn_ARetriedRevealReplaysItsVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host, hiddenCards), 6)

	// when
	got, err := svc.RevealColumn(t.Context(), facilitatedColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(6, nil), got)
}

func TestHideColumn_HidesTheColumnAndClearsEverySpotlightReferenceIntoIt(t *testing.T) {
	// given a presented card in the column, a highlighted card in another column and the column in focus
	svc, m := newTestService(t)
	access := boardAccess(host, spotlitBoard(model.BoardSpotlight{PresentingCardID: new(cardID), HighlightedCardID: new(otherID), FocusedColumnID: new(columnID)}))
	m.entered(access)
	m.column()
	m.stored(card(theirID))
	m.stored(model.Card{ID: otherID, BoardID: boardID, ColumnID: secondColumnID, AuthorParticipantID: new(theirID), Body: secret, Position: "V", CreatedAt: stamp})
	kept := model.BoardSpotlight{HighlightedCardID: new(otherID)}
	state := columnFacilitationState(access, func(board *model.Board) {
		board.HiddenColumnIDs = []uuid.UUID{columnID}
		board.Spotlight = kept
	})
	write := spec.BoardFacilitationWrite{BoardWrite: target(), HiddenColumnIDs: []uuid.UUID{columnID}, Spotlight: kept}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.HideColumn(t.Context(), facilitatedColumnCommand())

	// then everyone is told the column is hidden, and the cleared spotlight follows at the same version
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 2)
	assert.True(t, slices.Contains(m.events.published[0].access.Header.HiddenColumnIDs, columnID), "the roster learns the column is hidden with the event")
	hidden := m.events.published[0].event
	assert.Equal(t, dto.ColumnHiddenEvent{BoardEventHead: dto.BoardEventHead{Type: projection.EventColumnHidden, Version: 8}, ColumnID: columnID}, hidden.Payload(projection.ClassOthers))
	assert.Equal(t, projection.ClassOthers, hidden.ClassOf(otherViewer), "a participant hears of it too, and drops the column")
	spotlight := m.events.published[1].event
	assert.Equal(t, int64(8), spotlight.Version())
	assert.Equal(t, dto.SpotlightChangedEvent{
		BoardEventHead: dto.BoardEventHead{Type: projection.EventSpotlightChanged, Version: 8},
		Spotlight:      dto.SpotlightView{HighlightedCardID: new(otherID)},
	}, spotlight.Payload(projection.ClassOthers))
}

func TestHideColumn_ASpotlitCardSinceDeletedCountsAsGone(t *testing.T) {
	// given a presented card that no longer exists
	svc, m := newTestService(t)
	access := boardAccess(host, spotlitBoard(model.BoardSpotlight{PresentingCardID: new(cardID)}))
	m.entered(access)
	m.column()
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errNotFound).Once()
	state := columnFacilitationState(access, func(board *model.Board) {
		board.HiddenColumnIDs = []uuid.UUID{columnID}
		board.Spotlight = model.BoardSpotlight{}
	})
	write := spec.BoardFacilitationWrite{BoardWrite: target(), HiddenColumnIDs: []uuid.UUID{columnID}}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	_, err := svc.HideColumn(t.Context(), facilitatedColumnCommand())

	// then
	require.NoError(t, err)
	require.Len(t, m.events.published, 2)
}

func TestHideColumn_ASpotlightOutsideTheColumnIsLeftAlone(t *testing.T) {
	// given the focus on another column
	svc, m := newTestService(t)
	access := boardAccess(host, spotlitBoard(model.BoardSpotlight{FocusedColumnID: new(secondColumnID)}))
	m.entered(access)
	m.column()
	state := columnFacilitationState(access, func(board *model.Board) {
		board.HiddenColumnIDs = []uuid.UUID{columnID}
	})
	write := spec.BoardFacilitationWrite{BoardWrite: target(), HiddenColumnIDs: []uuid.UUID{columnID}, Spotlight: model.BoardSpotlight{FocusedColumnID: new(secondColumnID)}}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	_, err := svc.HideColumn(t.Context(), facilitatedColumnCommand())

	// then only the column's own event goes out
	require.NoError(t, err)
	event := m.onlyEvent(t)
	_, ok := event.Payload(projection.ClassOthers).(dto.ColumnHiddenEvent)
	assert.True(t, ok)
}

func TestHideColumn_AColumnAlreadyHiddenIsLeftAsItIs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, columnHidden))
	m.column()

	// when
	got, err := svc.HideColumn(t.Context(), facilitatedColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(7), got)
	assert.Empty(t, m.events.published)
}

func TestShowColumn_ShowsTheColumnAndResyncsEveryone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, columnHidden)
	m.entered(access)
	m.column()
	state := columnFacilitationState(access, func(board *model.Board) {
		board.HiddenColumnIDs = []uuid.UUID{}
	})
	write := spec.BoardFacilitationWrite{BoardWrite: target(), HiddenColumnIDs: []uuid.UUID{}}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, write).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.ShowColumn(t.Context(), facilitatedColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	event := m.onlyEvent(t)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, event.Payload(projection.ClassOthers))
	assert.False(t, slices.Contains(m.events.published[0].access.Header.HiddenColumnIDs, columnID))
}

func TestShowColumn_AColumnNotHiddenIsLeftAsItIs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.column()

	// when
	got, err := svc.ShowColumn(t.Context(), facilitatedColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(7), got)
	assert.Empty(t, m.events.published)
}
