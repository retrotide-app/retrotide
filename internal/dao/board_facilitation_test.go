package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	beforeStagesMigration = 20261008213553
)

func (f boardFixture) cardPosition(t *testing.T, cardID uuid.UUID) string {
	t.Helper()

	card, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: cardID})
	require.NoError(t, err)

	return card.Position
}

func (f boardFixture) touch(t *testing.T, boardID uuid.UUID, at time.Time) {
	t.Helper()

	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET updated_at = $2 WHERE id = $1`, boardID, at)
	require.NoError(t, err)
}

func TestBoard_RevealCardsShowsTheCardsAndRewritesTheirKeysInOneVersion(t *testing.T) {
	// given a board in its writing phase with two cards in typing order
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(0), "a1")
	before := f.version(t)

	// when the keys are shuffled into the opposite order
	revealed, err := f.repos.Board.RevealCards(t.Context(), spec.CardsRevealWrite{
		BoardWrite: f.write(t),
		Positions:  []spec.PositionChange{{ID: first.ID, Position: "a5"}, {ID: second.ID, Position: "a2"}},
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, before+1, revealed.Version)
	assert.False(t, revealed.Value.Header.CardsHidden)
	require.Len(t, revealed.Value.Cards, 2)
	assert.Equal(t, second.ID, revealed.Value.Cards[0].ID, "the state read back follows the new keys")
	assert.Equal(t, "a5", f.cardPosition(t, first.ID))
	assert.Equal(t, "a2", f.cardPosition(t, second.ID))
	assert.Len(t, revealed.Value.Participants, 1)
}

func TestBoard_ANewBoardStartsInTheWriteStage(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	header, err := f.repos.Board.GetHeader(t.Context(), f.boardID())

	// then
	require.NoError(t, err)
	assert.Equal(t, model.StageWrite, header.Stage)
}

func TestBoard_AdvanceStageIntoVoteShowsTheCardsAndRewritesTheirKeysInOneVersion(t *testing.T) {
	// given a board in its Write stage with its cards hidden
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(0), "a1")
	before := f.version(t)

	// when
	moved, err := f.repos.Board.AdvanceStage(t.Context(), spec.BoardStageWrite{
		BoardWrite: f.write(t),
		From:       model.StageWrite,
		To:         model.StageVote,
		Positions:  []spec.PositionChange{{ID: first.ID, Position: "a5"}, {ID: second.ID, Position: "a2"}},
		Reveal:     true,
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, before+1, moved.Version)
	assert.Equal(t, model.StageVote, moved.Value.Header.Stage)
	assert.False(t, moved.Value.Header.CardsHidden, "leaving Write shows every card")
	require.Len(t, moved.Value.Cards, 2)
	assert.Equal(t, second.ID, moved.Value.Cards[0].ID, "the state read back follows the new keys")
	assert.Len(t, moved.Value.Participants, 1)
}

func TestBoard_AdvanceStageWalksEveryStepUpToActions(t *testing.T) {
	// given
	f := newBoardFixture(t)
	steps := []model.Stage{model.StageVote, model.StageDiscuss, model.StageActions}
	from := model.StageWrite

	for _, to := range steps {
		// when
		moved, err := f.repos.Board.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: f.write(t), From: from, To: to})

		// then
		require.NoError(t, err)
		assert.Equal(t, to, moved.Value.Header.Stage)
		assert.False(t, moved.Value.Header.CardsHidden)
		from = to
	}
}

func TestBoard_AdvanceStageFromAStageTheBoardIsNotInIsRefusedAndBurnsNoVersion(t *testing.T) {
	// given a board still in its Write stage
	f := newBoardFixture(t)
	before := f.version(t)

	// when the write expects the board to be voting
	_, err := f.repos.Board.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: f.write(t), From: model.StageVote, To: model.StageDiscuss})

	// then
	require.ErrorIs(t, err, dao.ErrStageMoved)
	assert.Equal(t, before, f.version(t))
	header, err := f.repos.Board.GetHeader(t.Context(), f.boardID())
	require.NoError(t, err)
	assert.Equal(t, model.StageWrite, header.Stage)
	assert.True(t, header.CardsHidden)
}

func TestBoard_AdvanceStageOnAClosedBoardIsRefused(t *testing.T) {
	// given
	f := newBoardFixture(t)
	_, err := f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t)})
	require.NoError(t, err)

	// when
	_, err = f.repos.Board.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: f.write(t), From: model.StageEnd, To: model.StageEnd})

	// then
	require.ErrorIs(t, err, dao.ErrStageMoved)
}

func TestBoard_CloseBoardMovesTheBoardToItsEndStage(t *testing.T) {
	// given a board in its Discuss stage
	f := newBoardFixture(t)
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET stage = 'discuss', cards_hidden = FALSE WHERE id = $1`, f.boardID())
	require.NoError(t, err)

	// when
	closed, err := f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t)})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.StageEnd, closed.Value.Stage)
	assert.Equal(t, 1, countWhere(t, f.repos, `SELECT COUNT(*) FROM boards WHERE id = $1 AND stage = 'end'`, f.boardID()))
}

