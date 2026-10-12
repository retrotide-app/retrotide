package dao_test

import (
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

func (f boardFixture) key() spec.BoardKey {
	return spec.BoardKey{OrgID: f.orgID(), BoardID: f.boardID()}
}

func (f boardFixture) deletedAgo(t *testing.T, boardID uuid.UUID, ago time.Duration) {
	t.Helper()

	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET deleted_at = $2 WHERE id = $1`, boardID, time.Now().Add(-ago))
	require.NoError(t, err)
}

func TestBoard_PinsAreIdempotentPerUserAndListedForTheirMembership(t *testing.T) {
	// given
	f := newBoardFixture(t)
	owner := f.workspace.Owner.UserID
	pin := spec.BoardPinKey{OrgID: f.orgID(), BoardID: f.boardID(), UserID: owner}
	member := daotest.CreateUser(t, f.repos)
	daotest.AddOrgMember(t, f.repos, f.orgID(), member.ID, model.OrgRoleMember)

	// when
	firstErr := f.repos.Board.Pin(t.Context(), pin)
	secondErr := f.repos.Board.Pin(t.Context(), pin)
	pinned, listErr := f.repos.Board.ListPinned(t.Context(), spec.OrganisationMemberKey{OrgID: f.orgID(), UserID: owner})
	others, othersErr := f.repos.Board.ListPinned(t.Context(), spec.OrganisationMemberKey{OrgID: f.orgID(), UserID: member.ID})

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, listErr)
	assert.Equal(t, []uuid.UUID{f.boardID()}, pinned)
	require.NoError(t, othersErr)
	assert.Empty(t, others, "a pin belongs to the user who made it")
}

func TestBoard_UnpinRemovesThePinAndAMissingPinIsNoError(t *testing.T) {
	// given
	f := newBoardFixture(t)
	pin := spec.BoardPinKey{OrgID: f.orgID(), BoardID: f.boardID(), UserID: f.workspace.Owner.UserID}
	require.NoError(t, f.repos.Board.Pin(t.Context(), pin))

	// when
	firstErr := f.repos.Board.Unpin(t.Context(), pin)
	secondErr := f.repos.Board.Unpin(t.Context(), pin)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	pinned, err := f.repos.Board.ListPinned(t.Context(), spec.OrganisationMemberKey{OrgID: f.orgID(), UserID: f.workspace.Owner.UserID})
	require.NoError(t, err)
	assert.Empty(t, pinned)
}

func TestBoard_SetArchivedKeepsTheFirstArchiveTimeAndUnarchiveClearsIt(t *testing.T) {
	// given
	f := newBoardFixture(t)
	archive := spec.BoardArchiveChange{OrgID: f.orgID(), BoardID: f.boardID(), Archived: true}

	// when
	first, firstErr := f.repos.Board.SetArchived(t.Context(), archive)
	second, secondErr := f.repos.Board.SetArchived(t.Context(), archive)
	cleared, clearErr := f.repos.Board.SetArchived(t.Context(), spec.BoardArchiveChange{OrgID: f.orgID(), BoardID: f.boardID()})

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, clearErr)
	require.NotNil(t, first.ArchivedAt)
	assert.Equal(t, first.ArchivedAt, second.ArchivedAt)
	assert.Nil(t, cleared.ArchivedAt)
}

func TestBoard_SoftDeleteHidesTheBoardFromItsListsAndOnlyOnce(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	deleted, deleteErr := f.repos.Board.SoftDelete(t.Context(), f.key())
	_, againErr := f.repos.Board.SoftDelete(t.Context(), f.key())
	listed, listErr := f.repos.Board.ListForOrganisation(t.Context(), f.orgID())
	gone, goneErr := f.repos.Board.ListDeletedForOrganisation(t.Context(), spec.DeletedBoardsQuery{OrgID: f.orgID(), DeletedSince: time.Now().Add(-model.BoardRestoreWindow)})

	// then
	require.NoError(t, deleteErr)
	assert.True(t, deleted.Deleted())
	require.ErrorIs(t, againErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	assert.Empty(t, listed)
	require.NoError(t, goneErr)
	require.Len(t, gone, 1)
	assert.Equal(t, f.boardID(), gone[0].ID)
	assert.NotEmpty(t, gone[0].CreatorName)
	assert.NotNil(t, gone[0].DeletedAt)
}

func TestBoard_ListDeletedForOrganisationListsTheNewestFirstAndOnlyInsideTheWindow(t *testing.T) {
	// given an older deletion, a newer one, and one past the restore window
	f := newBoardFixture(t)
	newer := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	expired := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	f.deletedAgo(t, f.boardID(), 2*time.Hour)
	f.deletedAgo(t, newer.Header.ID, time.Hour)
	f.deletedAgo(t, expired.Header.ID, model.BoardRestoreWindow+time.Hour)

	// when
	gone, err := f.repos.Board.ListDeletedForOrganisation(t.Context(), spec.DeletedBoardsQuery{OrgID: f.orgID(), DeletedSince: time.Now().Add(-model.BoardRestoreWindow)})

	// then
	require.NoError(t, err)
	require.Len(t, gone, 2)
	assert.Equal(t, newer.Header.ID, gone[0].ID)
	assert.Equal(t, f.boardID(), gone[1].ID)
}

func TestBoard_RestoreBringsBackABoardDeletedInsideTheWindowOnly(t *testing.T) {
	// given one board deleted an hour ago and another past the window
	f := newBoardFixture(t)
	expired := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	f.deletedAgo(t, f.boardID(), time.Hour)
	f.deletedAgo(t, expired.Header.ID, model.BoardRestoreWindow+time.Hour)
	since := time.Now().Add(-model.BoardRestoreWindow)

	// when
	restored, restoreErr := f.repos.Board.Restore(t.Context(), spec.BoardRestore{OrgID: f.orgID(), BoardID: f.boardID(), DeletedSince: since})
	_, expiredErr := f.repos.Board.Restore(t.Context(), spec.BoardRestore{OrgID: f.orgID(), BoardID: expired.Header.ID, DeletedSince: since})
	_, liveErr := f.repos.Board.Restore(t.Context(), spec.BoardRestore{OrgID: f.orgID(), BoardID: f.boardID(), DeletedSince: since})

	// then
	require.NoError(t, restoreErr)
	assert.False(t, restored.Deleted())
	require.ErrorIs(t, expiredErr, dao.ErrNotFound)
	require.ErrorIs(t, liveErr, dao.ErrNotFound, "a live board has nothing to restore")
}

func TestBoard_PurgeDeletedBeforeHardDeletesOnlyBoardsPastTheCutoff(t *testing.T) {
	// given an expired deletion with a card, a recent deletion, and a live board
	f := newBoardFixture(t)
	f.addCard(t, f.column(0), "a0")
	recent := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	live := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	f.deletedAgo(t, f.boardID(), model.BoardRestoreWindow+time.Hour)
	f.deletedAgo(t, recent.Header.ID, time.Hour)

	// when
	purged, err := f.repos.Board.PurgeDeletedBefore(t.Context(), time.Now().Add(-model.BoardRestoreWindow))

	// then the expired board and everything on it is gone
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged)
	_, goneErr := f.repos.Board.GetHeader(t.Context(), f.boardID())
	require.ErrorIs(t, goneErr, dao.ErrNotFound)
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM cards WHERE board_id = $1`, f.boardID()))
	_, recentErr := f.repos.Board.GetHeader(t.Context(), recent.Header.ID)
	require.NoError(t, recentErr)
	_, liveErr := f.repos.Board.GetHeader(t.Context(), live.Header.ID)
	require.NoError(t, liveErr)
}

