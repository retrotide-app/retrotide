package repository

import (
	"context"
	"database/sql"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMoveBoard_MovesTheBoardThenItsActionItemsInOneComposite(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	teamID := uuid.New()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().SetTeam(mock.Anything, spec.BoardTeamChange{OrgID: write.OrgID, BoardID: write.BoardID, TeamID: new(teamID)}, inTx).RunAndReturn(func(_ context.Context, s spec.BoardTeamChange, _ ...*sql.Tx) (*model.Board, error) {
		order = append(order, "board")
		return &model.Board{ID: s.BoardID, OrgID: s.OrgID, TeamID: s.TeamID}, nil
	}).Once()
	d.actionItems.EXPECT().SetTeamForBoard(mock.Anything, spec.ActionItemTeamChange{OrgID: write.OrgID, BoardID: write.BoardID, TeamID: new(teamID)}, inTx).RunAndReturn(func(context.Context, spec.ActionItemTeamChange, ...*sql.Tx) (int64, error) {
		order = append(order, "items")
		return 2, nil
	}).Once()

	// when
	moved, err := repo.MoveBoard(t.Context(), spec.BoardTeamWrite{BoardWrite: write, TeamID: new(teamID)}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"bump", "board", "items"}, order)
	assert.Equal(t, int64(42), moved.Version)
	assert.Equal(t, new(teamID), moved.Value.TeamID)
}

func TestMoveBoard_AMissingTeamMovesNoActionItem(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().SetTeam(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	moved, err := repo.MoveBoard(t.Context(), spec.BoardTeamWrite{BoardWrite: write, TeamID: new(uuid.New())}, inTx...)

	// then the strict action item mock refuses the rewrite
	assert.Nil(t, moved)
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestImportCards_CreatesTheNewColumnsThenTheCardsAtOneVersion(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	author := uuid.New()
	existing := uuid.New()
	column := spec.BoardColumnSeed{ID: uuid.New(), Title: "Kudos", Colour: model.ColourBlue, Position: "a2"}
	seeds := []spec.CardSeed{
		{CardID: uuid.New(), ColumnID: existing, AuthorParticipantID: new(author), Body: "Fast reviews", Position: "a0"},
		{CardID: uuid.New(), ColumnID: column.ID, AuthorParticipantID: new(author), Body: "Thanks Sam", Position: "a0"},
	}
	var order []string
	d.bumped(write, inTx, &order)
	d.columns.EXPECT().Create(mock.Anything, spec.NewBoardColumn{BoardID: write.BoardID, BoardColumnSeed: column}, inTx).RunAndReturn(func(_ context.Context, s spec.NewBoardColumn, _ ...*sql.Tx) (*model.BoardColumn, error) {
		order = append(order, "column")
		return &model.BoardColumn{ID: s.ID, BoardID: s.BoardID, Title: s.Title, Colour: s.Colour, Position: s.Position}, nil
	}).Once()
	for _, seed := range seeds {
		d.cards.EXPECT().Create(mock.Anything, spec.NewCard{
			ID:                  seed.CardID,
			BoardID:             write.BoardID,
			ColumnID:            seed.ColumnID,
			AuthorParticipantID: seed.AuthorParticipantID,
			Body:                seed.Body,
			Position:            seed.Position,
		}, inTx).RunAndReturn(func(_ context.Context, s spec.NewCard, _ ...*sql.Tx) (*model.Card, error) {
			order = append(order, "card")
			return &model.Card{ID: s.ID, BoardID: s.BoardID, ColumnID: s.ColumnID, Body: s.Body, Position: s.Position}, nil
		}).Once()
	}

	// when
	imported, err := repo.ImportCards(t.Context(), spec.CardImportWrite{BoardWrite: write, Columns: []spec.BoardColumnSeed{column}, Cards: seeds}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"bump", "column", "card", "card"}, order)
	assert.Equal(t, int64(42), imported.Version)
	require.Len(t, imported.Value.Columns, 1)
	assert.Equal(t, column.ID, imported.Value.Columns[0].ID)
	require.Len(t, imported.Value.Cards, 2)
	assert.Equal(t, "Thanks Sam", imported.Value.Cards[1].Body)
}

func TestImportCards_AFailedCardStopsTheImport(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	d.cards.EXPECT().Create(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	imported, err := repo.ImportCards(t.Context(), spec.CardImportWrite{BoardWrite: write, Cards: []spec.CardSeed{
		{CardID: uuid.New(), ColumnID: uuid.New(), Body: "Lost", Position: "a0"},
		{CardID: uuid.New(), ColumnID: uuid.New(), Body: "Never written", Position: "a1"},
	}}, inTx...)

	// then the second card is never attempted
	assert.Nil(t, imported)
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestDeleteBoard_BumpsTheVersionBeforeTheSoftDelete(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().SoftDelete(mock.Anything, spec.BoardKey{OrgID: write.OrgID, BoardID: write.BoardID}, inTx).RunAndReturn(func(_ context.Context, s spec.BoardKey, _ ...*sql.Tx) (*model.Board, error) {
		order = append(order, "delete")
		return &model.Board{ID: s.BoardID}, nil
	}).Once()

	// when
	deleted, err := repo.DeleteBoard(t.Context(), write, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"bump", "delete"}, order)
	assert.Equal(t, int64(42), deleted.Version)
}

func TestArchiveBoard_SetsTheArchiveFlagInsideABoardWrite(t *testing.T) {
	for _, archived := range []bool{true, false} {
		t.Run(map[bool]string{true: "archive", false: "unarchive"}[archived], func(t *testing.T) {
			// given
			repo, d := newBoardDAOs(t)
			write, inTx := boardWriteFixture()
			var order []string
			d.bumped(write, inTx, &order)
			d.boards.EXPECT().SetArchived(mock.Anything, spec.BoardArchiveChange{OrgID: write.OrgID, BoardID: write.BoardID, Archived: archived}, inTx).Return(&model.Board{ID: write.BoardID}, nil).Once()

			// when
			written, err := repo.ArchiveBoard(t.Context(), spec.BoardArchiveWrite{BoardWrite: write, Archived: archived}, inTx...)

			// then
			require.NoError(t, err)
			assert.Equal(t, int64(42), written.Version)
		})
	}
}

func TestSetBoardBackground_SetsTheMediaInsideABoardWrite(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	mediaID := uuid.New()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().SetBackground(mock.Anything, spec.BoardBackgroundChange{OrgID: write.OrgID, BoardID: write.BoardID, MediaID: new(mediaID)}, inTx).Return(&model.Board{ID: write.BoardID, BackgroundMediaID: new(mediaID)}, nil).Once()

	// when
	written, err := repo.SetBoardBackground(t.Context(), spec.BoardBackgroundWrite{BoardWrite: write, MediaID: new(mediaID)}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, new(mediaID), written.Value.BackgroundMediaID)
}
