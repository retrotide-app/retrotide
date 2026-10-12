package dao_test

import (
	"slices"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f boardFixture) header(t *testing.T, boardID uuid.UUID) *model.BoardHeader {
	t.Helper()

	header, err := f.repos.Board.GetHeader(t.Context(), boardID)
	require.NoError(t, err)

	return header
}

func (f boardFixture) armAutoLock(t *testing.T, boardID uuid.UUID, endsAt time.Time) {
	t.Helper()

	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET timer_auto_lock = TRUE, timer_duration_ms = 60000, timer_ends_at = $2 WHERE id = $1`, boardID, endsAt)
	require.NoError(t, err)
}

func (f boardFixture) spotlight(t *testing.T, spotlight model.BoardSpotlight) {
	t.Helper()

	_, err := f.repos.Board.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{BoardWrite: f.write(t), Spotlight: spotlight})
	require.NoError(t, err)
}

func TestBoard_SetFacilitationStateStoresTheColumnsAndTheSpotlightInOneVersion(t *testing.T) {
	// given a board with a card in its first column
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	before := f.version(t)
	spotlight := model.BoardSpotlight{PresentingCardID: new(card.ID), HighlightedCardID: new(card.ID), FocusedColumnID: new(f.column(0))}

	// when the first column is revealed, the second hidden and the card spotlit
	written, err := f.repos.Board.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{
		BoardWrite:        f.write(t),
		RevealedColumnIDs: []uuid.UUID{f.column(0)},
		HiddenColumnIDs:   []uuid.UUID{f.column(1)},
		Spotlight:         spotlight,
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, before+1, written.Version)
	assert.Nil(t, written.Value.Cards, "a write that reveals no column reads back only the board")
	header := f.header(t, f.boardID())
	assert.Equal(t, []uuid.UUID{f.column(0)}, header.RevealedColumnIDs)
	assert.Equal(t, []uuid.UUID{f.column(1)}, header.HiddenColumnIDs)
	assert.Equal(t, spotlight, header.Spotlight)
	assert.True(t, slices.Contains(header.RevealedColumnIDs, f.column(0)))
	assert.False(t, header.CardsHiddenIn(f.column(0)))
	assert.True(t, header.CardsHiddenIn(f.column(1)))
	assert.True(t, slices.Contains(header.HiddenColumnIDs, f.column(1)))
	assert.Equal(t, written.Value.Header.Board.HiddenColumnIDs, header.HiddenColumnIDs)
}

func TestBoard_SetFacilitationStateRevealingAColumnReadsBackItsCardsAndRewritesTheirKeys(t *testing.T) {
	// given two cards in the first column
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(0), "a1")

	// when the column is revealed with its keys shuffled
	written, err := f.repos.Board.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{
		BoardWrite:        f.write(t),
		RevealedColumnIDs: []uuid.UUID{f.column(0)},
		RevealedColumnID:  new(f.column(0)),
		Positions:         []spec.PositionChange{{ID: first.ID, Position: "a5"}, {ID: second.ID, Position: "a2"}},
	})

	// then
	require.NoError(t, err)
	require.Len(t, written.Value.Cards, 2)
	assert.Equal(t, second.ID, written.Value.Cards[0].ID)
	assert.Len(t, written.Value.Participants, 1)
	assert.Equal(t, "a5", f.cardPosition(t, first.ID))
}

func TestBoard_HidingCardsOrLeavingWriteOrClosingClearsTheFacilitationState(t *testing.T) {
	// given a revealed column, a hidden column and a spotlit card
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	_, err := f.repos.Board.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{
		BoardWrite:        f.write(t),
		RevealedColumnIDs: []uuid.UUID{f.column(0)},
		HiddenColumnIDs:   []uuid.UUID{f.column(1)},
		Spotlight:         model.BoardSpotlight{PresentingCardID: new(card.ID), HighlightedCardID: new(card.ID), FocusedColumnID: new(f.column(0))},
	})
	require.NoError(t, err)

	// when the cards are hidden again
	_, err = f.repos.Board.HideCards(t.Context(), f.write(t))

	// then no column stays revealed and no hidden card stays spotlit
	require.NoError(t, err)
	hidden := f.header(t, f.boardID())
	assert.Empty(t, hidden.RevealedColumnIDs)
	assert.Nil(t, hidden.Spotlight.PresentingCardID)
	assert.Nil(t, hidden.Spotlight.HighlightedCardID)
	assert.Equal(t, new(f.column(0)), hidden.Spotlight.FocusedColumnID)
	assert.Equal(t, []uuid.UUID{f.column(1)}, hidden.HiddenColumnIDs)

	// when a column is revealed and the board leaves Write
	_, err = f.repos.Board.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{BoardWrite: f.write(t), RevealedColumnIDs: []uuid.UUID{f.column(0)}, HiddenColumnIDs: []uuid.UUID{f.column(1)}})
	require.NoError(t, err)
	_, err = f.repos.Board.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: f.write(t), From: model.StageWrite, To: model.StageVote})

	// then
	require.NoError(t, err)
	assert.Empty(t, f.header(t, f.boardID()).RevealedColumnIDs)

	// when the board closes
	_, err = f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t)})

	// then
	require.NoError(t, err)
	closed := f.header(t, f.boardID())
	assert.Empty(t, closed.HiddenColumnIDs)
	assert.Equal(t, model.BoardSpotlight{}, closed.Spotlight)
}

func TestBoard_SetFacilitationStateNamingACardOfAnotherBoardIsRefused(t *testing.T) {
	// given a card on another board
	f := newBoardFixture(t)
	other := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	added, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
		OrgID: f.orgID(), BoardID: other.Header.ID, Version: other.Header.Version, OpID: uuid.New(),
		CardID:              uuid.New(),
		ColumnID:            other.Columns[0].ID,
		AuthorParticipantID: new(other.Participants[0].ID),
		Body:                "Elsewhere",
		Position:            "a0",
	})
	require.NoError(t, err)
	before := f.version(t)

	// when
	_, err = f.repos.Board.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{BoardWrite: f.write(t), Spotlight: model.BoardSpotlight{PresentingCardID: new(added.Value.ID)}})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, before, f.version(t))
}

func TestBoard_DeletingASpotlitCardOrFocusedColumnEndsItsSpotlight(t *testing.T) {
	// given a presented card in the first column and a focused second column
	f := newBoardFixture(t)
	presented := f.addCard(t, f.column(0), "a0")
	f.spotlight(t, model.BoardSpotlight{PresentingCardID: new(presented.ID), HighlightedCardID: new(presented.ID), FocusedColumnID: new(f.column(1))})

	// when the card is deleted
	_, err := f.repos.Card.Delete(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: presented.ID})

	// then
	require.NoError(t, err)
	afterCard := f.header(t, f.boardID())
	assert.Nil(t, afterCard.Spotlight.PresentingCardID)
	assert.Nil(t, afterCard.Spotlight.HighlightedCardID)
	assert.Equal(t, new(f.column(1)), afterCard.Spotlight.FocusedColumnID)

	// when the focused column is deleted
	_, err = f.repos.BoardColumn.Delete(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(1)})

	// then
	require.NoError(t, err)
	assert.Nil(t, f.header(t, f.boardID()).Spotlight.FocusedColumnID)
}

func TestBoard_LockAtTimerEndLocksOnlyWhileTheTimerStillEndsWhenItWasScheduled(t *testing.T) {
	// given a board whose running timer locks it when it ends
	f := newBoardFixture(t)
	endsAt := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	f.armAutoLock(t, f.boardID(), endsAt)
	before := f.version(t)

	// when the firing names an end the timer no longer has
	_, movedErr := f.repos.Board.LockBoardAtTimerEnd(t.Context(), spec.BoardTimerLockWrite{
		BoardWrite: spec.BoardWrite{OrgID: f.orgID(), BoardID: f.boardID(), Version: before},
		EndsAt:     endsAt.Add(time.Minute),
	})

	// then the board stays open and no version is burned
	require.ErrorIs(t, movedErr, dao.ErrTimerMoved)
	assert.Equal(t, before, f.version(t))
	assert.False(t, f.header(t, f.boardID()).Locked())

	// when the firing names the timer's end
	locked, err := f.repos.Board.LockBoardAtTimerEnd(t.Context(), spec.BoardTimerLockWrite{
		BoardWrite: spec.BoardWrite{OrgID: f.orgID(), BoardID: f.boardID(), Version: before},
		EndsAt:     endsAt,
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, before+1, locked.Version)
	require.NotNil(t, locked.Value.LockedAt)
	assert.True(t, endsAt.Equal(*locked.Value.LockedAt), "the board locks at the moment its timer ended")
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM board_ops WHERE board_id = $1 AND version = $2`, f.boardID(), locked.Version), "a timer firing records no op")

	// when it fires again
	_, againErr := f.repos.Board.LockAtTimerEnd(t.Context(), spec.BoardTimerLock{OrgID: f.orgID(), BoardID: f.boardID(), EndsAt: endsAt})

	// then
	require.ErrorIs(t, againErr, dao.ErrTimerMoved)
}

