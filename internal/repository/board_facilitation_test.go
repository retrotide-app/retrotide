package repository

import (
	"context"
	"database/sql"
	"errors"
	"slices"
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

type (
	boardDAOs struct {
		boards      *dao.MockBoardDAO
		columns     *dao.MockBoardColumnDAO
		cards       *dao.MockCardDAO
		votes       *dao.MockCardVoteDAO
		comments    *dao.MockCardCommentDAO
		actionItems *dao.MockActionItemDAO
		reactions   *dao.MockCardReactionDAO
		surveys     *dao.MockSurveyDAO
	}
)

func newBoardDAOs(t *testing.T) (BoardRepository, boardDAOs) {
	t.Helper()

	daos := boardDAOs{
		boards:      dao.NewMockBoardDAO(t),
		columns:     dao.NewMockBoardColumnDAO(t),
		cards:       dao.NewMockCardDAO(t),
		votes:       dao.NewMockCardVoteDAO(t),
		comments:    dao.NewMockCardCommentDAO(t),
		actionItems: dao.NewMockActionItemDAO(t),
		reactions:   dao.NewMockCardReactionDAO(t),
		surveys:     dao.NewMockSurveyDAO(t),
	}

	repo := NewBoardRepo(nil, daos.boards, BoardRepoDeps{
		Columns:     daos.columns,
		Cards:       daos.cards,
		Votes:       daos.votes,
		Comments:    daos.comments,
		ActionItems: daos.actionItems,
		Reactions:   daos.reactions,
		Surveys:     daos.surveys,
	})

	return repo, daos
}

func (d boardDAOs) bumped(write spec.BoardWrite, inTx []*sql.Tx, order *[]string) {
	d.boards.EXPECT().BumpVersion(mock.Anything, write, inTx).RunAndReturn(func(context.Context, spec.BoardWrite, ...*sql.Tx) (int64, error) {
		*order = append(*order, "bump")
		return 42, nil
	}).Once()

	if write.OpID == uuid.Nil {
		return
	}

	d.boards.EXPECT().RecordOp(mock.Anything, spec.NewBoardOp{BoardID: write.BoardID, OpID: write.OpID, Version: 42}, inTx).Return(nil).Once()
}

func TestCloseBoard_SeversAnAnonymousRetroInTheSameComposite(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	cardID := uuid.New()
	var order []string
	d.bumped(write, inTx, &order)
	d.cards.EXPECT().SetPosition(mock.Anything, spec.CardPositionChange{BoardID: write.BoardID, CardID: cardID, Position: "V"}, inTx).RunAndReturn(func(context.Context, spec.CardPositionChange, ...*sql.Tx) error {
		order = append(order, "position")
		return nil
	}).Once()
	d.boards.EXPECT().Close(mock.Anything, spec.BoardKey{OrgID: write.OrgID, BoardID: write.BoardID}, inTx).RunAndReturn(func(context.Context, spec.BoardKey, ...*sql.Tx) (*model.Board, error) {
		order = append(order, "close")
		return &model.Board{ID: write.BoardID}, nil
	}).Once()
	d.surveys.EXPECT().CloseOpen(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "surveys")
		return 1, nil
	}).Once()
	d.reactions.EXPECT().Sever(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "reactions")
		return 4, nil
	}).Once()
	d.surveys.EXPECT().SeverResponses(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "answers")
		return 6, nil
	}).Once()
	d.cards.EXPECT().SeverAuthors(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "cards")
		return 3, nil
	}).Once()
	d.comments.EXPECT().SeverAuthors(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "comments")
		return 1, nil
	}).Once()
	d.votes.EXPECT().DeleteAll(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "votes")
		return 2, nil
	}).Once()
	d.votes.EXPECT().DeleteBudgets(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "budgets")
		return 2, nil
	}).Once()

	// when
	closed, err := repo.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: write, Positions: []spec.PositionChange{{ID: cardID, Position: "V"}}, Sever: true}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(42), closed.Version)
	assert.Equal(t, []string{"bump", "position", "close", "surveys", "reactions", "answers", "cards", "comments", "votes", "budgets"}, order)
}

func TestCloseBoard_ANamedRetroKeepsItsAuthorsAndVotes(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().Close(mock.Anything, spec.BoardKey{OrgID: write.OrgID, BoardID: write.BoardID}, inTx).Return(&model.Board{ID: write.BoardID}, nil).Once()
	d.surveys.EXPECT().CloseOpen(mock.Anything, write.BoardID, inTx).Return(0, nil).Once()

	// when
	_, err := repo.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: write}, inTx...)

	// then the strict mocks prove nothing was severed, while the open surveys still close
	require.NoError(t, err)
}

