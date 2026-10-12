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

func boardWriteFixture() (spec.BoardWrite, []*sql.Tx) {
	return spec.BoardWrite{OrgID: uuid.New(), BoardID: uuid.New(), Version: 41, OpID: uuid.New()}, []*sql.Tx{new(sql.Tx)}
}

func TestBoardWrite_BumpsTheVersionThenRecordsTheOpThenWrites(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	cards := dao.NewMockCardDAO(t)
	repo := NewCardRepo(nil, cards, boards, dao.NewMockCardVoteDAO(t))
	write, inTx := boardWriteFixture()
	command := spec.CardCreate{BoardWrite: write, CardID: uuid.New(), ColumnID: uuid.New(), Body: "Deploys were smooth", Position: "a0"}
	var order []string
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).RunAndReturn(func(context.Context, spec.BoardWrite, ...*sql.Tx) (int64, error) {
		order = append(order, "bump")
		return 42, nil
	}).Once()
	boards.EXPECT().RecordOp(mock.Anything, spec.NewBoardOp{BoardID: write.BoardID, OpID: write.OpID, Version: 42}, inTx).RunAndReturn(func(context.Context, spec.NewBoardOp, ...*sql.Tx) error {
		order = append(order, "op")
		return nil
	}).Once()
	cards.EXPECT().Create(mock.Anything, spec.NewCard{ID: command.CardID, BoardID: write.BoardID, ColumnID: command.ColumnID, Body: command.Body, Position: "a0"}, inTx).RunAndReturn(func(_ context.Context, s spec.NewCard, _ ...*sql.Tx) (*model.Card, error) {
		order = append(order, "insert")
		return &model.Card{ID: s.ID, BoardID: s.BoardID}, nil
	}).Once()
	cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: write.BoardID, CardID: command.CardID}, inTx).Return(&model.Card{ID: command.CardID, BoardID: write.BoardID}, nil).Once()

	// when
	added, err := repo.AddCard(t.Context(), command, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"bump", "op", "insert"}, order)
	assert.Equal(t, int64(42), added.Version)
	assert.Equal(t, command.CardID, added.Value.ID)
}

func TestBoardWrite_AWriteWithoutAnOpRecordsNone(t *testing.T) {
	// given a participant joining, which carries no op
	boards := dao.NewMockBoardDAO(t)
	repo := NewBoardRepo(nil, boards, BoardRepoDeps{})
	write, inTx := boardWriteFixture()
	write.OpID = uuid.Nil
	userID := uuid.New()
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().AddParticipant(mock.Anything, spec.NewBoardParticipant{BoardID: write.BoardID, UserID: new(userID), Role: model.BoardRoleParticipant}, inTx).Return(&model.BoardParticipant{BoardID: write.BoardID, UserID: new(userID)}, nil).Once()

	// when
	joined, err := repo.JoinParticipant(t.Context(), spec.BoardParticipantJoin{BoardWrite: write, UserID: userID, Role: model.BoardRoleParticipant}, inTx...)

	// then the strict mock refuses any RecordOp
	require.NoError(t, err)
	assert.Equal(t, int64(42), joined.Version)
}

func TestBoardWrite_StopsAtAConditionalStatementThatMatchesNothing(t *testing.T) {
	tests := []struct {
		name    string
		bumpErr error
		opErr   error
		want    error
	}{
		{name: "a version read before another command committed", bumpErr: dao.ErrVersionConflict, want: dao.ErrVersionConflict},
		{name: "an op already applied", opErr: dao.ErrDuplicateOp, want: dao.ErrDuplicateOp},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			boards := dao.NewMockBoardDAO(t)
			cards := dao.NewMockCardDAO(t)
			repo := NewCardRepo(nil, cards, boards, dao.NewMockCardVoteDAO(t))
			write, inTx := boardWriteFixture()
			boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, tc.bumpErr).Once()
			if tc.bumpErr == nil {
				boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(tc.opErr).Once()
			}

			// when
			_, err := repo.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: write, CardID: uuid.New()}, inTx...)

			// then the strict card and vote mocks refuse the refund and the delete
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCardMove_RewritesTheColumnBeforeMovingTheCard(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	cards := dao.NewMockCardDAO(t)
	repo := NewCardRepo(nil, cards, boards, dao.NewMockCardVoteDAO(t))
	write, inTx := boardWriteFixture()
	command := spec.CardPlacement{
		BoardWrite: write,
		CardID:     uuid.New(),
		ColumnID:   uuid.New(),
		Position:   "b1",
		Rewrite:    []spec.PositionChange{{ID: uuid.New(), Position: "b0"}, {ID: uuid.New(), Position: "b2"}},
	}
	var moved []uuid.UUID
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	cards.EXPECT().Move(mock.Anything, mock.Anything, inTx).RunAndReturn(func(_ context.Context, s spec.CardMove, _ ...*sql.Tx) (*model.Card, error) {
		moved = append(moved, s.CardID)
		return &model.Card{ID: s.CardID, BoardID: s.BoardID, ColumnID: s.ColumnID, Position: s.Position}, nil
	}).Times(3)
	cards.EXPECT().Restack(mock.Anything, spec.CardRestack{BoardID: write.BoardID, FromParentCardID: command.CardID, ToParentCardID: command.CardID, ColumnID: command.ColumnID}, inTx).Return(nil, nil).Once()

	// when
	placed, err := repo.MoveCard(t.Context(), command, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{command.Rewrite[0].ID, command.Rewrite[1].ID, command.CardID}, moved)
	assert.Equal(t, command.CardID, placed.Value.ID)
	assert.Equal(t, "b1", placed.Value.Position)
}