func TestBoard_AClosedBoardReadsAsEndedWhateverItsStoredStage(t *testing.T) {
	// given a board an older release closed without moving its stage
	f := newBoardFixture(t)
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET closed_at = NOW(), cards_hidden = FALSE WHERE id = $1`, f.boardID())
	require.NoError(t, err)

	// when
	header, err := f.repos.Board.GetHeader(t.Context(), f.boardID())

	// then
	require.NoError(t, err)
	assert.Equal(t, model.StageEnd, header.Stage)
}

func TestBoard_TheStageMigrationPlacesEveryExistingBoard(t *testing.T) {
	// given three boards from before stages existed: one writing, one revealed and one ended
	f := newBoardFixture(t)
	revealed := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	ended := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	conn := f.repos.DB()
	require.NoError(t, db.MigrateDownTo(conn, beforeStagesMigration))
	_, err := conn.ExecContext(t.Context(), `UPDATE boards SET cards_hidden = FALSE WHERE id = $1`, revealed.Header.ID)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), `UPDATE boards SET cards_hidden = FALSE, closed_at = NOW() WHERE id = $1`, ended.Header.ID)
	require.NoError(t, err)

	// when
	require.NoError(t, db.Migrate(conn))

	// then
	want := map[uuid.UUID]string{f.boardID(): "write", revealed.Header.ID: "discuss", ended.Header.ID: "end"}
	for id, stage := range want {
		assert.Equal(t, 1, countWhere(t, f.repos, `SELECT COUNT(*) FROM boards WHERE id = $1 AND stage = $2`, id, stage), "board %s", id)
	}
}

func TestBoard_HideCardsHidesThemAgainAndReadsEveryCard(t *testing.T) {
	// given a shown board
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	_, err := f.repos.Board.RevealCards(t.Context(), spec.CardsRevealWrite{BoardWrite: f.write(t)})
	require.NoError(t, err)

	// when
	hidden, err := f.repos.Board.HideCards(t.Context(), f.write(t))

	// then
	require.NoError(t, err)
	assert.True(t, hidden.Value.Header.CardsHidden)
	require.Len(t, hidden.Value.Cards, 1)
	assert.Equal(t, card.ID, hidden.Value.Cards[0].ID)
}

func TestBoard_CloseBoardSeversAuthorshipAndVotesButKeepsTheTotals(t *testing.T) {
	// given a card and a comment by the facilitator, voted for twice
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	comment := f.addComment(t, card.ID, "Agreed")
	_, err := f.vote(t, card.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)
	_, err = f.vote(t, card.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)

	// when
	closed, err := f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t), Sever: true})

	// then nothing links anyone to a card, a comment or a vote, and the totals stay
	require.NoError(t, err)
	assert.True(t, closed.Value.Closed())
	assert.False(t, closed.Value.CardsHidden, "no card stays hidden once nobody owns it")
	kept, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: card.ID})
	require.NoError(t, err)
	assert.Nil(t, kept.AuthorParticipantID)
	assert.Equal(t, 2, kept.VoteTotal)
	keptComment, err := f.repos.CardComment.GetByID(t.Context(), spec.CardCommentRef{BoardID: f.boardID(), CommentID: comment.ID})
	require.NoError(t, err)
	assert.Nil(t, keptComment.AuthorParticipantID)
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_votes WHERE board_id = $1`, f.boardID()))
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM board_vote_budgets WHERE board_id = $1`, f.boardID()))
}

func TestBoard_CloseBoardWithoutSeveringKeepsEveryLink(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	_, err := f.vote(t, card.ID, f.facilitator(), 6, 0)
	require.NoError(t, err)

	// when
	_, err = f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t)})

	// then
	require.NoError(t, err)
	kept, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: card.ID})
	require.NoError(t, err)
	assert.Equal(t, new(f.facilitator()), kept.AuthorParticipantID)
	assert.Equal(t, 1, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_votes WHERE board_id = $1`, f.boardID()))
}