func TestCloseBoard_AFailedSurveyCloseStopsTheComposite(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	closeErr := errors.New("db down")
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().Close(mock.Anything, mock.Anything, inTx).Return(&model.Board{ID: write.BoardID}, nil).Once()
	d.surveys.EXPECT().CloseOpen(mock.Anything, write.BoardID, inTx).Return(0, closeErr).Once()

	// when
	closed, err := repo.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: write, Sever: true}, inTx...)

	// then nothing is severed after the failure
	assert.Nil(t, closed)
	require.ErrorIs(t, err, closeErr)
}

func TestCloseBoard_AnAlreadyClosedBoardIsNotFound(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().Close(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	closed, err := repo.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: write, Sever: true}, inTx...)

	// then
	assert.Nil(t, closed)
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestRevealCards_ShowsTheCardsThenReadsTheStateItCommitted(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	cardID := uuid.New()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().SetCardsHidden(mock.Anything, spec.BoardCardsHiddenChange{OrgID: write.OrgID, BoardID: write.BoardID, Hidden: false}, inTx).RunAndReturn(func(context.Context, spec.BoardCardsHiddenChange, ...*sql.Tx) (*model.Board, error) {
		order = append(order, "show")
		return &model.Board{ID: write.BoardID}, nil
	}).Once()
	d.cards.EXPECT().SetPosition(mock.Anything, spec.CardPositionChange{BoardID: write.BoardID, CardID: cardID, Position: "V"}, inTx).RunAndReturn(func(context.Context, spec.CardPositionChange, ...*sql.Tx) error {
		order = append(order, "position")
		return nil
	}).Once()
	cards := []model.Card{{ID: cardID, BoardID: write.BoardID, Position: "V"}}
	d.cards.EXPECT().List(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) ([]model.Card, error) {
		order = append(order, "cards")
		return cards, nil
	}).Once()
	d.comments.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardComment{}, nil).Once()
	d.boards.EXPECT().ListParticipants(mock.Anything, write.BoardID, inTx).Return([]model.BoardParticipantProfile{}, nil).Once()
	d.votes.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardVote{}, nil).Once()
	d.reactions.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardReaction{}, nil).Once()

	// when
	revealed, err := repo.RevealCards(t.Context(), spec.CardsRevealWrite{BoardWrite: write, Positions: []spec.PositionChange{{ID: cardID, Position: "V"}}}, inTx...)

	// then the cards are read after their new keys are written
	require.NoError(t, err)
	assert.Equal(t, []string{"bump", "show", "position", "cards"}, order)
	assert.Equal(t, cards, revealed.Value.Cards)
	assert.Equal(t, write.BoardID, revealed.Value.Header.ID)
}

func TestAdvanceStage_EnteringVoteRevealsTheCardsThenReadsTheStateItCommitted(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	cardID := uuid.New()
	var order []string
	d.bumped(write, inTx, &order)
	change := spec.BoardStageChange{OrgID: write.OrgID, BoardID: write.BoardID, From: model.StageWrite, To: model.StageVote}
	d.boards.EXPECT().SetStage(mock.Anything, change, inTx).RunAndReturn(func(context.Context, spec.BoardStageChange, ...*sql.Tx) (*model.Board, error) {
		order = append(order, "stage")
		return &model.Board{ID: write.BoardID, Stage: model.StageVote}, nil
	}).Once()
	d.cards.EXPECT().SetPosition(mock.Anything, spec.CardPositionChange{BoardID: write.BoardID, CardID: cardID, Position: "V"}, inTx).RunAndReturn(func(context.Context, spec.CardPositionChange, ...*sql.Tx) error {
		order = append(order, "position")
		return nil
	}).Once()
	cards := []model.Card{{ID: cardID, BoardID: write.BoardID, Position: "V"}}
	d.cards.EXPECT().List(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) ([]model.Card, error) {
		order = append(order, "cards")
		return cards, nil
	}).Once()
	d.comments.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardComment{}, nil).Once()
	d.boards.EXPECT().ListParticipants(mock.Anything, write.BoardID, inTx).Return([]model.BoardParticipantProfile{}, nil).Once()
	d.votes.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardVote{}, nil).Once()
	d.reactions.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardReaction{}, nil).Once()

	// when
	moved, err := repo.AdvanceStage(t.Context(), spec.BoardStageWrite{
		BoardWrite: write,
		From:       model.StageWrite,
		To:         model.StageVote,
		Positions:  []spec.PositionChange{{ID: cardID, Position: "V"}},
		Reveal:     true,
	}, inTx...)

	// then the stage, the keys and the read all sit in one composite, in that order
	require.NoError(t, err)
	assert.Equal(t, int64(42), moved.Version)
	assert.Equal(t, []string{"bump", "stage", "position", "cards"}, order)
	assert.Equal(t, model.StageVote, moved.Value.Header.Stage)
	assert.Equal(t, cards, moved.Value.Cards)
}

