package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f boardFixture) merge(t *testing.T, card, parent model.Card, position string) *model.Versioned[model.CardMerge] {
	t.Helper()

	merged, err := f.repos.Card.MergeCard(t.Context(), spec.CardMergeWrite{
		BoardWrite:   f.write(t),
		CardID:       card.ID,
		ParentCardID: parent.ID,
		ColumnID:     parent.ColumnID,
		Position:     position,
	})
	require.NoError(t, err)

	return merged
}

func (f boardFixture) card(t *testing.T, id uuid.UUID) model.Card {
	t.Helper()

	card, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: id})
	require.NoError(t, err)

	return *card
}

func TestCard_MergeStacksTheCardInItsTargetsColumnAndKeepsItsVotes(t *testing.T) {
	// given a voted card in the second column and a target in the first
	f := newBoardFixture(t)
	target := f.addCard(t, f.column(0), "a0")
	dragged := f.addCard(t, f.column(1), "a0")
	_, err := f.vote(t, dragged.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)

	// when
	merged := f.merge(t, dragged, target, "a1")

	// then
	assert.Equal(t, new(target.ID), merged.Value.Card.ParentCardID)
	assert.Equal(t, f.column(0), merged.Value.Card.ColumnID)
	assert.Equal(t, "a1", merged.Value.Card.Position)
	assert.Empty(t, merged.Value.Restacked)
	assert.Equal(t, 1, merged.Value.Card.VoteTotal)
	assert.Equal(t, int64(4), merged.Version)
}

func TestCard_MergingAStackMovesItsCardsOntoTheNewTarget(t *testing.T) {
	// given a stack of two in the second column, and a target in the first
	f := newBoardFixture(t)
	target := f.addCard(t, f.column(0), "a0")
	top := f.addCard(t, f.column(1), "a0")
	child := f.addCard(t, f.column(1), "a1")
	f.merge(t, child, top, "a1")

	// when
	merged := f.merge(t, top, target, "a2")

	// then the stack stays one level deep
	assert.Equal(t, []uuid.UUID{child.ID}, merged.Value.Restacked)
	restacked := f.card(t, child.ID)
	assert.Equal(t, new(target.ID), restacked.ParentCardID)
	assert.Equal(t, f.column(0), restacked.ColumnID)
}

func TestCard_MovingAStackTakesItsCardsAlongAndMovingAChildUnmergesIt(t *testing.T) {
	// given a stack of two in the first column
	f := newBoardFixture(t)
	top := f.addCard(t, f.column(0), "a0")
	child := f.addCard(t, f.column(0), "a1")
	f.merge(t, child, top, "a1")

	// when the stack moves, then its child is dragged out again
	_, moveErr := f.repos.Card.MoveCard(t.Context(), spec.CardPlacement{BoardWrite: f.write(t), CardID: top.ID, ColumnID: f.column(1), Position: "b0"})
	carried := f.card(t, child.ID)
	unmerged, unmergeErr := f.repos.Card.MoveCard(t.Context(), spec.CardPlacement{BoardWrite: f.write(t), CardID: child.ID, ColumnID: f.column(1), Position: "b1"})

	// then
	require.NoError(t, moveErr)
	assert.Equal(t, f.column(1), carried.ColumnID)
	assert.Equal(t, new(top.ID), carried.ParentCardID)
	require.NoError(t, unmergeErr)
	assert.Nil(t, unmerged.Value.ParentCardID)
	assert.Equal(t, "b1", unmerged.Value.Position)
}

func TestCard_CrossTenantMergesAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given
	bt := newBoardTenants(t)
	boardB := bt.boardB.Header.ID
	cardB, err := bt.repos.Card.Create(t.Context(), spec.NewCard{ID: uuid.New(), BoardID: boardB, ColumnID: bt.boardB.Columns[0].ID, Body: "Elsewhere", Position: "a0"})
	require.NoError(t, err)

	// when
	_, stackErr := bt.repos.Card.Stack(t.Context(), spec.CardStacking{BoardID: boardB, CardID: bt.cardA.ID, ParentCardID: cardB.ID, ColumnID: cardB.ColumnID, Position: "a1"})
	_, foreignParentErr := bt.repos.Card.Stack(t.Context(), spec.CardStacking{BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, ParentCardID: cardB.ID, ColumnID: bt.cardA.ColumnID, Position: "a1"})
	restacked, restackErr := bt.repos.Card.Restack(t.Context(), spec.CardRestack{BoardID: boardB, FromParentCardID: bt.cardA.ID, ToParentCardID: cardB.ID, ColumnID: cardB.ColumnID})
	_, mergeErr := bt.repos.Card.MergeCard(t.Context(), spec.CardMergeWrite{BoardWrite: bt.boardAThroughB(), CardID: bt.cardA.ID, ParentCardID: cardB.ID, ColumnID: cardB.ColumnID, Position: "a1"})

	// then a card of board A can never join a stack on board B
	require.ErrorIs(t, stackErr, dao.ErrNotFound)
	require.ErrorIs(t, foreignParentErr, dao.ErrNotFound)
	require.ErrorIs(t, mergeErr, dao.ErrNotFound)
	require.NoError(t, restackErr)
	assert.Empty(t, restacked)
	bt.boardAUnchanged(t)
}
