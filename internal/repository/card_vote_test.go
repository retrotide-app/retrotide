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

type (
	voteMocks struct {
		boards *dao.MockBoardDAO
		cards  *dao.MockCardDAO
		votes  *dao.MockCardVoteDAO
		repo   CardVoteRepository
	}
)

func newVoteMocks(t *testing.T) voteMocks {
	t.Helper()

	m := voteMocks{boards: dao.NewMockBoardDAO(t), cards: dao.NewMockCardDAO(t), votes: dao.NewMockCardVoteDAO(t)}
	m.repo = NewCardVoteRepo(nil, m.votes, CardVoteRepoDeps{Boards: m.boards, Cards: m.cards})

	return m
}

func (m voteMocks) began(write spec.BoardWrite, inTx []*sql.Tx) {
	m.boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	m.boards.EXPECT().RecordOp(mock.Anything, spec.NewBoardOp{BoardID: write.BoardID, OpID: write.OpID, Version: 42}, inTx).Return(nil).Once()
}

func TestCardVote_AddVoteSpendsThenCountsThenRaisesTheTotalThenListsTheVotersVotes(t *testing.T) {
	// given
	m := newVoteMocks(t)
	write, inTx := boardWriteFixture()
	command := spec.CardVote{BoardWrite: write, CardID: uuid.New(), ParticipantID: uuid.New(), MaxVotes: 6, MaxVotesPerCard: 2}
	mine := []model.CardVote{{CardID: command.CardID, ParticipantID: command.ParticipantID, Count: 2}}
	var order []string
	m.began(write, inTx)
	m.votes.EXPECT().SpendBudget(mock.Anything, spec.VoteBudgetSpend{BoardID: write.BoardID, ParticipantID: command.ParticipantID, MaxVotes: 6}, inTx).RunAndReturn(func(context.Context, spec.VoteBudgetSpend, ...*sql.Tx) (int, error) {
		order = append(order, "spend")
		return 2, nil
	}).Once()
	m.votes.EXPECT().IncrementVote(mock.Anything, spec.CardVoteIncrement{BoardID: write.BoardID, CardID: command.CardID, ParticipantID: command.ParticipantID, MaxVotesPerCard: 2}, inTx).RunAndReturn(func(context.Context, spec.CardVoteIncrement, ...*sql.Tx) (int, error) {
		order = append(order, "count")
		return 2, nil
	}).Once()
	m.cards.EXPECT().AddVoteTotal(mock.Anything, spec.CardVoteTotalChange{BoardID: write.BoardID, CardID: command.CardID, Delta: 1}, inTx).RunAndReturn(func(_ context.Context, s spec.CardVoteTotalChange, _ ...*sql.Tx) (*model.Card, error) {
		order = append(order, "total")
		return &model.Card{ID: s.CardID, BoardID: s.BoardID, VoteTotal: 5}, nil
	}).Once()
	m.votes.EXPECT().ListForParticipant(mock.Anything, spec.BoardParticipantKey{BoardID: write.BoardID, ParticipantID: command.ParticipantID}, inTx).RunAndReturn(func(context.Context, spec.BoardParticipantKey, ...*sql.Tx) ([]model.CardVote, error) {
		order = append(order, "list")
		return mine, nil
	}).Once()

	// when
	voted, err := m.repo.AddVote(t.Context(), command, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"spend", "count", "total", "list"}, order)
	assert.Equal(t, int64(42), voted.Version)
	assert.Equal(t, 5, voted.Value.Card.VoteTotal)
	assert.Equal(t, 2, voted.Value.Count)
	assert.Equal(t, mine, voted.Value.Mine)
}

