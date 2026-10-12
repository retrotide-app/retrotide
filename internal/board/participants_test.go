package board

import (
	"context"
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	theirUserID  = uuid.MustParse("1a2b3c4d-0000-4000-8000-00000000000e")
	theirGuestID = uuid.MustParse("1a2b3c4d-0000-4000-8000-00000000000f")
	theirSeat    = spec.BoardParticipantKey{BoardID: boardID, ParticipantID: theirID}
	theirUser    = model.Principal{Kind: model.PrincipalUser, ID: theirUserID}
)

func theirRow() *model.BoardParticipant {
	return &model.BoardParticipant{ID: theirID, BoardID: boardID, UserID: new(theirUserID), Role: model.BoardRoleParticipant, JoinedVia: model.JoinPathMembership}
}

func theirGuestRow() *model.BoardParticipant {
	return &model.BoardParticipant{ID: theirID, BoardID: boardID, GuestDeviceID: new(theirGuestID), DisplayName: "Gil", Role: model.BoardRoleParticipant, JoinedVia: model.JoinPathLink}
}

func roleCommand(role model.BoardRole) ParticipantRoleCommand {
	return ParticipantRoleCommand{Command: baseCommand(), ParticipantID: theirID, Role: role}
}

func removalCommand(participantID uuid.UUID) ParticipantCommand {
	return ParticipantCommand{Command: baseCommand(), ParticipantID: participantID}
}

func TestChangeParticipantRole_PromotesAMemberToCoFacilitate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(theirRow(), nil)
	m.live.EXPECT().MembershipRole(mock.Anything, access.Header, theirUser).Return(model.BoardRoleParticipant, nil)
	promoted := theirRow()
	promoted.Role = model.BoardRoleFacilitator
	m.boards.EXPECT().ChangeParticipantRole(mock.Anything, spec.ParticipantRoleWrite{BoardWrite: target(), ParticipantID: theirID, Role: model.BoardRoleFacilitator}).Return(&model.Versioned[model.BoardParticipant]{Value: *promoted, Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return([]model.BoardParticipantProfile{{BoardParticipant: *promoted, AccountName: "Ali"}}, nil)

	// when
	got, err := svc.ChangeParticipantRole(t.Context(), roleCommand(model.BoardRoleFacilitator))

	// then everyone learns the new role, and the promoted participant refetches for their new capabilities
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	assert.Equal(t, dto.ParticipantUpsertedEvent{
		Type: projection.EventParticipantUpserted, Version: 8,
		Participant: dto.ParticipantView{ParticipantID: theirID, DisplayName: "Ali", Role: model.BoardRoleFacilitator},
	}, event.Payload(event.ClassOf(projection.Viewer{ParticipantID: new(myID)})))
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, event.Payload(event.ClassOf(projection.Viewer{ParticipantID: new(theirID)})))
}

