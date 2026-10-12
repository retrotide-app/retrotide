package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCard_AddCardStoresItsAuthorAndColour(t *testing.T) {
	// given
	f := newBoardFixture(t)
	id := uuid.New()

	// when
	added, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
		BoardWrite:          f.write(t),
		CardID:              id,
		ColumnID:            f.column(0),
		AuthorParticipantID: new(f.facilitator()),
		Body:                "Deploys were smooth",
		Colour:              new(model.ColourAmber),
		Position:            "a0",
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), added.Version)
	assert.Equal(t, id, added.Value.ID)
	assert.Equal(t, new(f.facilitator()), added.Value.AuthorParticipantID)
	assert.Equal(t, new(model.ColourAmber), added.Value.Colour)
	assert.Nil(t, added.Value.ParentCardID)
	assert.Nil(t, added.Value.UpdatedAt)
	assert.Zero(t, added.Value.VoteTotal)

	read, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: id})
	require.NoError(t, err)
	assert.Equal(t, added.Value, *read)
}

func TestCard_ASecondCreateWithTheSameIDConflictsAnywhere(t *testing.T) {
	// given a card on one board, and a second board
	f := newBoardFixture(t)
	existing := f.addCard(t, f.column(0), "a0")
	other := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)

	// when
	_, sameBoardErr := f.repos.Card.AddCard(t.Context(), spec.CardCreate{BoardWrite: f.write(t), CardID: existing.ID, ColumnID: f.column(1), Body: "Again", Position: "a1"})
	_, otherBoardErr := f.repos.Card.Create(t.Context(), spec.NewCard{ID: existing.ID, BoardID: other.Header.ID, ColumnID: other.Columns[0].ID, Body: "Elsewhere", Position: "a0"})

	// then
	require.ErrorIs(t, sameBoardErr, dao.ErrConflict)
	require.ErrorIs(t, otherBoardErr, dao.ErrConflict)
	read, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: existing.ID})
	require.NoError(t, err)
	assert.Equal(t, existing, *read)
}

func TestCard_EditCardMarksItEditedAndRecolouringDoesNot(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")

	// when
	recoloured, colourErr := f.repos.Card.RecolourCard(t.Context(), spec.CardColourWrite{BoardWrite: f.write(t), CardID: card.ID, Colour: new(model.ColourPink)})
	cleared, clearErr := f.repos.Card.RecolourCard(t.Context(), spec.CardColourWrite{BoardWrite: f.write(t), CardID: card.ID})
	edited, editErr := f.repos.Card.EditCard(t.Context(), spec.CardBodyWrite{BoardWrite: f.write(t), CardID: card.ID, Body: "Standups ran long"})
	_, missingErr := f.repos.Card.EditCard(t.Context(), spec.CardBodyWrite{BoardWrite: f.write(t), CardID: uuid.New(), Body: "Nobody"})

	// then
	require.NoError(t, colourErr)
	assert.Equal(t, new(model.ColourPink), recoloured.Value.Colour)
	assert.Nil(t, recoloured.Value.UpdatedAt)

	require.NoError(t, clearErr)
	assert.Nil(t, cleared.Value.Colour)

	require.NoError(t, editErr)
	assert.Equal(t, "Standups ran long", edited.Value.Body)
	assert.NotNil(t, edited.Value.UpdatedAt)
	assert.Equal(t, int64(4), edited.Version)

	require.ErrorIs(t, missingErr, dao.ErrNotFound)
	assert.Equal(t, int64(4), f.version(t))
}

func TestCard_MoveCardChangesColumnAndRewritesTheOthers(t *testing.T) {
	// given two cards in the second column, and one in the first
	f := newBoardFixture(t)
	moving := f.addCard(t, f.column(0), "a0")
	upper := f.addCard(t, f.column(1), "a0")
	lower := f.addCard(t, f.column(1), "a1")

	// when the card lands between them, with the column's keys rewritten
	moved, err := f.repos.Card.MoveCard(t.Context(), spec.CardPlacement{
		BoardWrite: f.write(t),
		CardID:     moving.ID,
		ColumnID:   f.column(1),
		Position:   "b1",
		Rewrite:    []spec.PositionChange{{ID: upper.ID, Position: "b0"}, {ID: lower.ID, Position: "b2"}},
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, f.column(1), moved.Value.ColumnID)
	assert.Equal(t, int64(4), moved.Version)

	inColumn, err := f.repos.Card.ListInColumn(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(1)})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{upper.ID, moving.ID, lower.ID}, cardIDs(inColumn))
	emptied, err := f.repos.Card.ListInColumn(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)})
	require.NoError(t, err)
	assert.Empty(t, emptied)
}

func TestCard_ListInColumnBreaksPositionTiesByID(t *testing.T) {
	// given
	f := newBoardFixture(t)
	low := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	high := uuid.MustParse("ffffffff-ffff-4fff-bfff-ffffffffffff")
	for _, id := range []uuid.UUID{high, low} {
		_, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{BoardWrite: f.write(t), CardID: id, ColumnID: f.column(0), Body: "Tied", Position: "a0"})
		require.NoError(t, err)
	}

	// when
	listed, err := f.repos.Card.ListInColumn(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)})

	// then
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{low, high}, cardIDs(listed))
}

func TestCard_DeleteCardRemovesOnlyThatCard(t *testing.T) {
	// given
	f := newBoardFixture(t)
	gone := f.addCard(t, f.column(0), "a0")
	kept := f.addCard(t, f.column(0), "a1")

	// when
	deleted, err := f.repos.Card.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: f.write(t), CardID: gone.ID})
	_, againErr := f.repos.Card.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: f.write(t), CardID: gone.ID})

	// then
	require.NoError(t, err)
	assert.Equal(t, gone.ID, deleted.Value.ID)
	require.ErrorIs(t, againErr, dao.ErrNotFound)
	listed, listErr := f.repos.Card.List(t.Context(), f.boardID())
	require.NoError(t, listErr)
	assert.Equal(t, []uuid.UUID{kept.ID}, cardIDs(listed))
}

func TestCard_ClearCardsEmptiesOnlyThatColumn(t *testing.T) {
	// given
	f := newBoardFixture(t)
	f.addCard(t, f.column(0), "a0")
	f.addCard(t, f.column(0), "a1")
	kept := f.addCard(t, f.column(1), "a0")

	// when
	cleared, err := f.repos.Card.ClearCards(t.Context(), spec.ColumnRemoval{BoardWrite: f.write(t), ColumnID: f.column(0)})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), cleared.Value)
	assert.Equal(t, int64(4), cleared.Version)
	listed, listErr := f.repos.Card.List(t.Context(), f.boardID())
	require.NoError(t, listErr)
	assert.Equal(t, []uuid.UUID{kept.ID}, cardIDs(listed))
	_, columnErr := f.repos.BoardColumn.GetByID(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)})
	require.NoError(t, columnErr)
}

func TestCard_AnAuthorFromAnotherBoardIsRefused(t *testing.T) {
	// given a participant of another board
	f := newBoardFixture(t)
	other := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)

	// when
	_, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
		BoardWrite:          f.write(t),
		CardID:              uuid.New(),
		ColumnID:            f.column(0),
		AuthorParticipantID: new(other.Participants[0].ID),
		Body:                "Borrowed author",
		Position:            "a0",
	})

	// then the composite key refuses an author who is not on this board
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Zero(t, f.version(t))
}

func cardIDs(cards []model.Card) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.ID)
	}

	return ids
}
