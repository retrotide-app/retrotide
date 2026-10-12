package board

import (
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

var (
	fourthCardID = uuid.MustParse("1a2b3c4d-0000-4000-8000-00000000000d")
)

func reverseShuffle(n int, swap func(i, j int)) {
	for i := range n / 2 {
		swap(i, n-1-i)
	}
}

func hiddenCard(id uuid.UUID, column uuid.UUID, author uuid.UUID, position string) model.Card {
	return model.Card{ID: id, BoardID: boardID, ColumnID: column, AuthorParticipantID: new(author), Body: secret, Position: position, CreatedAt: stamp}
}

func shownBoard(access *model.BoardAccess) model.Board {
	board := access.Header.Board
	board.CardsHidden = false

	return board
}

func TestRevealCards_ShowsEveryCardToEveryoneAtOnceAndRestoresEachViewersVotes(t *testing.T) {
	// given a named board in its writing phase
	svc, m := newTestService(t)
	access := boardAccess(host, hiddenCards)
	m.entered(access)
	card := hiddenCard(cardID, columnID, theirID, "a0")
	votes := []model.CardVote{{CardID: cardID, ParticipantID: myID, Count: 1, CardAuthorParticipantID: new(theirID)}}
	state := model.BoardState{Header: model.BoardHeader{Board: shownBoard(access)}, Cards: []model.Card{card}, Votes: votes}
	m.boards.EXPECT().RevealCards(mock.Anything, spec.CardsRevealWrite{BoardWrite: target()}).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.RevealCards(t.Context(), baseCommand())

	// then one event carries every card, identically for everyone and never marked as anyone's own
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	published := m.events.published[0]
	assert.False(t, published.access.Header.CardsHidden, "the roster learns the board is shown with the event")
	assert.Equal(t, projection.ClassOthers, published.event.ClassOf(projection.Viewer{ParticipantID: new(theirID)}), "the author is sent the same payload as everyone else")
	revealed, ok := published.event.Payload(projection.ClassOthers).(dto.CardsRevealedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventCardsRevealed, revealed.Type)
	require.Len(t, revealed.Cards, 1)
	assert.Equal(t, secret, revealed.Cards[0].Body)
	assert.False(t, revealed.Cards[0].IsMine)
	assert.False(t, revealed.Cards[0].Hidden)
	require.Len(t, m.events.everyoneMe, 1)
	assert.Equal(t, everyoneMe{board: state.Header.Board, votes: votes, version: 8}, m.events.everyoneMe[0])
}

func TestRevealCards_ShufflesEveryKeyOnAnAnonymousBoard(t *testing.T) {
	// given an anonymous board whose cards were typed in order
	svc, m := newTestService(t)
	svc.shuffle = reverseShuffle
	access := boardAccess(host, hiddenCards, anonymous)
	m.entered(access)
	typed := []model.Card{
		hiddenCard(cardID, columnID, myID, "a0"),
		hiddenCard(otherID, columnID, theirID, "a1"),
		hiddenCard(thirdCardID, columnID, myID, "a2"),
		hiddenCard(fourthCardID, secondColumnID, theirID, "a0"),
	}
	m.cards.EXPECT().List(mock.Anything, boardID).Return(typed, nil)
	keys := fracindex.Spread(3)
	shuffled := []spec.PositionChange{
		{ID: thirdCardID, Position: keys[0]},
		{ID: otherID, Position: keys[1]},
		{ID: cardID, Position: keys[2]},
		{ID: fourthCardID, Position: fracindex.Spread(1)[0]},
	}
	state := model.BoardState{Header: model.BoardHeader{Board: shownBoard(access)}}
	m.boards.EXPECT().RevealCards(mock.Anything, spec.CardsRevealWrite{BoardWrite: target(), Positions: shuffled}).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.RevealCards(t.Context(), baseCommand())

	// then every card gets a new key, in the shuffled order rather than the typed one
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestRevealCards_AShownBoardIsLeftAsItIs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))

	// when
	got, err := svc.RevealCards(t.Context(), baseCommand())

	// then the strict mocks prove nothing was written or announced
	require.NoError(t, err)
	assert.Equal(t, versionResponse(7), got)
	assert.Empty(t, m.events.published)
}

func TestRevealCards_IsTheFacilitatorsAlone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hiddenCards))

	// when
	got, err := svc.RevealCards(t.Context(), baseCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestHideCards_SendsEachViewerTheirOwnCounts(t *testing.T) {
	// given a shown board
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	hidden := access.Header.Board
	hidden.CardsHidden = true
	cards := []model.Card{hiddenCard(cardID, columnID, myID, "a0"), hiddenCard(otherID, columnID, theirID, "a1")}
	state := model.BoardState{Header: model.BoardHeader{Board: hidden}, Cards: cards}
	m.boards.EXPECT().HideCards(mock.Anything, target()).Return(&model.Versioned[model.BoardState]{Value: state, Version: 8}, nil)

	// when
	got, err := svc.HideCards(t.Context(), baseCommand())

	// then the event carries the tally the outbox projects for each viewer
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.True(t, m.events.published[0].access.Header.CardsHidden)
	tally, hides := m.events.published[0].event.Hides()
	require.True(t, hides)
	assert.Equal(t, projection.Tally(hidden, cards), tally)
	assert.Equal(t, map[uuid.UUID]int{columnID: 1}, tally.For(hidden, projection.Viewer{ParticipantID: new(myID)}))
}

func TestHideCards_AHiddenBoardIsLeftAsItIs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards))

	// when
	got, err := svc.HideCards(t.Context(), baseCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(7), got)
	assert.Empty(t, m.events.published)
}

func TestHideCards_IsRefusedOnceTheBoardHasLeftItsWriteStage(t *testing.T) {
	for _, stage := range []model.Stage{model.StageVote, model.StageDiscuss, model.StageActions} {
		t.Run(string(stage), func(t *testing.T) {
			// given a board whose cards were revealed when it moved on
			svc, m := newTestService(t)
			m.entered(boardAccess(host, atStage(stage)))

			// when
			got, err := svc.HideCards(t.Context(), baseCommand())

			// then the strict board mock proves nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, policy.ErrHideOutOfStage)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestHideCards_IsRefusedOnAClosedBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	access.Header.ClosedAt = new(stamp)
	m.entered(access)

	// when
	got, err := svc.HideCards(t.Context(), baseCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrReadOnly)
}
