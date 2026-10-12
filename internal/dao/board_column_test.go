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

func TestColumn_AddColumnPlacesItByItsPosition(t *testing.T) {
	// given
	f := newBoardFixture(t)
	seed := spec.BoardColumnSeed{ID: uuid.New(), Title: "Ideas", Description: "Anything new", Colour: model.ColourBlue, Position: "a0V"}

	// when
	added, err := f.repos.BoardColumn.AddColumn(t.Context(), spec.ColumnCreate{BoardWrite: f.write(t), Column: seed})
	_, dupErr := f.repos.BoardColumn.AddColumn(t.Context(), spec.ColumnCreate{BoardWrite: f.write(t), Column: seed})
	listed, listErr := f.repos.BoardColumn.List(t.Context(), f.boardID())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), added.Version)
	assert.Equal(t, seed.ID, added.Value.ID)
	assert.Equal(t, "Anything new", added.Value.Description)
	assert.Equal(t, model.ColourBlue, added.Value.Colour)
	require.ErrorIs(t, dupErr, dao.ErrConflict)
	require.NoError(t, listErr)
	assert.Equal(t, []uuid.UUID{f.column(0), seed.ID, f.column(1)}, columnIDs(listed))
}

func TestColumn_ListBreaksPositionTiesByID(t *testing.T) {
	// given two columns that share a position
	f := newBoardFixture(t)
	low := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	high := uuid.MustParse("ffffffff-ffff-4fff-bfff-ffffffffffff")
	for _, id := range []uuid.UUID{high, low} {
		_, err := f.repos.BoardColumn.AddColumn(t.Context(), spec.ColumnCreate{BoardWrite: f.write(t), Column: spec.BoardColumnSeed{ID: id, Title: "Tied", Colour: model.ColourGrey, Position: "b0"}})
		require.NoError(t, err)
	}

	// when
	listed, err := f.repos.BoardColumn.List(t.Context(), f.boardID())

	// then
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{f.column(0), f.column(1), low, high}, columnIDs(listed))
}

func TestColumn_PositionsSortByteWise(t *testing.T) {
	// given keys whose order differs between byte order and a linguistic collation
	f := newBoardFixture(t)
	upper := uuid.New()
	lower := uuid.New()
	_, err := f.repos.BoardColumn.Move(t.Context(), spec.BoardColumnMove{BoardID: f.boardID(), ColumnID: f.column(0), Position: "b"})
	require.NoError(t, err)
	_, err = f.repos.BoardColumn.Move(t.Context(), spec.BoardColumnMove{BoardID: f.boardID(), ColumnID: f.column(1), Position: "c"})
	require.NoError(t, err)
	for id, position := range map[uuid.UUID]string{upper: "Z", lower: "a"} {
		_, err := f.repos.BoardColumn.Create(t.Context(), spec.NewBoardColumn{BoardID: f.boardID(), ID: id, Title: position, Colour: model.ColourGrey, Position: position})
		require.NoError(t, err)
	}

	// when
	listed, err := f.repos.BoardColumn.List(t.Context(), f.boardID())

	// then upper case sorts before lower case, as fracindex expects
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{upper, lower, f.column(0), f.column(1)}, columnIDs(listed))
}

func TestColumn_EditColumnRenamesDescribesAndRecolours(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	edited, err := f.repos.BoardColumn.EditColumn(t.Context(), spec.ColumnEdit{BoardWrite: f.write(t), ColumnID: f.column(0), Title: "Glad", Description: "What made you happy", Colour: model.ColourTeal})
	_, missingErr := f.repos.BoardColumn.EditColumn(t.Context(), spec.ColumnEdit{BoardWrite: f.write(t), ColumnID: uuid.New(), Title: "Gone", Colour: model.ColourTeal})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), edited.Version)
	assert.Equal(t, "Glad", edited.Value.Title)
	assert.Equal(t, "What made you happy", edited.Value.Description)
	assert.Equal(t, model.ColourTeal, edited.Value.Colour)
	require.ErrorIs(t, missingErr, dao.ErrNotFound)
	assert.Equal(t, int64(1), f.version(t))

	read, err := f.repos.BoardColumn.GetByID(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)})
	require.NoError(t, err)
	assert.Equal(t, edited.Value, *read)
}

func TestColumn_MoveColumnRewritesTheOthersInTheSameCommand(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when the second column moves first and the first is given a fresh key
	moved, err := f.repos.BoardColumn.MoveColumn(t.Context(), spec.ColumnPlacement{
		BoardWrite: f.write(t),
		ColumnID:   f.column(1),
		Position:   "a0",
		Rewrite:    []spec.PositionChange{{ID: f.column(0), Position: "a1"}},
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), moved.Version)
	listed, err := f.repos.BoardColumn.List(t.Context(), f.boardID())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{f.column(1), f.column(0)}, columnIDs(listed))
}

func TestColumn_AMoveWhoseRewriteFailsChangesNothing(t *testing.T) {
	// given
	f := newBoardFixture(t)
	read := f.write(t)

	// when the rewrite names a column the board does not have
	_, err := f.repos.BoardColumn.MoveColumn(t.Context(), spec.ColumnPlacement{
		BoardWrite: read,
		ColumnID:   f.column(1),
		Position:   "a0",
		Rewrite:    []spec.PositionChange{{ID: uuid.New(), Position: "a1"}},
	})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, read.Version, f.version(t))
	listed, listErr := f.repos.BoardColumn.List(t.Context(), f.boardID())
	require.NoError(t, listErr)
	assert.Equal(t, []uuid.UUID{f.column(0), f.column(1)}, columnIDs(listed))
}

func TestColumn_DeleteColumnTakesItsCardsWithIt(t *testing.T) {
	// given
	f := newBoardFixture(t)
	f.addCard(t, f.column(0), "a0")
	kept := f.addCard(t, f.column(1), "a0")

	// when
	deleted, err := f.repos.BoardColumn.DeleteColumn(t.Context(), spec.ColumnRemoval{BoardWrite: f.write(t), ColumnID: f.column(0)})
	_, againErr := f.repos.BoardColumn.DeleteColumn(t.Context(), spec.ColumnRemoval{BoardWrite: f.write(t), ColumnID: f.column(0)})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted.Version)
	require.ErrorIs(t, againErr, dao.ErrNotFound)
	cards, listErr := f.repos.Card.List(t.Context(), f.boardID())
	require.NoError(t, listErr)
	require.Len(t, cards, 1)
	assert.Equal(t, kept.ID, cards[0].ID)
}

func columnIDs(columns []model.BoardColumn) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(columns))
	for _, column := range columns {
		ids = append(ids, column.ID)
	}

	return ids
}
