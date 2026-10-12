package live

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	claimBoards struct {
		named     model.BoardHeader
		anonymous model.BoardHeader
	}
)

var (
	claimingUser   = uuid.MustParse("0a1b2c3d-4e5f-4a6b-8c7d-8e9f0a1b2c3d")
	claimingDevice = uuid.MustParse("1b2c3d4e-5f6a-4b7c-9d8e-9f0a1b2c3d4e")
)

func newClaimBoards() claimBoards {
	named := liveHeader()
	named.ID = uuid.MustParse("2c3d4e5f-6a7b-4c8d-ae9f-0a1b2c3d4e5f")

	anonymous := liveHeader()
	anonymous.ID = uuid.MustParse("3d4e5f6a-7b8c-4d9e-8f0a-1b2c3d4e5f6a")
	anonymous.Anonymity = model.AnonymityAnonymous

	return claimBoards{named: named, anonymous: anonymous}
}

func claimSeat(header model.BoardHeader) *model.BoardParticipant {
	return &model.BoardParticipant{ID: uuid.New(), BoardID: header.ID, GuestDeviceID: new(claimingDevice), DisplayName: "Gil", Role: model.BoardRoleParticipant, JoinedVia: model.JoinPathLink, AccessEpoch: header.AccessEpoch}
}

func (m *liveMocks) joinedAsGuest(boards ...model.BoardHeader) map[uuid.UUID]*model.BoardParticipant {
	seats := make(map[uuid.UUID]*model.BoardParticipant, len(boards))
	joined := make([]model.GuestBoard, 0, len(boards))
	for _, header := range boards {
		held := claimSeat(header)
		seats[header.ID] = held
		joined = append(joined, model.GuestBoard{Board: header.Board, Participant: *held})
	}

	m.boards.EXPECT().ListForGuestDevice(mock.Anything, claimingDevice).Return(joined, nil)

	return seats
}

func TestClaimGuest_HandsTheGuestSeatToAUserWithNone(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	boards := newClaimBoards()
	seats := m.joinedAsGuest(boards.named)
	m.boards.EXPECT().GetHeader(mock.Anything, boards.named.ID).Return(&boards.named, nil)
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: boards.named.ID, GuestDeviceID: claimingDevice}).Return(seats[boards.named.ID], nil)
	m.boards.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: boards.named.ID, UserID: claimingUser}).Return(nil, errNoRow)
	want := spec.GuestClaimWrite{
		OrgID: boards.named.OrgID, BoardID: boards.named.ID, Version: boards.named.Version,
		GuestParticipantID: seats[boards.named.ID].ID,
		UserID:             claimingUser,
	}
	m.boards.EXPECT().ClaimGuestParticipant(mock.Anything, want).Return(&model.Versioned[model.BoardParticipant]{Value: *seats[boards.named.ID], Version: boards.named.Version + 1}, nil)

	// when
	claimed, err := svc.ClaimGuest(t.Context(), spec.GuestClaim{UserID: claimingUser, DeviceID: claimingDevice, BoardIDs: []uuid.UUID{boards.named.ID}})

	// then
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, boards.named.ID, claimed[0].ID)
	assert.Empty(t, svc.(*service).gates.byID)
}

func TestClaimGuest_MovesTheGuestsCardsToTheUsersExistingSeat(t *testing.T) {
	// given a user already seated on the board they joined as a guest from another browser
	svc, m := newTestLive(t)
	boards := newClaimBoards()
	seats := m.joinedAsGuest(boards.named)
	userSeat := storedRow(boards.named, model.Principal{Kind: model.PrincipalUser, ID: claimingUser}, model.BoardRoleParticipant)
	m.boards.EXPECT().GetHeader(mock.Anything, boards.named.ID).Return(&boards.named, nil)
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, mock.Anything).Return(seats[boards.named.ID], nil)
	m.boards.EXPECT().GetParticipantForUser(mock.Anything, mock.Anything).Return(userSeat, nil)
	m.boards.EXPECT().ClaimGuestParticipant(mock.Anything, mock.MatchedBy(func(write spec.GuestClaimWrite) bool {
		return write.UserParticipantID != nil && *write.UserParticipantID == userSeat.ID && write.GuestParticipantID == seats[boards.named.ID].ID
	})).Return(&model.Versioned[model.BoardParticipant]{Value: *userSeat, Version: boards.named.Version + 1}, nil)

	// when
	claimed, err := svc.ClaimGuest(t.Context(), spec.GuestClaim{UserID: claimingUser, DeviceID: claimingDevice, BoardIDs: []uuid.UUID{boards.named.ID}})

	// then
	require.NoError(t, err)
	assert.Len(t, claimed, 1)
}