func TestAdvanceStage_AStepThatRevealsNothingReadsNothing(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	change := spec.BoardStageChange{OrgID: write.OrgID, BoardID: write.BoardID, From: model.StageVote, To: model.StageDiscuss}
	d.boards.EXPECT().SetStage(mock.Anything, change, inTx).Return(&model.Board{ID: write.BoardID, Stage: model.StageDiscuss}, nil).Once()

	// when
	moved, err := repo.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: write, From: model.StageVote, To: model.StageDiscuss}, inTx...)

	// then the strict mocks prove no card, comment, participant or vote was read
	require.NoError(t, err)
	assert.Equal(t, model.StageDiscuss, moved.Value.Header.Stage)
	assert.Empty(t, moved.Value.Cards)
}

func TestAdvanceStage_AStageThatMovedIsRefusedWithItsSentinel(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().SetStage(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrStageMoved).Once()

	// when
	moved, err := repo.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: write, From: model.StageVote, To: model.StageDiscuss}, inTx...)

	// then
	assert.Nil(t, moved)
	require.ErrorIs(t, err, dao.ErrStageMoved)
}

func TestHideCards_HidesTheCardsAndReadsThemForTheTally(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().SetCardsHidden(mock.Anything, spec.BoardCardsHiddenChange{OrgID: write.OrgID, BoardID: write.BoardID, Hidden: true}, inTx).Return(&model.Board{ID: write.BoardID, CardsHidden: true}, nil).Once()
	cards := []model.Card{{ID: uuid.New(), BoardID: write.BoardID}}
	d.cards.EXPECT().List(mock.Anything, write.BoardID, inTx).Return(cards, nil).Once()

	// when
	hidden, err := repo.HideCards(t.Context(), write, inTx...)

	// then
	require.NoError(t, err)
	assert.True(t, hidden.Value.Header.CardsHidden)
	assert.Equal(t, cards, hidden.Value.Cards)
}

func TestSetFacilitationState_RevealingAColumnWritesTheStateThenTheKeysThenReadsTheState(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	columnID := uuid.New()
	cardID := uuid.New()
	var order []string
	d.bumped(write, inTx, &order)
	change := spec.BoardFacilitationChange{OrgID: write.OrgID, BoardID: write.BoardID, RevealedColumnIDs: []uuid.UUID{columnID}}
	d.boards.EXPECT().SetFacilitation(mock.Anything, change, inTx).RunAndReturn(func(context.Context, spec.BoardFacilitationChange, ...*sql.Tx) (*model.Board, error) {
		order = append(order, "state")
		return &model.Board{ID: write.BoardID, CardsHidden: true, RevealedColumnIDs: []uuid.UUID{columnID}}, nil
	}).Once()
	d.cards.EXPECT().SetPosition(mock.Anything, spec.CardPositionChange{BoardID: write.BoardID, CardID: cardID, Position: "V"}, inTx).RunAndReturn(func(context.Context, spec.CardPositionChange, ...*sql.Tx) error {
		order = append(order, "position")
		return nil
	}).Once()
	cards := []model.Card{{ID: cardID, BoardID: write.BoardID, ColumnID: columnID, Position: "V"}}
	d.cards.EXPECT().List(mock.Anything, write.BoardID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) ([]model.Card, error) {
		order = append(order, "cards")
		return cards, nil
	}).Once()
	d.comments.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardComment{}, nil).Once()
	d.boards.EXPECT().ListParticipants(mock.Anything, write.BoardID, inTx).Return([]model.BoardParticipantProfile{}, nil).Once()
	d.votes.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardVote{}, nil).Once()
	d.reactions.EXPECT().List(mock.Anything, write.BoardID, inTx).Return([]model.CardReaction{}, nil).Once()

	// when
	revealed, err := repo.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{
		BoardWrite:        write,
		RevealedColumnIDs: []uuid.UUID{columnID},
		RevealedColumnID:  new(columnID),
		Positions:         []spec.PositionChange{{ID: cardID, Position: "V"}},
	}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(42), revealed.Version)
	assert.Equal(t, []string{"bump", "state", "position", "cards"}, order)
	assert.Equal(t, cards, revealed.Value.Cards)
	assert.True(t, slices.Contains(revealed.Value.Header.RevealedColumnIDs, columnID))
}