func TestBoard_CloseBoardTwiceIsNotFoundAndBurnsNoVersion(t *testing.T) {
	// given
	f := newBoardFixture(t)
	_, err := f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t)})
	require.NoError(t, err)
	before := f.version(t)

	// when
	_, err = f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t), Sever: true})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, before, f.version(t))
}

func TestBoard_CloseBoardClearsTheTimer(t *testing.T) {
	// given a running timer
	f := newBoardFixture(t)
	ends := time.Now().Add(time.Minute).UTC().Truncate(time.Millisecond)
	_, err := f.repos.Board.SetBoardTimer(t.Context(), spec.BoardTimerWrite{BoardWrite: f.write(t), Timer: model.BoardTimer{DurationMS: new(60000), EndsAt: &ends}})
	require.NoError(t, err)

	// when
	closed, err := f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t)})

	// then
	require.NoError(t, err)
	assert.False(t, closed.Value.Timer().Set())
}

func TestBoard_SetBoardTimerStoresEachState(t *testing.T) {
	ends := time.Date(2026, 10, 4, 12, 5, 0, 0, time.UTC)

	tests := []struct {
		name  string
		timer model.BoardTimer
	}{
		{name: "running", timer: model.BoardTimer{DurationMS: new(300000), EndsAt: &ends}},
		{name: "paused", timer: model.BoardTimer{DurationMS: new(300000), PausedRemainingMS: new(42000)}},
		{name: "stopped", timer: model.BoardTimer{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newBoardFixture(t)

			// when
			written, err := f.repos.Board.SetBoardTimer(t.Context(), spec.BoardTimerWrite{BoardWrite: f.write(t), Timer: tc.timer})

			// then
			require.NoError(t, err)
			got := written.Value.Timer()
			assert.Equal(t, tc.timer.DurationMS, got.DurationMS)
			assert.Equal(t, tc.timer.PausedRemainingMS, got.PausedRemainingMS)
			if tc.timer.EndsAt == nil {
				assert.Nil(t, got.EndsAt)
				return
			}

			require.NotNil(t, got.EndsAt)
			assert.True(t, tc.timer.EndsAt.Equal(*got.EndsAt))
		})
	}
}

func TestBoard_SetBoardTimerRefusesARunningAndPausedTimerAtOnce(t *testing.T) {
	// given
	f := newBoardFixture(t)
	ends := time.Now().Add(time.Minute)
	before := f.version(t)

	// when
	_, err := f.repos.Board.SetBoardTimer(t.Context(), spec.BoardTimerWrite{BoardWrite: f.write(t), Timer: model.BoardTimer{DurationMS: new(60000), EndsAt: &ends, PausedRemainingMS: new(1000)}})

	// then the boards_timer_state check rolls the whole command back
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boards_timer_state")
	assert.Equal(t, before, f.version(t))
}

func TestBoard_SettingsCarryTheLocksAndTheTimerSound(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when the board is locked
	locked, err := f.repos.Board.SetBoardSettings(t.Context(), spec.BoardSettingsWrite{BoardWrite: f.write(t), Patch: spec.BoardSettingsPatch{
		CardsLocked:    new(true),
		MovesLocked:    new(true),
		BoardLocked:    new(true),
		TimerPlaySound: new(false),
	}})

	// then
	require.NoError(t, err)
	assert.True(t, locked.Value.CardsLocked)
	assert.True(t, locked.Value.MovesLocked)
	assert.True(t, locked.Value.Locked())
	assert.False(t, locked.Value.TimerPlaySound)

	// when it is locked again, then unlocked
	relocked, err := f.repos.Board.SetBoardSettings(t.Context(), spec.BoardSettingsWrite{BoardWrite: f.write(t), Patch: spec.BoardSettingsPatch{BoardLocked: new(true)}})
	require.NoError(t, err)
	unlocked, err := f.repos.Board.SetBoardSettings(t.Context(), spec.BoardSettingsWrite{BoardWrite: f.write(t), Patch: spec.BoardSettingsPatch{BoardLocked: new(false)}})

	// then a second lock keeps when it was first locked, and a field left out keeps its value
	require.NoError(t, err)
	assert.True(t, locked.Value.LockedAt.Equal(*relocked.Value.LockedAt))
	assert.False(t, unlocked.Value.Locked())
	assert.True(t, unlocked.Value.CardsLocked)
	assert.False(t, unlocked.Value.TimerPlaySound)
}