func TestClaimGuest_NeverClaimsAnAnonymousBoard(t *testing.T) {
	// given an anonymous board chosen by a crafted request, which the strict repository proves is never read or written
	svc, m := newTestLive(t)
	boards := newClaimBoards()
	m.joinedAsGuest(boards.anonymous)

	// when
	claimed, err := svc.ClaimGuest(t.Context(), spec.GuestClaim{UserID: claimingUser, DeviceID: claimingDevice, BoardIDs: []uuid.UUID{boards.anonymous.ID}})

	// then claiming would link anonymous cards to an account
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestClaimGuest_SkipsABoardThatTurnedAnonymousSinceTheOffer(t *testing.T) {
	// given the board was named when the offer was listed and is anonymous under its gate
	svc, m := newTestLive(t)
	boards := newClaimBoards()
	m.joinedAsGuest(boards.named)
	switched := boards.named
	switched.Anonymity = model.AnonymityAnonymous
	m.boards.EXPECT().GetHeader(mock.Anything, boards.named.ID).Return(&switched, nil)

	// when
	claimed, err := svc.ClaimGuest(t.Context(), spec.GuestClaim{UserID: claimingUser, DeviceID: claimingDevice, BoardIDs: []uuid.UUID{boards.named.ID}})

	// then
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestClaimGuest_IgnoresABoardTheDeviceNeverJoined(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	boards := newClaimBoards()
	m.joinedAsGuest(boards.named)

	// when
	claimed, err := svc.ClaimGuest(t.Context(), spec.GuestClaim{UserID: claimingUser, DeviceID: claimingDevice, BoardIDs: []uuid.UUID{uuid.New()}})

	// then nobody can claim a seat by naming a board
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestClaimGuest_SkipsABoardWhereTheUserWasRemoved(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	boards := newClaimBoards()
	seats := m.joinedAsGuest(boards.named)
	removed := storedRow(boards.named, model.Principal{Kind: model.PrincipalUser, ID: claimingUser}, model.BoardRoleParticipant)
	removed.RemovedAt = new(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	m.boards.EXPECT().GetHeader(mock.Anything, boards.named.ID).Return(&boards.named, nil)
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, mock.Anything).Return(seats[boards.named.ID], nil)
	m.boards.EXPECT().GetParticipantForUser(mock.Anything, mock.Anything).Return(removed, nil)

	// when
	claimed, err := svc.ClaimGuest(t.Context(), spec.GuestClaim{UserID: claimingUser, DeviceID: claimingDevice, BoardIDs: []uuid.UUID{boards.named.ID}})

	// then a claim never puts a removed user back on the board
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestClaimGuest_TakesTheGatesInAscendingBoardOrder(t *testing.T) {
	// given two named boards listed in descending order
	svc, m := newTestLive(t)
	first := liveHeader()
	first.ID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	second := liveHeader()
	second.ID = uuid.MustParse("ffffffff-ffff-4fff-bfff-ffffffffffff")
	seats := m.joinedAsGuest(second, first)
	var order []uuid.UUID
	for _, header := range []model.BoardHeader{first, second} {
		m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
		m.boards.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: header.ID, GuestDeviceID: claimingDevice}).Return(seats[header.ID], nil)
		m.boards.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: header.ID, UserID: claimingUser}).Return(nil, errNoRow)
	}
	m.boards.EXPECT().ClaimGuestParticipant(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, write spec.GuestClaimWrite, _ ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
		order = append(order, write.BoardID)

		return &model.Versioned[model.BoardParticipant]{Value: *seats[write.BoardID], Version: write.Version + 1}, nil
	})

	// when
	claimed, err := svc.ClaimGuest(t.Context(), spec.GuestClaim{UserID: claimingUser, DeviceID: claimingDevice, BoardIDs: []uuid.UUID{second.ID, first.ID}})

	// then one gate at a time in a fixed order can never deadlock
	require.NoError(t, err)
	assert.Len(t, claimed, 2)
	assert.Equal(t, []uuid.UUID{first.ID, second.ID}, order)
}