func TestSetFacilitationState_AChangeThatRevealsNothingReadsNothing(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	columnID := uuid.New()
	var order []string
	d.bumped(write, inTx, &order)
	change := spec.BoardFacilitationChange{OrgID: write.OrgID, BoardID: write.BoardID, HiddenColumnIDs: []uuid.UUID{columnID}}
	d.boards.EXPECT().SetFacilitation(mock.Anything, change, inTx).Return(&model.Board{ID: write.BoardID, HiddenColumnIDs: []uuid.UUID{columnID}}, nil).Once()

	// when
	hidden, err := repo.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{BoardWrite: write, HiddenColumnIDs: []uuid.UUID{columnID}}, inTx...)

	// then the strict mocks prove no card, comment, participant or vote was read
	require.NoError(t, err)
	assert.True(t, slices.Contains(hidden.Value.Header.HiddenColumnIDs, columnID))
	assert.Nil(t, hidden.Value.Cards)
}

func TestLockBoardAtTimerEnd_LocksWithoutRecordingAnOp(t *testing.T) {
	// given a firing, which carries no op
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	write.OpID = uuid.Nil
	endsAt := time.Now().UTC()
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().LockAtTimerEnd(mock.Anything, spec.BoardTimerLock{OrgID: write.OrgID, BoardID: write.BoardID, EndsAt: endsAt}, inTx).Return(&model.Board{ID: write.BoardID, LockedAt: new(endsAt)}, nil).Once()

	// when
	locked, err := repo.LockBoardAtTimerEnd(t.Context(), spec.BoardTimerLockWrite{BoardWrite: write, EndsAt: endsAt}, inTx...)

	// then the strict mock refuses any RecordOp
	require.NoError(t, err)
	assert.Equal(t, int64(42), locked.Version)
	assert.True(t, locked.Value.Locked())
}

func TestLockBoardAtTimerEnd_AMovedTimerIsRefusedWithItsSentinel(t *testing.T) {
	// given
	repo, d := newBoardDAOs(t)
	write, inTx := boardWriteFixture()
	write.OpID = uuid.Nil
	var order []string
	d.bumped(write, inTx, &order)
	d.boards.EXPECT().LockAtTimerEnd(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrTimerMoved).Once()

	// when
	locked, err := repo.LockBoardAtTimerEnd(t.Context(), spec.BoardTimerLockWrite{BoardWrite: write, EndsAt: time.Now()}, inTx...)

	// then
	assert.Nil(t, locked)
	require.ErrorIs(t, err, dao.ErrTimerMoved)
}

func TestSetColumnVoteLimit_SetsTheLimitInsideABoardWrite(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	columns := dao.NewMockBoardColumnDAO(t)
	repo := NewBoardColumnRepo(nil, columns, boards, dao.NewMockCardVoteDAO(t))
	write, inTx := boardWriteFixture()
	columnID := uuid.New()
	var order []string
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).RunAndReturn(func(context.Context, spec.BoardWrite, ...*sql.Tx) (int64, error) {
		order = append(order, "bump")
		return 42, nil
	}).Once()
	boards.EXPECT().RecordOp(mock.Anything, spec.NewBoardOp{BoardID: write.BoardID, OpID: write.OpID, Version: 42}, inTx).Return(nil).Once()
	columns.EXPECT().SetMaxVotes(mock.Anything, spec.BoardColumnMaxVotes{BoardID: write.BoardID, ColumnID: columnID, MaxVotes: new(3)}, inTx).RunAndReturn(func(_ context.Context, s spec.BoardColumnMaxVotes, _ ...*sql.Tx) (*model.BoardColumn, error) {
		order = append(order, "limit")
		return &model.BoardColumn{ID: s.ColumnID, BoardID: s.BoardID, MaxVotes: s.MaxVotes}, nil
	}).Once()

	// when
	limited, err := repo.SetColumnVoteLimit(t.Context(), spec.ColumnVoteLimitWrite{BoardWrite: write, ColumnID: columnID, MaxVotes: new(3)}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"bump", "limit"}, order)
	assert.Equal(t, int64(42), limited.Version)
	assert.Equal(t, new(3), limited.Value.MaxVotes)
}