func TestCardVote_AddVoteStopsAtAnySpentLimit(t *testing.T) {
	tests := []struct {
		name     string
		spendErr error
		countErr error
		want     error
	}{
		{name: "the board's budget", spendErr: dao.ErrVoteBudgetSpent, want: dao.ErrVoteBudgetSpent},
		{name: "the card's cap", countErr: dao.ErrCardVoteCapReached, want: dao.ErrCardVoteCapReached},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			m := newVoteMocks(t)
			write, inTx := boardWriteFixture()
			m.began(write, inTx)
			m.votes.EXPECT().SpendBudget(mock.Anything, mock.Anything, inTx).Return(1, tc.spendErr).Once()
			if tc.spendErr == nil {
				m.votes.EXPECT().IncrementVote(mock.Anything, mock.Anything, inTx).Return(0, tc.countErr).Once()
			}

			// when
			_, err := m.repo.AddVote(t.Context(), spec.CardVote{BoardWrite: write, CardID: uuid.New(), ParticipantID: uuid.New(), MaxVotes: 1}, inTx...)

			// then the strict mocks prove nothing else is written
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCardVote_RemoveVoteDropsTheLastVoteThenRefunds(t *testing.T) {
	// given a participant holding one vote on the card
	m := newVoteMocks(t)
	write, inTx := boardWriteFixture()
	command := spec.CardVote{BoardWrite: write, CardID: uuid.New(), ParticipantID: uuid.New()}
	key := spec.CardVoteKey{BoardID: write.BoardID, CardID: command.CardID, ParticipantID: command.ParticipantID}
	m.began(write, inTx)
	m.votes.EXPECT().LowerVote(mock.Anything, key, inTx).Return(0, dao.ErrNotFound).Once()
	m.votes.EXPECT().DropVote(mock.Anything, key, inTx).Return(nil).Once()
	m.votes.EXPECT().RefundBudget(mock.Anything, spec.BoardParticipantKey{BoardID: write.BoardID, ParticipantID: command.ParticipantID}, inTx).Return(0, nil).Once()
	m.cards.EXPECT().AddVoteTotal(mock.Anything, spec.CardVoteTotalChange{BoardID: write.BoardID, CardID: command.CardID, Delta: -1}, inTx).Return(&model.Card{ID: command.CardID}, nil).Once()
	m.votes.EXPECT().ListForParticipant(mock.Anything, mock.Anything, inTx).Return([]model.CardVote{}, nil).Once()

	// when
	removed, err := m.repo.RemoveVote(t.Context(), command, inTx...)

	// then
	require.NoError(t, err)
	assert.Zero(t, removed.Value.Count)
	assert.Empty(t, removed.Value.Mine)
}

func TestCardVote_RemoveVoteWithNoVoteToTakeBackRefundsNothing(t *testing.T) {
	// given
	m := newVoteMocks(t)
	write, inTx := boardWriteFixture()
	m.began(write, inTx)
	m.votes.EXPECT().LowerVote(mock.Anything, mock.Anything, inTx).Return(0, dao.ErrNotFound).Once()
	m.votes.EXPECT().DropVote(mock.Anything, mock.Anything, inTx).Return(dao.ErrNotFound).Once()

	// when
	_, err := m.repo.RemoveVote(t.Context(), spec.CardVote{BoardWrite: write, CardID: uuid.New(), ParticipantID: uuid.New()}, inTx...)

	// then the strict mocks refuse any refund
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestCardVote_ResetVotesDeletesVotesAndBudgetsThenZeroesTheTotals(t *testing.T) {
	// given
	m := newVoteMocks(t)
	write, inTx := boardWriteFixture()
	var order []string
	m.began(write, inTx)
	m.votes.EXPECT().DeleteAll(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "votes")
		return 3, nil
	}).Once()
	m.votes.EXPECT().DeleteBudgets(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "budgets")
		return 2, nil
	}).Once()
	m.cards.EXPECT().ResetVoteTotals(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "totals")
		return 2, nil
	}).Once()

	// when
	reset, err := m.repo.ResetVotes(t.Context(), write, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"votes", "budgets", "totals"}, order)
	assert.Equal(t, int64(3), reset.Value)
}

func TestCardVote_ResetColumnVotesRefundsThenDeletesThenZeroesTheColumnsTotals(t *testing.T) {
	// given
	m := newVoteMocks(t)
	write, inTx := boardWriteFixture()
	key := spec.BoardColumnKey{BoardID: write.BoardID, ColumnID: uuid.New()}
	var order []string
	m.began(write, inTx)
	m.votes.EXPECT().RefundColumn(mock.Anything, key, inTx).RunAndReturn(func(context.Context, spec.BoardColumnKey, ...*sql.Tx) (int64, error) {
		order = append(order, "refund")
		return 2, nil
	}).Once()
	m.votes.EXPECT().DeleteColumn(mock.Anything, key, inTx).RunAndReturn(func(context.Context, spec.BoardColumnKey, ...*sql.Tx) (int64, error) {
		order = append(order, "votes")
		return 3, nil
	}).Once()
	m.cards.EXPECT().ResetColumnVoteTotals(mock.Anything, key, inTx).RunAndReturn(func(context.Context, spec.BoardColumnKey, ...*sql.Tx) (int64, error) {
		order = append(order, "totals")
		return 2, nil
	}).Once()

	// when
	reset, err := m.repo.ResetColumnVotes(t.Context(), spec.ColumnVotesResetWrite{BoardWrite: write, ColumnID: key.ColumnID}, inTx...)

	// then the budgets are refunded while the votes still exist to count them
	require.NoError(t, err)
	assert.Equal(t, []string{"refund", "votes", "totals"}, order)
	assert.Equal(t, int64(42), reset.Version)
	assert.Equal(t, int64(3), reset.Value)
}

