package board

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	idleBoardID = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000010")
	idleSince   = stamp.Add(-model.RetroIdleLimit)
)

func closedBoard(access *model.BoardAccess) model.Board {
	board := access.Header.Board
	board.ClosedAt = new(stamp)
	board.Stage = model.StageEnd
	board.CardsHidden = false

	return board
}

func closeAudit() audit.NewEntry {
	return audit.NewEntry{
		OrgID:      orgID,
		ActorID:    userID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionBoardClosed,
		TargetType: audit.TargetBoard,
		TargetID:   boardID.String(),
		SubjectID:  userID,
	}
}

func idleHeader(id uuid.UUID, lastCommand time.Time) *model.BoardHeader {
	return &model.BoardHeader{
		ID:        id,
		OrgID:     orgID,
		Version:   7,
		Anonymity: model.AnonymityAnonymous,
		UpdatedAt: lastCommand}
}

func idleAudit(id uuid.UUID) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      orgID,
		ActorKind:  audit.ActorSystem,
		Action:     audit.ActionBoardClosed,
		TargetType: audit.TargetBoard,
		TargetID:   id.String(),
		Details:    map[string]string{"reason": "idle"},
	}
}

func (m *testMocks) gateOf(id uuid.UUID) {
	m.acquired++
	m.live.EXPECT().Acquire(mock.Anything, id).Return(func() {
		m.released++
	}, nil).Once()
}