func TestBoard_LockAtTimerEndOnABoardWithAutoLockOffIsRefused(t *testing.T) {
	// given a running timer on a board that does not lock at its end
	f := newBoardFixture(t)
	endsAt := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	f.armAutoLock(t, f.boardID(), endsAt)
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET timer_auto_lock = FALSE WHERE id = $1`, f.boardID())
	require.NoError(t, err)

	// when
	_, err = f.repos.Board.LockAtTimerEnd(t.Context(), spec.BoardTimerLock{OrgID: f.orgID(), BoardID: f.boardID(), EndsAt: endsAt})

	// then
	require.ErrorIs(t, err, dao.ErrTimerMoved)
	assert.False(t, f.header(t, f.boardID()).Locked())
}

func TestBoard_ListAutoLockTimersLeavesOutBoardsThatCanNoLongerLock(t *testing.T) {
	// given six boards with running auto-lock timers, five of which can no longer lock
	f := newBoardFixture(t)
	endsAt := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	open := f.boardID()
	locked := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID).Header.ID
	closed := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID).Header.ID
	deleted := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID).Header.ID
	archived := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID).Header.ID
	gone := daotest.CreateOrg(t, f.repos, daotest.CreateUser(t, f.repos).ID)
	orphan := daotest.CreateBoard(t, f.repos, gone.Team, gone.Owner.UserID).Header.ID
	off := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID).Header.ID
	for _, id := range []uuid.UUID{open, locked, closed, deleted, archived, orphan} {
		f.armAutoLock(t, id, endsAt)
	}

	conn := f.repos.DB()
	_, err := conn.ExecContext(t.Context(), `UPDATE boards SET locked_at = NOW() WHERE id = $1`, locked)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), `UPDATE boards SET closed_at = NOW() WHERE id = $1`, closed)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), `UPDATE boards SET archived_at = NOW() WHERE id = $1`, archived)
	require.NoError(t, err)
	f.deleteBoard(t, deleted)
	_, err = conn.ExecContext(t.Context(), `UPDATE organisations SET deleted_at = NOW() WHERE id = $1`, gone.Organisation.ID)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), `UPDATE boards SET timer_duration_ms = 60000, timer_ends_at = $2 WHERE id = $1`, off, endsAt)
	require.NoError(t, err)

	// when
	timers, err := f.repos.Board.ListAutoLockTimers(t.Context())

	// then
	require.NoError(t, err)
	require.Len(t, timers, 1)
	assert.Equal(t, open, timers[0].BoardID)
	assert.Equal(t, f.orgID(), timers[0].OrgID)
	assert.True(t, endsAt.Equal(timers[0].EndsAt))
}

func TestColumn_SetMaxVotesSetsAndClearsTheColumnsVoteLimit(t *testing.T) {
	// given
	f := newBoardFixture(t)
	key := spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)}

	// when a limit is set
	limited, err := f.repos.BoardColumn.SetMaxVotes(t.Context(), spec.BoardColumnMaxVotes{BoardID: key.BoardID, ColumnID: key.ColumnID, MaxVotes: new(3)})

	// then
	require.NoError(t, err)
	assert.Equal(t, new(3), limited.MaxVotes)
	read, err := f.repos.BoardColumn.GetByID(t.Context(), key)
	require.NoError(t, err)
	assert.Equal(t, new(3), read.MaxVotes)

	// when it is cleared
	cleared, err := f.repos.BoardColumn.SetMaxVotes(t.Context(), spec.BoardColumnMaxVotes{BoardID: key.BoardID, ColumnID: key.ColumnID})

	// then
	require.NoError(t, err)
	assert.Nil(t, cleared.MaxVotes)
}

func TestColumn_SetMaxVotesOutsideItsRangeOrOnAnotherBoardIsRefused(t *testing.T) {
	// given
	f := newBoardFixture(t)
	other := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)

	// when
	_, rangeErr := f.repos.BoardColumn.SetMaxVotes(t.Context(), spec.BoardColumnMaxVotes{BoardID: f.boardID(), ColumnID: f.column(0), MaxVotes: new(100)})
	_, otherErr := f.repos.BoardColumn.SetMaxVotes(t.Context(), spec.BoardColumnMaxVotes{BoardID: other.Header.ID, ColumnID: f.column(0), MaxVotes: new(3)})

	// then
	require.ErrorContains(t, rangeErr, "board_columns_max_votes_check")
	require.ErrorIs(t, otherErr, dao.ErrNotFound)
}

func TestCardVote_CheckColumnBudgetCountsOnlyTheVotersVotesInThatColumn(t *testing.T) {
	// given two votes in the first column and one in the second
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(1), "a0")
	_, err := f.vote(t, first.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)
	_, err = f.vote(t, first.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)
	_, err = f.vote(t, second.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)
	check := spec.ColumnVoteBudgetCheck{BoardID: f.boardID(), ColumnID: f.column(0), ParticipantID: f.facilitator()}

	// when
	check.MaxVotes = 2
	spentErr := f.repos.CardVote.CheckColumnBudget(t.Context(), check)
	check.MaxVotes = 3
	roomErr := f.repos.CardVote.CheckColumnBudget(t.Context(), check)
	check.ColumnID = f.column(1)
	check.MaxVotes = 2
	otherErr := f.repos.CardVote.CheckColumnBudget(t.Context(), check)

	// then
	require.ErrorIs(t, spentErr, dao.ErrColumnVoteBudgetSpent)
	require.NoError(t, roomErr)
	require.NoError(t, otherErr)
}

func TestCardVote_ListedVotesCarryTheirCardsColumn(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(1), "a0")
	_, err := f.vote(t, card.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)

	// when
	votes, err := f.repos.CardVote.List(t.Context(), f.boardID())
	mine, mineErr := f.repos.CardVote.ListForParticipant(t.Context(), spec.BoardParticipantKey{BoardID: f.boardID(), ParticipantID: f.facilitator()})

	// then
	require.NoError(t, err)
	require.NoError(t, mineErr)
	require.Len(t, votes, 1)
	require.Len(t, mine, 1)
	assert.Equal(t, f.column(1), votes[0].CardColumnID)
	assert.Equal(t, f.column(1), mine[0].CardColumnID)
}

func TestCardVote_DeleteColumnRemovesOnlyThatColumnsVotes(t *testing.T) {
	// given a vote in each column
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(1), "a0")
	_, err := f.vote(t, first.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)
	_, err = f.vote(t, second.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)

	// when
	deleted, err := f.repos.CardVote.DeleteColumn(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	votes, err := f.repos.CardVote.List(t.Context(), f.boardID())
	require.NoError(t, err)
	require.Len(t, votes, 1)
	assert.Equal(t, second.ID, votes[0].CardID)
}

func TestCard_ResetColumnVoteTotalsZeroesOnlyThatColumnsTotals(t *testing.T) {
	// given a vote in each column
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(1), "a0")
	_, err := f.vote(t, first.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)
	_, err = f.vote(t, second.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)

	// when
	reset, err := f.repos.Card.ResetColumnVoteTotals(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), reset)
	assert.Zero(t, f.voteTotal(t, first.ID))
	assert.Equal(t, 1, f.voteTotal(t, second.ID))
}
