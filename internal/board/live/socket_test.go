package live

import (
	"context"
	"testing"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"
	"retrotide/internal/ws"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *liveFixture) ref() ws.BoardRef {
	return ws.BoardRef{BoardID: f.boardID}
}

func TestHandleJoin_RepliesWithTheVersionAndJoinsTheHubBoardGroup(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, true)
	sam := f.seat("Sam", teamMember)

	// when
	joined, err := f.svc.HandleJoin(t.Context(), sam.conn, f.boardID)

	// then
	require.NoError(t, err)
	assert.True(t, joined)
	require.Len(t, sam.conn.sent, 1)
	assert.Equal(t, ws.BoardJoinedMessage(ws.BoardJoined{
		BoardRef:      f.ref(),
		Version:       7,
		ParticipantID: new(sam.participantID),
		Role:          model.BoardRoleParticipant,
	}), sam.conn.sent[0])
	assert.True(t, f.hub.member(f.boardID, sam.principal.ID))
	assert.Equal(t, int64(7), f.viewerOf(sam).lastTo)
}

func TestHandleJoin_RefusesWithTheReasonAndNeverJoinsTheHubBoardGroup(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(f *liveFixture, sam seat)
		want    ws.RejectReason
	}{
		{
			name: "removed",
			arrange: func(f *liveFixture, sam seat) {
				f.rows[sam.principal.ID].RemovedAt = new(time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))
			},
			want: ws.RejectRemoved,
		},
		{
			name: "no path in",
			arrange: func(f *liveFixture, sam seat) {
				delete(f.rows, sam.principal.ID)
				f.standings[sam.principal.ID] = offTeam
			},
			want: ws.RejectNotParticipant,
		},
		{
			name: "archived",
			arrange: func(f *liveFixture, _ seat) {
				f.header().ArchivedAt = new(time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))
			},
			want: ws.RejectArchived,
		},
		{
			name: "link switched off",
			arrange: func(f *liveFixture, sam seat) {
				f.rows[sam.principal.ID].JoinedVia = model.JoinPathLink
				f.standings[sam.principal.ID] = offTeam
			},
			want: ws.RejectLinkDisabled,
		},
		{
			name: "password changed",
			arrange: func(f *liveFixture, sam seat) {
				f.rows[sam.principal.ID].JoinedVia = model.JoinPathLink
				f.rows[sam.principal.ID].AccessEpoch = 1
				f.standings[sam.principal.ID] = offTeam
				f.header().LinkAccess = model.LinkAccessGuests
				f.header().PasswordHash = new("hash")
			},
			want: ws.RejectPasswordRequired,
		},
		{
			name: "missing board",
			arrange: func(f *liveFixture, _ seat) {
				delete(f.headers, f.boardID)
			},
			want: ws.RejectNotParticipant,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newLiveFixture(t, model.AnonymityNamed, true)
			sam := f.seat("Sam", teamMember)
			tc.arrange(f, sam)

			// when
			joined, err := f.svc.HandleJoin(t.Context(), sam.conn, f.boardID)

			// then
			require.NoError(t, err)
			assert.False(t, joined)
			assert.Equal(t, []ws.Message{ws.BoardJoinRejectedMessage(ws.BoardJoinRejected{BoardRef: f.ref(), Reason: tc.want})}, sam.conn.sent)
			assert.False(t, f.hub.member(f.boardID, sam.principal.ID))
			assert.Nil(t, f.viewerOf(sam))
		})
	}
}

func TestHandleJoin_ARefusedRejoinDropsTheTabItHad(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	f.join(t, sam)
	f.rows[sam.principal.ID].RemovedAt = new(time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))

	// when
	joined, err := f.svc.HandleJoin(t.Context(), sam.conn, f.boardID)

	// then
	require.NoError(t, err)
	assert.False(t, joined)
	assert.False(t, f.hub.member(f.boardID, sam.principal.ID))
	assert.Nil(t, f.viewerOf(sam))
	assert.Equal(t, ws.BoardJoinRejectedMessage(ws.BoardJoinRejected{BoardRef: f.ref(), Reason: ws.RejectRemoved}), sam.conn.sent[len(sam.conn.sent)-1])
}

func TestHandleJoin_ARefusedRejoinDropsEveryTabOfThePrincipal(t *testing.T) {
	// given two tabs of one person, and the person is then removed
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	ali := f.seat("Ali", teamMember)
	secondTab := sam
	secondTab.conn = &fakeConn{id: uuid.New(), principal: sam.principal}
	f.join(t, sam)
	f.join(t, ali)
	f.rows[sam.principal.ID].RemovedAt = new(time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))

	// when the second tab reconnects
	joined, err := f.svc.HandleJoin(t.Context(), secondTab.conn, f.boardID)

	// then
	require.NoError(t, err)
	assert.False(t, joined)
	assert.Equal(t, []ws.Message{ws.BoardJoinRejectedMessage(ws.BoardJoinRejected{BoardRef: f.ref(), Reason: ws.RejectRemoved})}, secondTab.conn.sent)
	assert.Equal(t, ws.BoardAccessRevokedMessage(ws.BoardAccessRevoked{BoardRef: f.ref(), Reason: ws.RejectRemoved}), sam.conn.sent[len(sam.conn.sent)-1])
	assert.False(t, f.hub.member(f.boardID, sam.principal.ID))
	assert.Nil(t, f.viewerOf(sam))

	// when
	f.svc.Publish(f.access(ali), projection.ColumnCleared(f.header().Board, liveColumnID, 8))
	f.timer().flush()

	// then
	assert.Empty(t, f.hub.framesOf(sam.principal.ID), "the tab that stayed open receives nothing after the refusal")
	assert.Len(t, f.hub.framesOf(ali.principal.ID), 1)
}