func TestChangeParticipantRole_RefusesWhatMembershipDecides(t *testing.T) {
	tests := []struct {
		name       string
		row        *model.BoardParticipant
		role       model.BoardRole
		membership model.BoardRole
		asked      bool
		want       error
	}{
		{name: "a guest is never promoted", row: theirGuestRow(), role: model.BoardRoleFacilitator, want: ErrCannotPromote},
		{name: "an account in through the link alone is never promoted", row: theirRow(), role: model.BoardRoleFacilitator, asked: true, want: ErrCannotPromote},
		{name: "a team lead is never demoted", row: theirRow(), role: model.BoardRoleParticipant, membership: model.BoardRoleFacilitator, asked: true, want: ErrLeadFacilitates},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			access := boardAccess(host)
			m.entered(access)
			m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(tc.row, nil)
			if tc.asked {
				m.live.EXPECT().MembershipRole(mock.Anything, access.Header, theirUser).Return(tc.membership, nil)
			}

			// when
			got, err := svc.ChangeParticipantRole(t.Context(), roleCommand(tc.role))

			// then the strict mocks prove nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestChangeParticipantRole_DemotesACoFacilitator(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	promoted := theirRow()
	promoted.Role = model.BoardRoleFacilitator
	m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(promoted, nil)
	m.live.EXPECT().MembershipRole(mock.Anything, access.Header, theirUser).Return(model.BoardRoleParticipant, nil)
	m.boards.EXPECT().ChangeParticipantRole(mock.Anything, spec.ParticipantRoleWrite{BoardWrite: target(), ParticipantID: theirID, Role: model.BoardRoleParticipant}).Return(&model.Versioned[model.BoardParticipant]{Value: *theirRow(), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return([]model.BoardParticipantProfile{{BoardParticipant: *theirRow(), AccountName: "Ali"}}, nil)

	// when
	got, err := svc.ChangeParticipantRole(t.Context(), roleCommand(model.BoardRoleParticipant))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestChangeParticipantRole_AnUnknownRoleIsRefusedBeforeTakingTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.ChangeParticipantRole(t.Context(), roleCommand("owner"))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrInvalidRole)
}

func TestChangeParticipantRole_IsTheFacilitatorsAlone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.ChangeParticipantRole(t.Context(), roleCommand(model.BoardRoleFacilitator))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestRemoveParticipant_EvictsThemBeforeAnnouncingTheRemoval(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(theirRow(), nil)
	m.live.EXPECT().MembershipRole(mock.Anything, access.Header, theirUser).Return(model.BoardRoleParticipant, nil)
	removed := theirRow()
	removed.RemovedAt = new(stamp)
	m.boards.EXPECT().RemoveBoardParticipant(mock.Anything, spec.ParticipantRemovalWrite{BoardWrite: target(), ParticipantID: theirID}).Return(&model.Versioned[model.BoardParticipant]{Value: *removed, Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).RunAndReturn(func(_ context.Context, _ uuid.UUID) error {
		assert.Empty(t, m.events.published, "the removed participant leaves the roster before the event is queued")
		return nil
	})

	// when
	got, err := svc.RemoveParticipant(t.Context(), removalCommand(theirID))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.ParticipantRemovedEvent{
		Type: projection.EventParticipantRemoved, Version: 8,
		ParticipantID: theirID,
	}, m.events.published[0].event.Payload(projection.ClassOthers))
}

func TestRemoveParticipant_RemovesAGuest(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(theirGuestRow(), nil)
	m.boards.EXPECT().RemoveBoardParticipant(mock.Anything, spec.ParticipantRemovalWrite{BoardWrite: target(), ParticipantID: theirID}).Return(&model.Versioned[model.BoardParticipant]{Value: *theirGuestRow(), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)

	// when
	got, err := svc.RemoveParticipant(t.Context(), removalCommand(theirID))

	// then a guest holds no membership to ask about
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestRemoveParticipant_RefusesWhoCannotBeRemoved(t *testing.T) {
	tests := []struct {
		name   string
		target uuid.UUID
		expect func(m *testMocks, access *model.BoardAccess)
		want   error
	}{
		{
			name:   "yourself",
			target: myID,
			expect: func(m *testMocks, _ *model.BoardAccess) {
				mine := &model.BoardParticipant{ID: myID, BoardID: boardID, UserID: new(userID), Role: model.BoardRoleFacilitator}
				m.boards.EXPECT().GetParticipant(mock.Anything, spec.BoardParticipantKey{BoardID: boardID, ParticipantID: myID}).Return(mine, nil)
			},
			want: ErrRemoveSelf,
		},
		{
			name:   "a team lead",
			target: theirID,
			expect: func(m *testMocks, access *model.BoardAccess) {
				m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(theirRow(), nil)
				m.live.EXPECT().MembershipRole(mock.Anything, access.Header, theirUser).Return(model.BoardRoleFacilitator, nil)
			},
			want: ErrLeadFacilitates,
		},
		{
			name:   "someone not on the board",
			target: theirID,
			expect: func(m *testMocks, _ *model.BoardAccess) {
				m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(nil, errNotFound)
			},
			want: ErrParticipantNotFound,
		},
		{
			name:   "someone already removed",
			target: theirID,
			expect: func(m *testMocks, _ *model.BoardAccess) {
				removed := theirRow()
				removed.RemovedAt = new(stamp)
				m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(removed, nil)
			},
			want: ErrParticipantNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			access := boardAccess(host)
			m.entered(access)
			tc.expect(m, access)

			// when
			got, err := svc.RemoveParticipant(t.Context(), removalCommand(tc.target))

			// then the strict mocks prove nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestRemoveParticipant_IsTheFacilitatorsAlone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.RemoveParticipant(t.Context(), removalCommand(theirID))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestRemoveParticipant_AFailedReevaluationTellsEveryoneToRefetch(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().GetParticipant(mock.Anything, theirSeat).Return(theirGuestRow(), nil)
	m.boards.EXPECT().RemoveBoardParticipant(mock.Anything, mock.Anything).Return(&model.Versioned[model.BoardParticipant]{Value: *theirGuestRow(), Version: 8}, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(errDatabase)

	// when
	got, err := svc.RemoveParticipant(t.Context(), removalCommand(theirID))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, errDatabase)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventResync, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
}
