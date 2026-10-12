package board

import (
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func stackedUnder(id uuid.UUID, author uuid.UUID, position string) model.Card {
	child := hiddenCard(id, columnID, author, position)
	child.ParentCardID = new(cardID)

	return child
}

func TestMoveCard_AHiddenStackCarriesItsChildrenIntoTheTalliesAndToTheirAuthors(t *testing.T) {
	// given a stack merged before the facilitator hid the cards again, with a child of someone else's under my card
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hiddenCards, anonymous))
	top := card(myID)
	theirChild := stackedUnder(firstCardID, theirID, "W")
	myChild := stackedUnder(secondCardID, myID, "X")
	unrelated := hiddenCard(thirdCardID, columnID, theirID, "Y")
	m.stored(top)
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: secondColumnID}).Return(&model.BoardColumn{ID: secondColumnID, BoardID: boardID}, nil).Once()
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: secondColumnID}).Return([]model.Card{}, nil).Once()
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return([]model.Card{top, theirChild, myChild, unrelated}, nil).Once()
	moved := top
	moved.ColumnID = secondColumnID
	moved.Position = "V"
	m.cards.EXPECT().MoveCard(mock.Anything, mock.MatchedBy(func(placing spec.CardPlacement) bool {
		return placing.BoardWrite == target() && placing.CardID == cardID && placing.ColumnID == secondColumnID
	})).Return(&model.Versioned[model.Card]{Value: moved, Version: 8}, nil)

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: secondColumnID})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.Len(t, m.events.published, 3, "the move, then one event for each stacked card")

	tally := projection.Tally(boardAccess(writer, hiddenCards, anonymous).Header.Board, []model.Card{top, theirChild, myChild, unrelated})
	for _, published := range m.events.published {
		tally = published.event.MoveHiddenCards(tally)
	}
	assert.Equal(t, projection.HiddenTally{
		columnID:       {theirID: 1},
		secondColumnID: {myID: 2, theirID: 1},
	}, tally, "every card of the stack is counted in its new column")

	childEvent := m.events.published[1].event
	assert.Equal(t, projection.ClassNone, childEvent.ClassOf(projection.Viewer{ParticipantID: new(otherID)}), "nobody else hears of a hidden card")
	assert.Equal(t, projection.ClassNone, childEvent.ClassOf(projection.Viewer{ParticipantID: new(theirID)}), "on an anonymous board the child's author is not told the instant someone else acts")
	deferredID, deferred := childEvent.DeferredCard()
	require.True(t, deferred, "the child's author hears where it went on the next tick")
	assert.Equal(t, firstCardID, deferredID)
}

func TestMoveCard_AShownStackNeedsNoExtraEvents(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	top := card(myID)
	m.stored(top)
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: secondColumnID}).Return(&model.BoardColumn{ID: secondColumnID, BoardID: boardID}, nil).Once()
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: secondColumnID}).Return([]model.Card{}, nil).Once()
	moved := top
	moved.ColumnID = secondColumnID
	m.cards.EXPECT().MoveCard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: moved, Version: 8}, nil)
	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return(profiles(), nil)

	// when
	_, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: secondColumnID})

	// then card_moved carries the stack for everyone who sees it
	require.NoError(t, err)
	assert.Len(t, m.events.published, 1)
}
