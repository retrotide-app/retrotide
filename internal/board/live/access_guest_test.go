package live

import (
	"testing"
	"time"

	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResolve_AGuestIsCappedAtParticipantAndNeverExports(t *testing.T) {
	// given a guest seated through the link at the current epoch, even with a facilitator role written to its row
	svc, m := newTestLive(t)
	header := liveHeader()
	header.LinkAccess = model.LinkAccessGuests
	header.OrgAllowsGuests = true
	deviceID := uuid.New()
	row := &model.BoardParticipant{ID: uuid.New(), BoardID: header.ID, GuestDeviceID: new(deviceID), Role: model.BoardRoleFacilitator, AccessEpoch: header.AccessEpoch, JoinedVia: model.JoinPathLink}
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: header.ID, GuestDeviceID: deviceID}).Return(row, nil)

	// when
	access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: model.Principal{Kind: model.PrincipalGuest, ID: deviceID}, BoardID: header.ID})

	// then no team or organisation is ever asked about a guest, which the strict orgauthz mock proves
	require.NoError(t, err)
	assert.Equal(t, model.BoardRoleParticipant, access.Role)
	assert.False(t, access.OrgMember)
	assert.Equal(t, []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityParticipate}, access.Capabilities)
	assert.Equal(t, row.ID, *access.ParticipantID)
}

func TestResolve_RefusesAGuestTheLinkNoLongerAdmits(t *testing.T) {
	tests := []struct {
		name string
		edit func(header *model.BoardHeader, row *model.BoardParticipant)
		want error
	}{
		{name: "an accounts-only link", edit: func(header *model.BoardHeader, _ *model.BoardParticipant) {
			header.LinkAccess = model.LinkAccessAccounts
		}, want: ErrAccountRequired},
		{name: "an organisation that turned guests off", edit: func(header *model.BoardHeader, _ *model.BoardParticipant) { header.OrgAllowsGuests = false }, want: ErrGuestsNotAllowed},
		{name: "a link switched off", edit: func(header *model.BoardHeader, _ *model.BoardParticipant) { header.LinkAccess = model.LinkAccessOff }, want: ErrLinkDisabled},
		{name: "a rotated link", edit: func(header *model.BoardHeader, _ *model.BoardParticipant) { header.AccessEpoch++ }, want: ErrNoAccess},
		{name: "a new password", edit: func(header *model.BoardHeader, _ *model.BoardParticipant) {
			header.AccessEpoch++
			header.PasswordHash = new("hash")
		}, want: ErrPasswordRequired},
		{name: "a removed guest", edit: func(_ *model.BoardHeader, row *model.BoardParticipant) {
			row.RemovedAt = new(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
		}, want: ErrRemoved},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := liveHeader()
			header.LinkAccess = model.LinkAccessGuests
			header.OrgAllowsGuests = true
			deviceID := uuid.New()
			row := &model.BoardParticipant{ID: uuid.New(), BoardID: header.ID, GuestDeviceID: new(deviceID), Role: model.BoardRoleParticipant, AccessEpoch: header.AccessEpoch, JoinedVia: model.JoinPathLink}
			tc.edit(&header, row)
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.boards.EXPECT().GetParticipantForGuest(mock.Anything, mock.Anything).Return(row, nil)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: model.Principal{Kind: model.PrincipalGuest, ID: deviceID}, BoardID: header.ID})

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, access)
		})
	}
}

func TestResolve_AGuestWithoutASeatHasNoPathIn(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	header := liveHeader()
	header.LinkAccess = model.LinkAccessGuests
	header.OrgAllowsGuests = true
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, mock.Anything).Return(nil, errNoRow)

	// when
	_, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()}, BoardID: header.ID})

	// then the board is not even shown to exist
	require.ErrorIs(t, err, ErrNoAccess)
}

func TestResolve_ReportsAGuestSeatItCannotRead(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	header := liveHeader()
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	_, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()}, BoardID: header.ID})

	// then a blip is never read as no access
	require.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, ErrNoAccess)
}
