package management

import (
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	firstBoard  = uuid.MustParse("10000000-0000-4000-8000-000000000001")
	secondBoard = uuid.MustParse("20000000-0000-4000-8000-000000000002")
	thirdBoard  = uuid.MustParse("30000000-0000-4000-8000-000000000003")
)

func accessTo(id uuid.UUID, org uuid.UUID, role model.BoardRole) *model.BoardAccess {
	access := boardAccess(role)
	access.Header.ID = id
	access.Header.OrgID = org

	return access
}

func (m *testMocks) enteredBoard(access *model.BoardAccess) {
	id := access.Header.ID

	m.acquired++
	m.live.EXPECT().Acquire(mock.Anything, id).Return(func() {
		m.released++
	}, nil).Once()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: id}).Return(access, nil).Once()
	m.boards.EXPECT().GetOp(mock.Anything, mock.MatchedBy(func(key spec.BoardOpKey) bool {
		return key.BoardID == id && key.OpID != uuid.Nil
	})).Return(nil, errNotFound).Once()
}

func (m *testMocks) settledBoard(access *model.BoardAccess) {
	id := access.Header.ID
	m.live.EXPECT().ReevaluateHeld(mock.Anything, id).Return(nil).Once()
}

func onBoard(id uuid.UUID) func(write spec.BoardArchiveWrite) bool {
	return func(write spec.BoardArchiveWrite) bool {
		return write.BoardID == id && write.OrgID == orgID
	}
}

func bulkCommand(action string, boardIDs ...uuid.UUID) BulkCommand {
	return BulkCommand{Principal: me, OrgID: orgID, Action: action, BoardIDs: boardIDs}
}

func TestBulk_ArchivesEachBoardInIDOrderAndKeepsGoingPastARefusal(t *testing.T) {
	// given one board the viewer manages, one they only take part in, and one of another organisation they also belong to
	svc, m := newTestService(t)
	managed := accessTo(firstBoard, orgID, host)
	joined := accessTo(secondBoard, orgID, writer)
	elsewhere := accessTo(thirdBoard, otherOrgID, host)
	m.enteredBoard(managed)
	m.enteredBoard(joined)
	m.enteredBoard(elsewhere)
	m.boards.EXPECT().ArchiveBoard(mock.Anything, mock.MatchedBy(onBoard(firstBoard))).Return(versioned(managed.Header.Board, 8), nil).Once()
	m.settledBoard(managed)
	m.auditLog.EXPECT().Create(mock.Anything, mock.MatchedBy(func(entry audit.NewEntry) bool {
		return entry.TargetID == firstBoard.String() && entry.Details["archived"] == "true"
	})).Return(nil).Once()

	// when
	outcomes, err := svc.Bulk(t.Context(), bulkCommand(BulkArchive, thirdBoard, firstBoard, secondBoard))

	// then the other organisation's board answers as if it did not exist, and nothing was written to it
	require.NoError(t, err)
	require.Len(t, outcomes, 3)
	assert.Equal(t, firstBoard, outcomes[0].BoardID)
	require.NoError(t, outcomes[0].Err)
	assert.Equal(t, secondBoard, outcomes[1].BoardID)
	require.ErrorIs(t, outcomes[1].Err, policy.ErrNotAllowed)
	assert.Equal(t, thirdBoard, outcomes[2].BoardID)
	require.ErrorIs(t, outcomes[2].Err, live.ErrNoAccess)
	assert.Len(t, m.events.published, 1)
}

func TestBulk_UnarchivesEveryBoardAndKeepsAnUnauditedOutcome(t *testing.T) {
	// given
	svc, m := newTestService(t)
	first := accessTo(firstBoard, orgID, host)
	archived(first)
	m.enteredBoard(first)
	m.boards.EXPECT().ArchiveBoard(mock.Anything, mock.MatchedBy(func(write spec.BoardArchiveWrite) bool {
		return write.BoardID == firstBoard && !write.Archived
	})).Return(versioned(first.Header.Board, 8), nil).Once()
	m.settledBoard(first)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabase).Once()

	// when
	outcomes, err := svc.Bulk(t.Context(), bulkCommand(BulkUnarchive, firstBoard))

	// then
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.ErrorIs(t, outcomes[0].Err, ErrNotAudited)
}

func TestBulk_DeletesABoardNamedTwiceOnlyOnce(t *testing.T) {
	// given
	svc, m := newTestService(t)
	first := accessTo(firstBoard, orgID, host)
	m.enteredBoard(first)
	m.boards.EXPECT().DeleteBoard(mock.Anything, mock.MatchedBy(func(write spec.BoardWrite) bool {
		return write.BoardID == firstBoard
	})).Return(versioned(first.Header.Board, 8), nil).Once()
	m.settledBoard(first)
	m.auditLog.EXPECT().Create(mock.Anything, mock.MatchedBy(func(entry audit.NewEntry) bool {
		return entry.Action == audit.ActionBoardDeleted
	})).Return(nil).Once()

	// when
	outcomes, err := svc.Bulk(t.Context(), bulkCommand(BulkDelete, firstBoard, firstBoard))

	// then
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.NoError(t, outcomes[0].Err)
}

func TestBulk_RefusesTheRequestBeforeTouchingAnyBoard(t *testing.T) {
	tooMany := make([]uuid.UUID, 0, MaxBulkBoards+1)
	for range MaxBulkBoards + 1 {
		tooMany = append(tooMany, uuid.New())
	}

	tests := []struct {
		name    string
		command BulkCommand
		want    error
	}{
		{name: "an unknown action", command: bulkCommand("restore", firstBoard), want: ErrInvalidBulkAction},
		{name: "no boards", command: bulkCommand(BulkArchive), want: ErrBoardsRequired},
		{name: "more than 100 boards", command: bulkCommand(BulkDelete, tooMany...), want: ErrTooManyBoards},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			outcomes, err := svc.Bulk(t.Context(), tc.command)

			// then the strict mocks prove no board was read
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, outcomes)
		})
	}
}

func TestCheckBulkBoards_AcceptsBetweenOneAndOneHundredBoards(t *testing.T) {
	// given
	exactly := make([]uuid.UUID, MaxBulkBoards)

	// when
	err := CheckBulkBoards(exactly)

	// then
	require.NoError(t, err)
}
