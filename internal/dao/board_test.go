package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoard_GetSnapshotReadsEveryRowWithItsVersion(t *testing.T) {
	// given a board with a card in each column
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(1), "a0")

	// when
	snapshot, err := f.repos.Board.GetSnapshot(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: f.boardID()})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), snapshot.Header.Version)
	assert.Equal(t, f.boardID(), snapshot.Header.Board.ID)
	assert.Len(t, snapshot.Columns, 2)
	require.Len(t, snapshot.Cards, 2)
	assert.ElementsMatch(t, []uuid.UUID{first.ID, second.ID}, []uuid.UUID{snapshot.Cards[0].ID, snapshot.Cards[1].ID})
	require.Len(t, snapshot.Participants, 1)
	assert.Equal(t, f.facilitator(), snapshot.Participants[0].ID)
}

func TestBoard_GetSnapshotRefusesTheCallersTransaction(t *testing.T) {
	// given
	f := newBoardFixture(t)
	tx, err := f.repos.DB().BeginTx(t.Context(), nil)
	require.NoError(t, err)

	// when
	_, snapshotErr := f.repos.Board.GetSnapshot(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: f.boardID()}, tx)

	// then a joined transaction would lose the snapshot
	require.ErrorIs(t, snapshotErr, db.ErrInboundTx)
	require.NoError(t, tx.Rollback())
}

func TestBoard_GetSnapshotOfAMissingBoardIsNotFound(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	_, err := f.repos.Board.GetSnapshot(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: uuid.New()})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestBoard_SetBoardContextWritesTheDescriptionLine(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	described, err := f.repos.Board.SetBoardContext(t.Context(), spec.BoardContextWrite{BoardWrite: f.write(t), Context: "Sprint 12, the release week"})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), described.Version)
	assert.Equal(t, "Sprint 12, the release week", described.Value.Context)

	header, err := f.repos.Board.GetByID(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: f.boardID()})
	require.NoError(t, err)
	assert.Equal(t, "Sprint 12, the release week", header.Board.Context)
	assert.Equal(t, int64(1), header.Version)
}

func TestBoard_SetContextOfADeletedBoardIsNotFound(t *testing.T) {
	// given
	f := newBoardFixture(t)
	f.deleteBoard(t, f.boardID())

	// when
	_, setErr := f.repos.Board.SetContext(t.Context(), spec.BoardContextChange{OrgID: f.orgID(), BoardID: f.boardID(), Context: "Too late"})

	// then
	require.ErrorIs(t, setErr, dao.ErrNotFound)
}