func TestBoard_SetTeamMovesTheBoardAndMoveBoardMovesItsActionItems(t *testing.T) {
	// given a second team and an action item raised on the board
	f := newBoardFixture(t)
	target := daotest.CreateTeam(t, f.repos, f.orgID())
	raised := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Fix the flaky test"}, nil)

	// when
	moved, moveErr := f.repos.Board.MoveBoard(t.Context(), spec.BoardTeamWrite{BoardWrite: f.write(t), TeamID: new(target.ID)})

	// then
	require.NoError(t, moveErr)
	assert.Equal(t, new(target.ID), moved.Value.TeamID)
	item, err := f.repos.ActionItem.GetByID(t.Context(), spec.ActionItemKey{OrgID: f.orgID(), ItemID: raised.ID})
	require.NoError(t, err)
	assert.Equal(t, new(target.ID), item.TeamID)
}

func TestBoard_SetTeamToNoTeamMakesAnOrganisationBoard(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	moved, err := f.repos.Board.SetTeam(t.Context(), spec.BoardTeamChange{OrgID: f.orgID(), BoardID: f.boardID()})

	// then
	require.NoError(t, err)
	assert.Nil(t, moved.TeamID)
}

func TestBoard_SetSettingsChangesShowCardDatesAndLeavesItWhenUnset(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	shown, shownErr := f.repos.Board.SetSettings(t.Context(), spec.BoardSettingsChange{OrgID: f.orgID(), BoardID: f.boardID(), Patch: spec.BoardSettingsPatch{ShowCardDates: new(true)}})
	kept, keptErr := f.repos.Board.SetSettings(t.Context(), spec.BoardSettingsChange{OrgID: f.orgID(), BoardID: f.boardID(), Patch: spec.BoardSettingsPatch{VotingOpen: new(false)}})

	// then
	require.NoError(t, shownErr)
	assert.True(t, shown.ShowCardDates)
	require.NoError(t, keptErr)
	assert.True(t, kept.ShowCardDates)
}

func TestBoard_SetBackgroundAttachesAndClearsAnImageOfTheBoard(t *testing.T) {
	// given
	f := newBoardFixture(t)
	media, err := f.repos.Media.Create(t.Context(), spec.NewMedia{OrgID: f.orgID(), BoardID: f.boardID(), Kind: model.MediaKindBackground, Width: 2560, Height: 1440, Data: []byte("webp")})
	require.NoError(t, err)

	// when
	set, setErr := f.repos.Board.SetBackground(t.Context(), spec.BoardBackgroundChange{OrgID: f.orgID(), BoardID: f.boardID(), MediaID: new(media.ID)})
	cleared, clearErr := f.repos.Board.SetBackground(t.Context(), spec.BoardBackgroundChange{OrgID: f.orgID(), BoardID: f.boardID()})

	// then
	require.NoError(t, setErr)
	assert.Equal(t, new(media.ID), set.BackgroundMediaID)
	require.NoError(t, clearErr)
	assert.Nil(t, cleared.BackgroundMediaID)
}