func TestBoard_ListIdleFindsOnlyOpenBoardsWithNoCommandSinceTheCutoff(t *testing.T) {
	// given an idle board, a busy board, an idle closed board, an idle deleted board and an idle board of a deleted organisation
	f := newBoardFixture(t)
	busy := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	closed := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	deleted := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	gone := daotest.CreateOrg(t, f.repos, daotest.CreateUser(t, f.repos).ID)
	inGoneOrg := daotest.CreateBoard(t, f.repos, gone.Team, gone.Owner.UserID)
	cutoff := time.Now().Add(-model.RetroIdleLimit)
	idleAt := cutoff.Add(-time.Hour)
	for _, boardID := range []uuid.UUID{f.boardID(), closed.Header.ID, deleted.Header.ID, inGoneOrg.Header.ID} {
		f.touch(t, boardID, idleAt)
	}
	f.touch(t, busy.Header.ID, cutoff.Add(time.Hour))
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET closed_at = NOW() WHERE id = $1`, closed.Header.ID)
	require.NoError(t, err)
	f.deleteBoard(t, deleted.Header.ID)
	_, err = f.repos.DB().ExecContext(t.Context(), `UPDATE organisations SET deleted_at = NOW() WHERE id = $1`, gone.Organisation.ID)
	require.NoError(t, err)

	// when
	idle, err := f.repos.Board.ListIdle(t.Context(), spec.IdleBoardsQuery{IdleSince: cutoff, Limit: 100})

	// then
	require.NoError(t, err)
	assert.Equal(t, []model.BoardRef{{ID: f.boardID(), OrgID: f.orgID()}}, idle)
}

func TestBoard_ListIdleHonoursItsLimit(t *testing.T) {
	// given two idle boards
	f := newBoardFixture(t)
	other := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	idleAt := time.Now().Add(-model.RetroIdleLimit - time.Hour)
	f.touch(t, f.boardID(), idleAt)
	f.touch(t, other.Header.ID, idleAt)

	// when
	idle, err := f.repos.Board.ListIdle(t.Context(), spec.IdleBoardsQuery{IdleSince: time.Now().Add(-model.RetroIdleLimit), Limit: 1})

	// then
	require.NoError(t, err)
	assert.Len(t, idle, 1)
}

func TestBoard_ChangeParticipantRolePromotesAndDemotes(t *testing.T) {
	// given
	f := newBoardFixture(t)
	seat := f.secondParticipant(t)

	// when
	promoted, err := f.repos.Board.ChangeParticipantRole(t.Context(), spec.ParticipantRoleWrite{BoardWrite: f.write(t), ParticipantID: seat, Role: model.BoardRoleFacilitator})
	require.NoError(t, err)
	demoted, err := f.repos.Board.ChangeParticipantRole(t.Context(), spec.ParticipantRoleWrite{BoardWrite: f.write(t), ParticipantID: seat, Role: model.BoardRoleParticipant})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.BoardRoleFacilitator, promoted.Value.Role)
	assert.Equal(t, model.BoardRoleParticipant, demoted.Value.Role)
}

func TestBoard_RemoveBoardParticipantKeepsTheirCardsAndRemovesOnce(t *testing.T) {
	// given a participant with a card
	f := newBoardFixture(t)
	seat := f.secondParticipant(t)
	added, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{BoardWrite: f.write(t), CardID: uuid.New(), ColumnID: f.column(0), AuthorParticipantID: new(seat), Body: "Mine", Position: "a0"})
	require.NoError(t, err)

	// when
	removed, err := f.repos.Board.RemoveBoardParticipant(t.Context(), spec.ParticipantRemovalWrite{BoardWrite: f.write(t), ParticipantID: seat})
	require.NoError(t, err)
	before := f.version(t)
	_, againErr := f.repos.Board.RemoveBoardParticipant(t.Context(), spec.ParticipantRemovalWrite{BoardWrite: f.write(t), ParticipantID: seat})
	_, roleErr := f.repos.Board.ChangeParticipantRole(t.Context(), spec.ParticipantRoleWrite{BoardWrite: f.write(t), ParticipantID: seat, Role: model.BoardRoleFacilitator})

	// then a removal never reveals which cards were theirs, and a removed seat cannot be changed again
	assert.True(t, removed.Value.Removed())
	require.ErrorIs(t, againErr, dao.ErrNotFound)
	require.ErrorIs(t, roleErr, dao.ErrNotFound)
	assert.Equal(t, before, f.version(t))
	kept, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: added.Value.ID})
	require.NoError(t, err)
	assert.Equal(t, new(seat), kept.AuthorParticipantID)
	read, err := f.repos.Board.GetParticipant(t.Context(), spec.BoardParticipantKey{BoardID: f.boardID(), ParticipantID: seat})
	require.NoError(t, err)
	assert.True(t, read.Removed())
}

func TestBoard_CrossTenantFacilitationIsOnlyReachedThroughItsOrganisationAndBoard(t *testing.T) {
	// given
	bt := newBoardTenants(t)
	seatA := bt.boardA.Participants[0].ID
	throughB := bt.boardAThroughB()
	seatAOnB := spec.BoardParticipantKey{BoardID: bt.boardB.Header.ID, ParticipantID: seatA}
	writeToB := spec.BoardWrite{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, Version: bt.boardB.Header.Version, OpID: uuid.New()}

	// when
	_, revealErr := bt.repos.Board.RevealCards(t.Context(), spec.CardsRevealWrite{BoardWrite: throughB})
	_, hideErr := bt.repos.Board.HideCards(t.Context(), throughB)
	_, advanceErr := bt.repos.Board.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: throughB, From: model.StageWrite, To: model.StageVote, Reveal: true})
	_, stageErr := bt.repos.Board.SetStage(t.Context(), spec.BoardStageChange{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, From: model.StageWrite, To: model.StageVote})
	_, closeErr := bt.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: throughB, Sever: true})
	_, timerErr := bt.repos.Board.SetBoardTimer(t.Context(), spec.BoardTimerWrite{BoardWrite: throughB, Timer: model.BoardTimer{}})
	_, hiddenErr := bt.repos.Board.SetCardsHidden(t.Context(), spec.BoardCardsHiddenChange{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID})
	_, closedErr := bt.repos.Board.Close(t.Context(), spec.BoardKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID})
	_, getErr := bt.repos.Board.GetParticipant(t.Context(), seatAOnB)
	_, roleErr := bt.repos.Board.SetParticipantRole(t.Context(), spec.ParticipantRoleChange{BoardID: bt.boardB.Header.ID, ParticipantID: seatA, Role: model.BoardRoleFacilitator})
	_, removeErr := bt.repos.Board.RemoveParticipant(t.Context(), seatAOnB)
	_, removeWriteErr := bt.repos.Board.RemoveBoardParticipant(t.Context(), spec.ParticipantRemovalWrite{BoardWrite: writeToB, ParticipantID: seatA})
	positionErr := bt.repos.Card.SetPosition(t.Context(), spec.CardPositionChange{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, Position: "z"})
	severedCards, severCardsErr := bt.repos.Card.SeverAuthors(t.Context(), bt.boardB.Header.ID)
	severedComments, severCommentsErr := bt.repos.CardComment.SeverAuthors(t.Context(), bt.boardB.Header.ID)

	// then board A, its seat and its card are untouched
	for _, err := range []error{revealErr, hideErr, advanceErr, closeErr, timerErr, hiddenErr, closedErr, getErr, roleErr, removeErr, removeWriteErr, positionErr} {
		require.ErrorIs(t, err, dao.ErrNotFound)
	}
	require.ErrorIs(t, stageErr, dao.ErrStageMoved, "a stage change through another organisation matches no row")
	headerA, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StageWrite, headerA.Stage)
	assert.True(t, headerA.CardsHidden)
	require.NoError(t, severCardsErr)
	require.NoError(t, severCommentsErr)
	assert.Zero(t, severedCards)
	assert.Zero(t, severedComments)
	bt.boardAUnchanged(t)
	seat, err := bt.repos.Board.GetParticipant(t.Context(), spec.BoardParticipantKey{BoardID: bt.boardA.Header.ID, ParticipantID: seatA})
	require.NoError(t, err)
	assert.False(t, seat.Removed())
	assert.Equal(t, model.BoardRoleFacilitator, seat.Role)
}
