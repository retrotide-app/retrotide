package live

import (
	"context"
	"database/sql"
	"testing"

	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/ws"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (f *liveFixture) openToGuests() {
	f.header().LinkAccess = model.LinkAccessGuests
	f.header().OrgAllowsGuests = true

	f.boards.EXPECT().GetParticipantForGuest(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key spec.BoardParticipantGuest, _ ...*sql.Tx) (*model.BoardParticipant, error) {
		row, ok := f.rows[key.GuestDeviceID]
		if !ok || row.BoardID != key.BoardID {
			return nil, errNoRow
		}

		copied := *row

		return &copied, nil
	}).Maybe()
}

func (f *liveFixture) seatGuest(name string) seat {
	principal := model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()}
	row := &model.BoardParticipant{
		ID:            uuid.New(),
		BoardID:       f.boardID,
		GuestDeviceID: new(principal.ID),
		DisplayName:   name,
		Role:          model.BoardRoleParticipant,
		AccessEpoch:   f.header().AccessEpoch,
		JoinedVia:     model.JoinPathLink,
	}

	f.rows[principal.ID] = row

	return seat{principal: principal, participantID: row.ID, conn: &fakeConn{id: uuid.New(), principal: principal}}
}

func TestHandleJoin_AGuestJoinsAsALinkParticipant(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, true)
	f.openToGuests()
	gil := f.seatGuest("Gil")

	// when
	joined, err := f.svc.HandleJoin(t.Context(), gil.conn, f.boardID)

	// then a guest is capped at participant, whatever its stored row says
	require.NoError(t, err)
	assert.True(t, joined)
	assert.Equal(t, ws.BoardJoinedMessage(ws.BoardJoined{BoardRef: f.ref(), Version: 7, ParticipantID: new(gil.participantID), Role: model.BoardRoleParticipant}), gil.conn.sent[0])
	assert.True(t, f.hub.member(f.boardID, gil.principal.ID))
}

func TestHandleJoin_AGuestShowsAsAnonymousWhileGuestNamesAreOff(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		typed   string
		want    string
	}{
		{name: "names on", enabled: true, typed: "Gil", want: "Gil"},
		{name: "names off", enabled: false, typed: "Gil", want: "Anonymous"},
		{name: "no name typed", enabled: true, typed: "", want: "Anonymous"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newLiveFixture(t, model.AnonymityNamed, true)
			f.openToGuests()
			f.header().GuestNamesEnabled = tc.enabled
			gil := f.seatGuest(tc.typed)

			// when
			f.join(t, gil)

			// then
			assert.Equal(t, tc.want, f.viewerOf(gil).displayName)
		})
	}
}

func TestHandleJoin_RefusesAGuestTheBoardNoLongerAdmits(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(f *liveFixture)
		want    ws.RejectReason
	}{
		{name: "the link is accounts only", arrange: func(f *liveFixture) { f.header().LinkAccess = model.LinkAccessAccounts }, want: ws.RejectAccountRequired},
		{name: "the organisation turned guests off", arrange: func(f *liveFixture) { f.header().OrgAllowsGuests = false }, want: ws.RejectAccountRequired},
		{name: "the link is off", arrange: func(f *liveFixture) { f.header().LinkAccess = model.LinkAccessOff }, want: ws.RejectLinkDisabled},
		{name: "the link was rotated", arrange: func(f *liveFixture) { f.header().AccessEpoch++ }, want: ws.RejectNotParticipant},
		{name: "a password was set", arrange: func(f *liveFixture) {
			f.header().AccessEpoch++
			f.header().PasswordHash = new("hash")
		}, want: ws.RejectPasswordRequired},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newLiveFixture(t, model.AnonymityNamed, true)
			f.openToGuests()
			gil := f.seatGuest("Gil")
			tc.arrange(f)

			// when
			joined, err := f.svc.HandleJoin(t.Context(), gil.conn, f.boardID)

			// then
			require.NoError(t, err)
			assert.False(t, joined)
			assert.Equal(t, []ws.Message{ws.BoardJoinRejectedMessage(ws.BoardJoinRejected{BoardRef: f.ref(), Reason: tc.want})}, gil.conn.sent)
		})
	}
}

func TestReevaluateHeld_EvictsTheGuestsAChangeShutsOutAndKeepsTheMembers(t *testing.T) {
	tests := []struct {
		name   string
		change func(f *liveFixture)
		want   ws.RejectReason
	}{
		{name: "a password change", change: func(f *liveFixture) {
			f.header().AccessEpoch++
			f.header().PasswordHash = new("hash")
		}, want: ws.RejectPasswordRequired},
		{name: "a link rotation", change: func(f *liveFixture) { f.header().AccessEpoch++ }, want: ws.RejectNotParticipant},
		{name: "narrowing the link to accounts", change: func(f *liveFixture) { f.header().LinkAccess = model.LinkAccessAccounts }, want: ws.RejectAccountRequired},
		{name: "switching the link off", change: func(f *liveFixture) { f.header().LinkAccess = model.LinkAccessOff }, want: ws.RejectLinkDisabled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a guest and a team member live on the board
			f := newLiveFixture(t, model.AnonymityNamed, false)
			f.openToGuests()
			gil := f.seatGuest("Gil")
			sam := f.seat("Sam", teamMember)
			f.join(t, gil)
			f.join(t, sam)
			tc.change(f)

			// when
			err := f.svc.ReevaluateHeld(t.Context(), f.boardID)

			// then the guest is told why and receives nothing further, and the member keeps the board
			require.NoError(t, err)
			assert.Equal(t, ws.BoardAccessRevokedMessage(ws.BoardAccessRevoked{BoardRef: f.ref(), Reason: tc.want}), gil.conn.sent[len(gil.conn.sent)-1])
			assert.False(t, f.hub.member(f.boardID, gil.principal.ID))
			assert.Nil(t, f.viewerOf(gil))
			assert.True(t, f.hub.member(f.boardID, sam.principal.ID))
			assert.NotNil(t, f.viewerOf(sam))
		})
	}
}

func TestReevaluateHeld_DoesNothingForABoardNobodyWatches(t *testing.T) {
	// given a board with no roster, whose header the strict mock would refuse to read
	f := newLiveFixture(t, model.AnonymityNamed, false)
	f.headerErr = errDatabase

	// when
	err := f.svc.ReevaluateHeld(t.Context(), f.boardID)

	// then
	require.NoError(t, err)
}

func TestReevaluateHeld_ReportsAHeaderItCannotRead(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	f.join(t, sam)
	f.headerErr = errDatabase

	// when
	err := f.svc.ReevaluateHeld(t.Context(), f.boardID)

	// then the viewer is never evicted on a blip
	require.ErrorIs(t, err, errDatabase)
	assert.NotNil(t, f.viewerOf(sam))
}