func TestClose_EndsANamedRetroAndKeepsItsAuthors(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.boards.EXPECT().CloseBoard(mock.Anything, spec.BoardCloseWrite{BoardWrite: target()}).Return(&model.Versioned[model.Board]{Value: closedBoard(access), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, closeAudit()).Return(nil).Once()

	// when
	got, err := svc.Close(t.Context(), baseCommand())

	// then everyone refetches the read-only board
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
	assert.True(t, m.events.published[0].access.Header.Closed())
}

func TestClose_SeversAnAnonymousRetroAndShufflesItsHiddenCards(t *testing.T) {
	// given an anonymous board still in its writing phase
	svc, m := newTestService(t)
	svc.shuffle = reverseShuffle
	access := boardAccess(host, anonymous, hiddenCards)
	m.entered(access)
	m.cards.EXPECT().List(mock.Anything, boardID).Return([]model.Card{hiddenCard(cardID, columnID, myID, "a0"), hiddenCard(otherID, columnID, theirID, "a1")}, nil)
	keys := fracindex.Spread(2)
	write := spec.BoardCloseWrite{
		BoardWrite: target(),
		Positions:  []spec.PositionChange{{ID: otherID, Position: keys[0]}, {ID: cardID, Position: keys[1]}},
		Sever:      true,
	}
	m.boards.EXPECT().CloseBoard(mock.Anything, write).Return(&model.Versioned[model.Board]{Value: closedBoard(access), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, closeAudit()).Return(nil).Once()

	// when
	got, err := svc.Close(t.Context(), baseCommand())

	// then the authorship link goes in the same composite, and no card stays hidden with nobody left to own it
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestClose_AMissingAuditEntryStillAnswersTheClose(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.boards.EXPECT().CloseBoard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: closedBoard(access), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, closeAudit()).Return(errDatabase).Once()

	// when
	got, err := svc.Close(t.Context(), baseCommand())

	// then
	assert.Equal(t, versionResponse(8), got)
	require.ErrorIs(t, err, ErrNotAudited)
	require.ErrorIs(t, err, errDatabase)
}

func TestClose_IsRefusedToAParticipantAndOnAClosedBoard(t *testing.T) {
	closed := func(access *model.BoardAccess) {
		access.Header.ClosedAt = new(stamp)
	}

	tests := []struct {
		name   string
		access *model.BoardAccess
		want   error
	}{
		{name: "a participant", access: boardAccess(writer), want: policy.ErrNotAllowed},
		{name: "a closed board", access: boardAccess(host, closed), want: policy.ErrReadOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)

			// when
			got, err := svc.Close(t.Context(), baseCommand())

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestEndIdle_EndsARetroWithNoCommandFor14DaysExactlyAsEndRetroDoes(t *testing.T) {
	// given
	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return stamp
	}
	header := idleHeader(idleBoardID, idleSince.Add(-time.Hour))
	m.boards.EXPECT().ListIdle(mock.Anything, spec.IdleBoardsQuery{IdleSince: idleSince, Limit: 100}).Return([]model.BoardRef{{ID: idleBoardID, OrgID: orgID}}, nil)
	m.gateOf(idleBoardID)
	m.boards.EXPECT().GetHeader(mock.Anything, idleBoardID).Return(header, nil)
	closed := header.Board
	closed.ClosedAt = new(stamp)
	write := spec.BoardCloseWrite{OrgID: orgID, BoardID: idleBoardID, Version: 7, Sever: true}
	m.boards.EXPECT().CloseBoard(mock.Anything, write).Return(&model.Versioned[model.Board]{Value: closed, Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, idleBoardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, idleAudit(idleBoardID)).Return(nil).Once()

	// when
	ended, err := svc.EndIdle(t.Context())

	// then the anonymous board is severed under its gate, with no op to record
	require.NoError(t, err)
	assert.Equal(t, int64(1), ended)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
}

func TestEndIdle_LeavesABoardThatChangedSinceItWasListed(t *testing.T) {
	tests := []struct {
		name   string
		header *model.BoardHeader
	}{
		{name: "a command since", header: idleHeader(idleBoardID, stamp.Add(-time.Hour))},
		{name: "ended since", header: func() *model.BoardHeader {
			header := idleHeader(idleBoardID, idleSince.Add(-time.Hour))
			header.ClosedAt = new(stamp)

			return header
		}()},
		{name: "deleted since", header: func() *model.BoardHeader {
			header := idleHeader(idleBoardID, idleSince.Add(-time.Hour))
			header.DeletedAt = new(stamp)

			return header
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			svc.now = func() time.Time {
				return stamp
			}
			m.boards.EXPECT().ListIdle(mock.Anything, mock.Anything).Return([]model.BoardRef{{ID: idleBoardID, OrgID: orgID}}, nil)
			m.gateOf(idleBoardID)
			m.boards.EXPECT().GetHeader(mock.Anything, idleBoardID).Return(tc.header, nil)

			// when
			ended, err := svc.EndIdle(t.Context())

			// then the strict mocks prove nothing was written
			require.NoError(t, err)
			assert.Zero(t, ended)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestEndIdle_ABusyBoardDoesNotStopTheOthers(t *testing.T) {
	// given
	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return stamp
	}
	busyID := uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000011")
	header := idleHeader(idleBoardID, idleSince.Add(-time.Hour))
	header.Anonymity = model.AnonymityNamed
	m.boards.EXPECT().ListIdle(mock.Anything, mock.Anything).Return([]model.BoardRef{{ID: busyID, OrgID: orgID}, {ID: idleBoardID, OrgID: orgID}}, nil)
	m.live.EXPECT().Acquire(mock.Anything, busyID).Return(nil, live.ErrBusy).Once()
	m.gateOf(idleBoardID)
	m.boards.EXPECT().GetHeader(mock.Anything, idleBoardID).Return(header, nil)
	m.boards.EXPECT().CloseBoard(mock.Anything, spec.BoardCloseWrite{OrgID: orgID, BoardID: idleBoardID, Version: 7}).Return(&model.Versioned[model.Board]{Value: header.Board, Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, idleBoardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, idleAudit(idleBoardID)).Return(nil).Once()

	// when
	ended, err := svc.EndIdle(t.Context())

	// then the busy board is reported and tried again on the next sweep
	assert.Equal(t, int64(1), ended)
	require.ErrorIs(t, err, live.ErrBusy)
}

func TestEndIdle_StopsBeforeTheNextBoardOnceShutdownCancelsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m.boards.EXPECT().ListIdle(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ spec.IdleBoardsQuery, _ ...*sql.Tx) ([]model.BoardRef, error) {
		cancel()

		return []model.BoardRef{{ID: idleBoardID, OrgID: orgID}}, nil
	})

	// when
	ended, err := svc.EndIdle(ctx)

	// then the strict mocks prove no gate was taken
	assert.Zero(t, ended)
	require.ErrorIs(t, err, context.Canceled)
}

func TestEndIdle_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.boards.EXPECT().ListIdle(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	ended, err := svc.EndIdle(t.Context())

	// then
	assert.Zero(t, ended)
	require.ErrorIs(t, err, errDatabase)
}
