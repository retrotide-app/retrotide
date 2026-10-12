package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReact_AddsOrRemovesThenListsTheCardsReactions(t *testing.T) {
	for _, add := range []bool{true, false} {
		t.Run(map[bool]string{true: "add", false: "remove"}[add], func(t *testing.T) {
			// given
			boards := dao.NewMockBoardDAO(t)
			reactions := dao.NewMockCardReactionDAO(t)
			repo := NewCardReactionRepo(nil, reactions, boards)
			write, inTx := boardWriteFixture()
			cardID := uuid.New()
			participantID := uuid.New()
			key := spec.CardReactionKey{OrgID: write.OrgID, BoardID: write.BoardID, CardID: cardID, ParticipantID: participantID, Reaction: model.ReactionRocket}
			listed := []model.CardReaction{{BoardID: write.BoardID, CardID: cardID, ParticipantID: new(participantID), Reaction: model.ReactionRocket}}
			var order []string
			boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
			boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
			if add {
				reactions.EXPECT().Add(mock.Anything, key, inTx).RunAndReturn(func(context.Context, spec.CardReactionKey, ...*sql.Tx) (bool, error) {
					order = append(order, "write")
					return true, nil
				}).Once()
			} else {
				reactions.EXPECT().Remove(mock.Anything, key, inTx).RunAndReturn(func(context.Context, spec.CardReactionKey, ...*sql.Tx) (bool, error) {
					order = append(order, "write")
					return true, nil
				}).Once()
			}
			reactions.EXPECT().ListForCard(mock.Anything, spec.CardRef{BoardID: write.BoardID, CardID: cardID}, inTx).RunAndReturn(func(context.Context, spec.CardRef, ...*sql.Tx) ([]model.CardReaction, error) {
				order = append(order, "list")
				return listed, nil
			}).Once()

			// when
			written, err := repo.React(t.Context(), spec.CardReactionWrite{BoardWrite: write, CardID: cardID, ParticipantID: participantID, Reaction: model.ReactionRocket, Add: add}, inTx...)

			// then
			require.NoError(t, err)
			assert.Equal(t, []string{"write", "list"}, order)
			assert.Equal(t, int64(42), written.Version)
			assert.Equal(t, listed, written.Value)
		})
	}
}

func TestPinCard_PinsThenReadsTheCardBack(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	cards := dao.NewMockCardDAO(t)
	repo := NewCardRepo(nil, cards, boards, dao.NewMockCardVoteDAO(t))
	write, inTx := boardWriteFixture()
	cardID := uuid.New()
	pinnedAt := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	cards.EXPECT().SetPinned(mock.Anything, spec.CardPinChange{BoardID: write.BoardID, CardID: cardID, Pinned: true}, inTx).Return(&model.Card{ID: cardID, PinnedAt: &pinnedAt}, nil).Once()
	cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: write.BoardID, CardID: cardID}, inTx).Return(&model.Card{ID: cardID, PinnedAt: &pinnedAt}, nil).Once()

	// when
	pinned, err := repo.PinCard(t.Context(), spec.CardPinWrite{BoardWrite: write, CardID: cardID, Pinned: true}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, &pinnedAt, pinned.Value.PinnedAt)
}

func TestSetCardImage_AnImageOfAnotherBoardIsNotFound(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	cards := dao.NewMockCardDAO(t)
	repo := NewCardRepo(nil, cards, boards, dao.NewMockCardVoteDAO(t))
	write, inTx := boardWriteFixture()
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	cards.EXPECT().SetImage(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	written, err := repo.SetCardImage(t.Context(), spec.CardImageWrite{BoardWrite: write, CardID: uuid.New(), MediaID: new(uuid.New())}, inTx...)

	// then the strict card mock refuses the read back
	assert.Nil(t, written)
	require.ErrorIs(t, err, dao.ErrNotFound)
}