func TestHandleJoin_AnUnexpectedFailureSendsNothing(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, true)
	sam := f.seat("Sam", teamMember)
	f.headerErr = errDatabaseDown

	// when
	joined, err := f.svc.HandleJoin(t.Context(), sam.conn, f.boardID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.False(t, joined)
	assert.Empty(t, sam.conn.sent)
	assert.False(t, f.hub.member(f.boardID, sam.principal.ID))
}

func TestHandleJoin_AJoinWaitingAtTheGateEndsWithItsConnection(t *testing.T) {
	// given a gate someone else holds and a connection that has already closed
	f := newLiveFixture(t, model.AnonymityNamed, true)
	sam := f.seat("Sam", teamMember)
	release, err := f.svc.Acquire(t.Context(), f.boardID)
	require.NoError(t, err)
	defer release()

	closed, cancel := context.WithCancel(t.Context())
	cancel()

	// when
	joined, err := f.svc.HandleJoin(closed, sam.conn, f.boardID)

	// then
	require.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, ErrBusy)
	assert.False(t, joined)
	assert.Empty(t, sam.conn.sent)
}

func TestHandleLeave_OnlyTheLastTabLeavesTheHubBoardGroup(t *testing.T) {
	// given two tabs of one person
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	secondTab := sam
	secondTab.conn = &fakeConn{id: uuid.New(), principal: sam.principal}
	f.join(t, sam)
	f.join(t, secondTab)
	f.join(t, sam)

	// when
	require.NoError(t, f.svc.HandleLeave(t.Context(), sam.conn, f.boardID))

	// then
	assert.True(t, f.hub.member(f.boardID, sam.principal.ID), "the other tab still watches")
	assert.Len(t, f.viewerOf(sam).tabs, 1, "a resent join from one socket never counts twice")

	// when
	require.NoError(t, f.svc.HandleLeave(t.Context(), secondTab.conn, f.boardID))

	// then
	assert.False(t, f.hub.member(f.boardID, sam.principal.ID))
	assert.False(t, f.svc.watched(f.boardID), "a board with no viewers is forgotten")
}

func TestHandleLeave_ALeaveOfABoardNeverJoinedIsHarmless(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)

	// when
	err := f.svc.HandleLeave(t.Context(), sam.conn, f.boardID)

	// then
	require.NoError(t, err)
	assert.False(t, f.hub.member(f.boardID, sam.principal.ID))
}

func TestRevalidate_KeepsABoardWhoseAccessStillHolds(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	f.join(t, sam)
	f.standings[sam.principal.ID] = teamLead

	// when
	kept, err := f.svc.Revalidate(t.Context(), sam.conn, f.boardID)

	// then
	require.NoError(t, err)
	assert.True(t, kept)
	assert.Equal(t, model.BoardRoleParticipant, f.viewerOf(sam).access.Role, "role changes reach the roster only under the gate")
	assert.True(t, f.hub.member(f.boardID, sam.principal.ID))
}

func TestRevalidate_RevokedAccessDropsTheViewerAndTellsEveryTab(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	ali := f.seat("Ali", teamMember)
	secondTab := sam
	secondTab.conn = &fakeConn{id: uuid.New(), principal: sam.principal}
	f.join(t, sam)
	f.join(t, secondTab)
	f.join(t, ali)
	f.rows[sam.principal.ID].RemovedAt = new(time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))

	// when
	kept, err := f.svc.Revalidate(t.Context(), sam.conn, f.boardID)

	// then
	require.NoError(t, err)
	assert.False(t, kept)
	revoked := ws.BoardAccessRevokedMessage(ws.BoardAccessRevoked{BoardRef: f.ref(), Reason: ws.RejectRemoved})
	assert.Equal(t, revoked, sam.conn.sent[len(sam.conn.sent)-1])
	assert.Equal(t, revoked, secondTab.conn.sent[len(secondTab.conn.sent)-1])
	assert.False(t, f.hub.member(f.boardID, sam.principal.ID))
	assert.Nil(t, f.viewerOf(sam))

	// when
	f.svc.Publish(f.access(ali), projection.ColumnCleared(f.header().Board, liveColumnID, 8))
	f.timer().flush()

	// then
	assert.Empty(t, f.hub.framesOf(sam.principal.ID), "nothing published afterwards reaches the revoked viewer")
	assert.Len(t, f.hub.framesOf(ali.principal.ID), 1)
}

func TestRevalidate_AnUnexpectedFailureIsReturned(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	f.join(t, sam)
	f.headerErr = errDatabaseDown

	// when
	kept, err := f.svc.Revalidate(t.Context(), sam.conn, f.boardID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.False(t, kept)
	assert.Equal(t, []string{ws.TypeBoardJoined}, sentTypes(sam.conn))
}

func TestHandleViewerState_IsRecordedForAJoinedViewer(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	f.join(t, sam)

	// when
	err := f.svc.HandleViewerState(t.Context(), sam.conn, ws.ViewerStateChange{BoardRef: f.ref(), State: ws.ViewerStateIdle})

	// then
	require.NoError(t, err)
	assert.Equal(t, ws.ViewerStateIdle, f.viewerOf(sam).state)
}
