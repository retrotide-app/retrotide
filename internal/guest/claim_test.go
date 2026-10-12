package guest

import (
	"context"
	"errors"
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	claimFixture struct {
		boards   *repository.MockBoardRepository
		claimer  *MockBoardClaimer
		devices  *MockDeviceRetirer
		auditLog *repository.MockAuditLogRepository
		svc      ClaimService
	}
)

var (
	claimUser      = uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")
	claimDevice    = uuid.MustParse("2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e")
	namedBoard     = model.Board{ID: uuid.MustParse("3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f"), OrgID: uuid.MustParse("4d5e6f7a-8b9c-4d0e-9f2a-3b4c5d6e7f80"), Title: "Sprint 42", Anonymity: model.AnonymityNamed}
	anonymousBoard = model.Board{ID: uuid.MustParse("5e6f7a8b-9c0d-4e1f-a03b-4c5d6e7f8091"), OrgID: uuid.MustParse("4d5e6f7a-8b9c-4d0e-9f2a-3b4c5d6e7f80"), Title: "Honest feedback", Anonymity: model.AnonymityAnonymous}
)

func newClaimFixture(t *testing.T) *claimFixture {
	t.Helper()

	f := &claimFixture{
		boards:   repository.NewMockBoardRepository(t),
		claimer:  NewMockBoardClaimer(t),
		devices:  NewMockDeviceRetirer(t),
		auditLog: repository.NewMockAuditLogRepository(t),
	}
	f.svc = NewClaimService(ClaimDeps{Boards: f.boards, Claimer: f.claimer, Devices: f.devices, AuditLog: f.auditLog})

	return f
}

func TestOffer_ListsTheNamedBoardsTheDeviceJoinedAndNeverAnAnonymousOne(t *testing.T) {
	// given a device seated on a named board and an anonymous one
	f := newClaimFixture(t)
	f.boards.EXPECT().ListForGuestDevice(mock.Anything, claimDevice).Return([]model.GuestBoard{
		{Board: namedBoard, Participant: model.BoardParticipant{ID: uuid.New(), DisplayName: "Gil"}},
		{Board: anonymousBoard, Participant: model.BoardParticipant{ID: uuid.New(), DisplayName: "Gil"}},
	}, nil)

	// when
	offer, err := f.svc.Offer(t.Context(), claimDevice)

	// then claiming the anonymous board would link anonymous cards to an account, so it is never offered
	require.NoError(t, err)
	assert.Equal(t, &dto.GuestClaimOffer{Boards: []dto.GuestClaimBoard{{BoardID: namedBoard.ID, Title: "Sprint 42", DisplayName: "Gil"}}}, offer)
}

func TestOffer_OffersNothingWithoutADevice(t *testing.T) {
	// given a browser holding no live guest device, which the strict repository proves is never read
	f := newClaimFixture(t)

	// when
	offer, err := f.svc.Offer(t.Context(), uuid.Nil)

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.GuestClaimOffer{Boards: []dto.GuestClaimBoard{}}, offer)
}

func TestOffer_ReportsAListItCannotRead(t *testing.T) {
	// given
	f := newClaimFixture(t)
	errDown := errors.New("boards store down")
	f.boards.EXPECT().ListForGuestDevice(mock.Anything, claimDevice).Return(nil, errDown)

	// when
	_, err := f.svc.Offer(t.Context(), claimDevice)

	// then
	require.ErrorIs(t, err, errDown)
}

func TestClaim_RefusesAnEmptyChoiceBeforeAnythingIsWritten(t *testing.T) {
	// given
	f := newClaimFixture(t)

	// when
	_, err := f.svc.Claim(t.Context(), spec.GuestClaim{UserID: claimUser, DeviceID: claimDevice})

	// then the strict mocks prove nothing was claimed or retired
	require.ErrorIs(t, err, ErrNothingClaimed)
}

func TestClaim_ClaimsThenRetiresTheDeviceThenAuditsEachBoard(t *testing.T) {
	// given
	f := newClaimFixture(t)
	claim := spec.GuestClaim{UserID: claimUser, DeviceID: claimDevice, BoardIDs: []uuid.UUID{namedBoard.ID}}
	var order []string
	f.claimer.EXPECT().ClaimGuest(mock.Anything, claim).RunAndReturn(func(_ context.Context, _ spec.GuestClaim) ([]model.Board, error) {
		order = append(order, "claim")

		return []model.Board{namedBoard}, nil
	})
	f.devices.EXPECT().Retire(mock.Anything, claimDevice).RunAndReturn(func(_ context.Context, _ uuid.UUID) error {
		order = append(order, "retire")

		return nil
	})
	f.auditLog.EXPECT().Create(mock.Anything, audit.NewEntry{
		OrgID:      namedBoard.OrgID,
		ActorID:    claimUser,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionGuestClaimed,
		TargetType: audit.TargetBoard,
		TargetID:   namedBoard.ID.String(),
		SubjectID:  claimUser,
	}).Return(nil)

	// when
	result, err := f.svc.Claim(t.Context(), claim)

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.GuestClaimResult{Claimed: []uuid.UUID{namedBoard.ID}}, result)
	assert.Equal(t, []string{"claim", "retire"}, order)
}

func TestClaim_KeepsTheDeviceWhenTheClaimFails(t *testing.T) {
	// given
	f := newClaimFixture(t)
	errBusy := errors.New("board busy")
	f.claimer.EXPECT().ClaimGuest(mock.Anything, mock.Anything).Return(nil, errBusy)

	// when
	_, err := f.svc.Claim(t.Context(), spec.GuestClaim{UserID: claimUser, DeviceID: claimDevice, BoardIDs: []uuid.UUID{namedBoard.ID}})

	// then the strict retirer proves the device survives, so the offer can be made again
	require.ErrorIs(t, err, errBusy)
}

func TestClaim_AnUnrecordedAuditStillAnswersTheClaim(t *testing.T) {
	// given
	f := newClaimFixture(t)
	errAudit := errors.New("audit store down")
	f.claimer.EXPECT().ClaimGuest(mock.Anything, mock.Anything).Return([]model.Board{namedBoard}, nil)
	f.devices.EXPECT().Retire(mock.Anything, claimDevice).Return(nil)
	f.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAudit)

	// when
	result, err := f.svc.Claim(t.Context(), spec.GuestClaim{UserID: claimUser, DeviceID: claimDevice, BoardIDs: []uuid.UUID{namedBoard.ID}})

	// then
	require.ErrorIs(t, err, ErrNotAudited)
	require.ErrorIs(t, err, errAudit)
	assert.Equal(t, &dto.GuestClaimResult{Claimed: []uuid.UUID{namedBoard.ID}}, result)
}