func TestCardVote_AddVoteChecksTheColumnsBudgetBeforeSpending(t *testing.T) {
	// given a column that allows two votes, both already cast
	m := newVoteMocks(t)
	write, inTx := boardWriteFixture()
	command := spec.CardVote{BoardWrite: write, CardID: uuid.New(), ParticipantID: uuid.New(), MaxVotes: 6, ColumnID: uuid.New(), ColumnMaxVotes: 2}
	m.began(write, inTx)
	check := spec.ColumnVoteBudgetCheck{BoardID: write.BoardID, ColumnID: command.ColumnID, ParticipantID: command.ParticipantID, MaxVotes: 2}
	m.votes.EXPECT().CheckColumnBudget(mock.Anything, check, inTx).Return(dao.ErrColumnVoteBudgetSpent).Once()

	// when
	_, err := m.repo.AddVote(t.Context(), command, inTx...)

	// then the strict mocks prove the board's budget is never spent
	require.ErrorIs(t, err, dao.ErrColumnVoteBudgetSpent)
}

func TestCardVote_AddVoteWithRoomInTheColumnVotesAsBefore(t *testing.T) {
	// given
	m := newVoteMocks(t)
	write, inTx := boardWriteFixture()
	command := spec.CardVote{BoardWrite: write, CardID: uuid.New(), ParticipantID: uuid.New(), MaxVotes: 6, ColumnID: uuid.New(), ColumnMaxVotes: 2}
	var order []string
	m.began(write, inTx)
	m.votes.EXPECT().CheckColumnBudget(mock.Anything, mock.Anything, inTx).RunAndReturn(func(context.Context, spec.ColumnVoteBudgetCheck, ...*sql.Tx) error {
		order = append(order, "column")
		return nil
	}).Once()
	m.votes.EXPECT().SpendBudget(mock.Anything, mock.Anything, inTx).RunAndReturn(func(context.Context, spec.VoteBudgetSpend, ...*sql.Tx) (int, error) {
		order = append(order, "spend")
		return 1, nil
	}).Once()
	m.votes.EXPECT().IncrementVote(mock.Anything, mock.Anything, inTx).Return(1, nil).Once()
	m.cards.EXPECT().AddVoteTotal(mock.Anything, mock.Anything, inTx).Return(&model.Card{ID: command.CardID}, nil).Once()
	m.votes.EXPECT().ListForParticipant(mock.Anything, mock.Anything, inTx).Return([]model.CardVote{}, nil).Once()

	// when
	_, err := m.repo.AddVote(t.Context(), command, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"column", "spend"}, order)
}

func TestCard_DeleteCardRefundsItsVotesBeforeTheCascade(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	cards := dao.NewMockCardDAO(t)
	votes := dao.NewMockCardVoteDAO(t)
	repo := NewCardRepo(nil, cards, boards, votes)
	write, inTx := boardWriteFixture()
	ref := spec.CardRef{BoardID: write.BoardID, CardID: uuid.New()}
	var order []string
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	votes.EXPECT().RefundCard(mock.Anything, ref, inTx).RunAndReturn(func(context.Context, spec.CardRef, ...*sql.Tx) (int64, error) {
		order = append(order, "refund")
		return 2, nil
	}).Once()
	cards.EXPECT().Delete(mock.Anything, ref, inTx).RunAndReturn(func(_ context.Context, s spec.CardRef, _ ...*sql.Tx) (*model.Card, error) {
		order = append(order, "delete")
		return &model.Card{ID: s.CardID, BoardID: s.BoardID}, nil
	}).Once()

	// when
	_, err := repo.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: write, CardID: ref.CardID}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"refund", "delete"}, order)
}

