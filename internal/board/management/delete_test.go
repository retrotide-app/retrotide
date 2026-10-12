package management

import (
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDelete_SoftDeletesTheBoardRevokesItsLiveViewersAndAuditsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	stored := access.Header.Board
	stored.DeletedAt = new(stamp)
	m.boards.EXPECT().DeleteBoard(mock.Anything, target()).Return(versioned(stored, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, boardAudit(audit.ActionBoardDeleted, nil)).Return(nil).Once()

	// when
	got, err := svc.Delete(t.Context(), baseCommand())

	// then everyone holding the board is told to refetch, and the refetch finds it gone
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, resyncAt(8), m.events.published[0].event.Payload(projection.ClassOthers))
	assert.True(t, m.events.published[0].access.Header.Deleted())
}

func TestDelete_IsAllowedOnAnArchivedBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, archived)
	m.entered(access)
	m.boards.EXPECT().DeleteBoard(mock.Anything, target()).Return(versioned(access.Header.Board, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, boardAudit(audit.ActionBoardDeleted, nil)).Return(nil).Once()

	// when
	_, err := svc.Delete(t.Context(), baseCommand())

	// then
	require.NoError(t, err)
}

func TestDelete_IsRefusedWithoutBoardManage(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.Delete(t.Context(), baseCommand())

	// then
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Nil(t, got)
	assert.Empty(t, m.events.published)
}

func TestDelete_AMissingAuditEntryStillAnswersTheDelete(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.boards.EXPECT().DeleteBoard(mock.Anything, target()).Return(versioned(access.Header.Board, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabase).Once()

	// when
	got, err := svc.Delete(t.Context(), baseCommand())

	// then
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.ErrorIs(t, err, ErrNotAudited)
}

func TestDelete_AFailedWriteIsReportedWithoutAnEvent(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().DeleteBoard(mock.Anything, target()).Return(nil, errDatabase)

	// when
	_, err := svc.Delete(t.Context(), baseCommand())

	// then
	require.ErrorIs(t, err, errDatabase)
	assert.Empty(t, m.events.published)
}

func TestPurgeDeleted_HardDeletesTheBoardsDeletedBeforeTheRestoreWindow(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.boards.EXPECT().PurgeDeletedBefore(mock.Anything, stamp.Add(-model.BoardRestoreWindow)).Return(int64(3), nil)

	// when
	purged, err := svc.PurgeDeleted(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(3), purged)
}

func TestPurgeDeleted_AFailedPurgeIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.boards.EXPECT().PurgeDeletedBefore(mock.Anything, mock.Anything).Return(int64(0), errDatabase)

	// when
	_, err := svc.PurgeDeleted(t.Context())

	// then
	require.ErrorIs(t, err, errDatabase)
}