func TestCard_MergeCardRestacksTheDraggedCardsStackBeforeStackingIt(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	cards := dao.NewMockCardDAO(t)
	repo := NewCardRepo(nil, cards, boards, dao.NewMockCardVoteDAO(t))
	write, inTx := boardWriteFixture()
	command := spec.CardMergeWrite{BoardWrite: write, CardID: uuid.New(), ParentCardID: uuid.New(), ColumnID: uuid.New(), Position: "a5"}
	child := uuid.New()
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	cards.EXPECT().Restack(mock.Anything, spec.CardRestack{BoardID: write.BoardID, FromParentCardID: command.CardID, ToParentCardID: command.ParentCardID, ColumnID: command.ColumnID}, inTx).Return([]uuid.UUID{child}, nil).Once()
	stacking := spec.CardStacking{BoardID: write.BoardID, CardID: command.CardID, ParentCardID: command.ParentCardID, ColumnID: command.ColumnID, Position: "a5"}
	cards.EXPECT().Stack(mock.Anything, stacking, inTx).Return(&model.Card{ID: command.CardID, ParentCardID: new(command.ParentCardID)}, nil).Once()

	// when
	merged, err := repo.MergeCard(t.Context(), command, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{child}, merged.Value.Restacked)
	assert.Equal(t, new(command.ParentCardID), merged.Value.Card.ParentCardID)
}

func TestBoard_ClaimingAGuestHandsOverItsCardsCommentsAndVotes(t *testing.T) {
	// given a guest who already holds an account seat on the board
	boards := dao.NewMockBoardDAO(t)
	cards := dao.NewMockCardDAO(t)
	votes := dao.NewMockCardVoteDAO(t)
	comments := dao.NewMockCardCommentDAO(t)
	reactions := dao.NewMockCardReactionDAO(t)
	surveys := dao.NewMockSurveyDAO(t)
	repo := NewBoardRepo(nil, boards, BoardRepoDeps{Cards: cards, Votes: votes, Comments: comments, Reactions: reactions, Surveys: surveys})
	write, inTx := boardWriteFixture()
	guest := uuid.New()
	seat := uuid.New()
	user := uuid.New()
	reassignment := spec.CardAuthorReassignment{BoardID: write.BoardID, FromParticipantID: guest, ToParticipantID: seat}
	handover := spec.VoteHandover{BoardID: write.BoardID, FromParticipantID: guest, ToParticipantID: seat}
	seatHandover := spec.ParticipantHandover{BoardID: write.BoardID, FromParticipantID: guest, ToParticipantID: seat}
	var order []string
	reactions.EXPECT().HandOver(mock.Anything, seatHandover, inTx).RunAndReturn(func(context.Context, spec.ParticipantHandover, ...*sql.Tx) (int64, error) {
		order = append(order, "reactions")
		return 2, nil
	}).Once()
	surveys.EXPECT().HandOverResponses(mock.Anything, seatHandover, inTx).RunAndReturn(func(context.Context, spec.ParticipantHandover, ...*sql.Tx) (int64, error) {
		order = append(order, "answers")
		return 3, nil
	}).Once()
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	cards.EXPECT().ReassignAuthor(mock.Anything, reassignment, inTx).RunAndReturn(func(context.Context, spec.CardAuthorReassignment, ...*sql.Tx) (int64, error) {
		order = append(order, "cards")
		return 1, nil
	}).Once()
	comments.EXPECT().ReassignAuthor(mock.Anything, reassignment, inTx).RunAndReturn(func(context.Context, spec.CardAuthorReassignment, ...*sql.Tx) (int64, error) {
		order = append(order, "comments")
		return 1, nil
	}).Once()
	votes.EXPECT().HandOverVotes(mock.Anything, handover, inTx).RunAndReturn(func(context.Context, spec.VoteHandover, ...*sql.Tx) (int64, error) {
		order = append(order, "votes")
		return 1, nil
	}).Once()
	votes.EXPECT().HandOverBudget(mock.Anything, handover, inTx).RunAndReturn(func(context.Context, spec.VoteHandover, ...*sql.Tx) (int64, error) {
		order = append(order, "budget")
		return 1, nil
	}).Once()
	boards.EXPECT().DeleteParticipant(mock.Anything, spec.BoardParticipantKey{BoardID: write.BoardID, ParticipantID: guest}, inTx).RunAndReturn(func(context.Context, spec.BoardParticipantKey, ...*sql.Tx) error {
		order = append(order, "seat")
		return nil
	}).Once()
	boards.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: write.BoardID, UserID: user}, inTx).Return(&model.BoardParticipant{ID: seat, BoardID: write.BoardID, UserID: new(user)}, nil).Once()

	// when
	claimed, err := repo.ClaimGuestParticipant(t.Context(), spec.GuestClaimWrite{BoardWrite: write, GuestParticipantID: guest, UserID: user, UserParticipantID: new(seat)}, inTx...)

	// then everything the guest wrote or spent moves before the guest's seat and its cascade go
	require.NoError(t, err)
	assert.Equal(t, []string{"cards", "comments", "votes", "budget", "reactions", "answers", "seat"}, order)
	assert.Equal(t, seat, claimed.Value.ID)
}
