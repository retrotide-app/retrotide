package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	guestBoard struct {
		repos *repository.Repositories
		owner *model.User
		org   *model.Workspace
		board *model.BoardState
	}
)

func newGuestBoard(t *testing.T) guestBoard {
	t.Helper()

	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	board := daotest.CreateBoard(t, repos, workspace.Team, owner.ID)

	return guestBoard{repos: repos, owner: owner, org: workspace, board: board}
}

func (g guestBoard) header(t *testing.T) *model.BoardHeader {
	t.Helper()

	header, err := g.repos.Board.GetHeader(t.Context(), g.board.Header.ID)
	require.NoError(t, err)

	return header
}

func (g guestBoard) write(t *testing.T) spec.BoardWrite {
	t.Helper()

	return spec.BoardWrite{OrgID: g.org.Organisation.ID, BoardID: g.board.Header.ID, Version: g.header(t).Version, OpID: uuid.New()}
}

func (g guestBoard) card(t *testing.T, authorID uuid.UUID, body string) model.Card {
	t.Helper()

	added, err := g.repos.Card.AddCard(t.Context(), spec.CardCreate{
		BoardWrite:          g.write(t),
		CardID:              uuid.New(),
		ColumnID:            g.board.Columns[0].ID,
		AuthorParticipantID: new(authorID),
		Body:                body,
		Position:            "a0",
	})
	require.NoError(t, err)

	return added.Value
}

func TestGuest_GetHeaderBySlugFindsTheBoardAndItsOrganisationsGuestPolicy(t *testing.T) {
	// given
	g := newGuestBoard(t)

	// when
	found, err := g.repos.Board.GetHeaderBySlug(t.Context(), g.board.Header.ShareSlug)
	_, unknownErr := g.repos.Board.GetHeaderBySlug(t.Context(), "never-shared")

	// then
	require.NoError(t, err)
	assert.Equal(t, g.board.Header.ID, found.ID)
	assert.True(t, found.OrgAllowsGuests)
	require.ErrorIs(t, unknownErr, dao.ErrNotFound)
}

func TestGuest_JoinLinkParticipantSeatsAGuestAtTheBoardsEpoch(t *testing.T) {
	// given
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)

	// when
	joined, err := g.repos.Board.JoinLinkParticipant(t.Context(), spec.LinkParticipantJoin{BoardWrite: g.write(t), GuestDeviceID: new(device.ID), DisplayName: "Gil", AccessEpoch: 0})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), joined.Version)
	assert.Equal(t, model.JoinPathLink, joined.Value.JoinedVia)
	assert.Equal(t, model.BoardRoleParticipant, joined.Value.Role)
	assert.Equal(t, "Gil", joined.Value.DisplayName)
	require.NotNil(t, joined.Value.GuestDeviceID)
	assert.Equal(t, device.ID, *joined.Value.GuestDeviceID)
	assert.Nil(t, joined.Value.UserID)

	got, err := g.repos.Board.GetParticipantForGuest(t.Context(), spec.BoardParticipantGuest{BoardID: g.board.Header.ID, GuestDeviceID: device.ID})
	require.NoError(t, err)
	assert.Equal(t, joined.Value.ID, got.ID)
}

func TestGuest_ARejoinRefreshesTheEpochAndNameButNeverClearsARemoval(t *testing.T) {
	// given a guest seated at epoch 0 and since removed
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)
	seat := daotest.CreateGuestParticipant(t, g.repos, g.board.Header, device.ID)
	_, err := g.repos.DB().ExecContext(t.Context(), `UPDATE board_participants SET removed_at = NOW() WHERE id = $1`, seat.ID)
	require.NoError(t, err)

	// when
	rejoined, err := g.repos.Board.JoinLinkParticipant(t.Context(), spec.LinkParticipantJoin{BoardWrite: g.write(t), GuestDeviceID: new(device.ID), DisplayName: "Gilly", AccessEpoch: 4})

	// then the same seat is refreshed, and the removal stands
	require.NoError(t, err)
	assert.Equal(t, seat.ID, rejoined.Value.ID)
	assert.Equal(t, 4, rejoined.Value.AccessEpoch)
	assert.Equal(t, "Gilly", rejoined.Value.DisplayName)
	assert.True(t, rejoined.Value.Removed())
}

func TestGuest_ALinkJoinOfAMemberKeepsTheirRoleAndMarksTheLink(t *testing.T) {
	// given the creator, seated as facilitator through membership
	g := newGuestBoard(t)

	// when
	joined, err := g.repos.Board.JoinLinkParticipant(t.Context(), spec.LinkParticipantJoin{BoardWrite: g.write(t), UserID: new(g.owner.ID), AccessEpoch: 0})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.BoardRoleFacilitator, joined.Value.Role)
	assert.Equal(t, model.JoinPathLink, joined.Value.JoinedVia)
	assert.Equal(t, g.board.Participants[0].ID, joined.Value.ID)
}

func TestGuest_ASeatBelongsToOneUserOrOneDevice(t *testing.T) {
	// given
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)
	member := daotest.CreateUser(t, g.repos)

	// when
	_, bothErr := g.repos.Board.AddParticipant(t.Context(), spec.NewBoardParticipant{BoardID: g.board.Header.ID, UserID: new(member.ID), GuestDeviceID: new(device.ID), Role: model.BoardRoleParticipant})
	_, upsertErr := g.repos.Board.UpsertLinkParticipant(t.Context(), spec.LinkParticipant{BoardID: g.board.Header.ID, UserID: new(member.ID), GuestDeviceID: new(device.ID)})

	// then
	require.Error(t, bothErr)
	require.ErrorIs(t, upsertErr, dao.ErrOnePrincipal)
}

func TestGuest_CountLiveParticipantsLeavesOutTheRemoved(t *testing.T) {
	// given the creator, a guest, and a removed guest
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)
	daotest.CreateGuestParticipant(t, g.repos, g.board.Header, device.ID)
	gone, _ := daotest.CreateGuest(t, g.repos)
	removed := daotest.CreateGuestParticipant(t, g.repos, g.board.Header, gone.ID)
	_, err := g.repos.DB().ExecContext(t.Context(), `UPDATE board_participants SET removed_at = NOW() WHERE id = $1`, removed.ID)
	require.NoError(t, err)

	// when
	count, err := g.repos.Board.CountLiveParticipants(t.Context(), spec.BoardSeatCount{BoardID: g.board.Header.ID, AccessEpoch: g.board.Header.AccessEpoch})

	// then
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestGuest_CountLiveParticipantsLeavesOutSeatsWithNoWayIn(t *testing.T) {
	// given the creator, a guest seated at the current epoch, a guest behind a rotated link, and a guest whose device is gone
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)
	daotest.CreateGuestParticipant(t, g.repos, g.board.Header, device.ID)
	stale, _ := daotest.CreateGuest(t, g.repos)
	daotest.CreateGuestParticipant(t, g.repos, g.board.Header, stale.ID)
	swept, _ := daotest.CreateGuest(t, g.repos)
	daotest.CreateGuestParticipant(t, g.repos, g.board.Header, swept.ID)
	_, err := g.repos.DB().ExecContext(t.Context(), `DELETE FROM guest_devices WHERE id = $1`, swept.ID)
	require.NoError(t, err)

	before, err := g.repos.Board.CountLiveParticipants(t.Context(), spec.BoardSeatCount{BoardID: g.board.Header.ID, AccessEpoch: g.board.Header.AccessEpoch})
	require.NoError(t, err)

	rotated, err := g.repos.Board.RotateBoardLink(t.Context(), spec.BoardLinkRotationWrite{BoardWrite: g.write(t), ShareSlug: "ROTATEDSLUGROTATEDSLUG23"})
	require.NoError(t, err)
	_, err = g.repos.DB().ExecContext(t.Context(), `UPDATE board_participants SET access_epoch = $1 WHERE guest_device_id = $2`, rotated.Value.AccessEpoch, device.ID)
	require.NoError(t, err)

	// when
	after, err := g.repos.Board.CountLiveParticipants(t.Context(), spec.BoardSeatCount{BoardID: g.board.Header.ID, AccessEpoch: rotated.Value.AccessEpoch})

	// then the device-less seat never counts, and the rotation frees the stale seat
	require.NoError(t, err)
	assert.Equal(t, 3, before)
	assert.Equal(t, 2, after)
}

func TestGuest_SharingWritesBumpTheEpochOnlyForThePasswordAndTheLink(t *testing.T) {
	// given
	g := newGuestBoard(t)
	hash := "$2a$10$daotest.board.password.hash.not.real"

	// when
	opened, openErr := g.repos.Board.SetBoardLinkAccess(t.Context(), spec.BoardLinkAccessWrite{BoardWrite: g.write(t), LinkAccess: model.LinkAccessGuests})
	protected, protectErr := g.repos.Board.SetBoardPassword(t.Context(), spec.BoardPasswordWrite{BoardWrite: g.write(t), PasswordHash: &hash})
	rotated, rotateErr := g.repos.Board.RotateBoardLink(t.Context(), spec.BoardLinkRotationWrite{BoardWrite: g.write(t), ShareSlug: "ROTATEDSLUGROTATEDSLUG23"})
	cleared, clearErr := g.repos.Board.SetBoardPassword(t.Context(), spec.BoardPasswordWrite{BoardWrite: g.write(t)})

	// then every participant who got in through the old link or password is left behind
	require.NoError(t, openErr)
	require.NoError(t, protectErr)
	require.NoError(t, rotateErr)
	require.NoError(t, clearErr)
	assert.Equal(t, model.LinkAccessGuests, opened.Value.LinkAccess)
	assert.Equal(t, 0, opened.Value.AccessEpoch)
	assert.Equal(t, 1, protected.Value.AccessEpoch)
	assert.Equal(t, &hash, protected.Value.PasswordHash)
	assert.Equal(t, 2, rotated.Value.AccessEpoch)
	assert.Equal(t, "ROTATEDSLUGROTATEDSLUG23", rotated.Value.ShareSlug)
	assert.Equal(t, 3, cleared.Value.AccessEpoch)
	assert.Nil(t, cleared.Value.PasswordHash)
	assert.Equal(t, int64(4), cleared.Version)

	_, oldSlugErr := g.repos.Board.GetHeaderBySlug(t.Context(), g.board.Header.ShareSlug)
	require.ErrorIs(t, oldSlugErr, dao.ErrNotFound)
}

func TestGuest_ListForGuestDeviceListsOnlyTheDevicesLiveSeats(t *testing.T) {
	// given a device seated on two boards, removed from a third, and another device on a fourth
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)
	other, _ := daotest.CreateGuest(t, g.repos)
	second := daotest.CreateBoard(t, g.repos, g.org.Team, g.owner.ID)
	removedFrom := daotest.CreateBoard(t, g.repos, g.org.Team, g.owner.ID)
	elsewhere := daotest.CreateBoard(t, g.repos, g.org.Team, g.owner.ID)
	daotest.CreateGuestParticipant(t, g.repos, g.board.Header, device.ID)
	daotest.CreateGuestParticipant(t, g.repos, second.Header, device.ID)
	removed := daotest.CreateGuestParticipant(t, g.repos, removedFrom.Header, device.ID)
	daotest.CreateGuestParticipant(t, g.repos, elsewhere.Header, other.ID)
	_, err := g.repos.DB().ExecContext(t.Context(), `UPDATE board_participants SET removed_at = NOW() WHERE id = $1`, removed.ID)
	require.NoError(t, err)

	// when
	joined, err := g.repos.Board.ListForGuestDevice(t.Context(), device.ID)

	// then
	require.NoError(t, err)
	var boards []uuid.UUID
	for _, entry := range joined {
		boards = append(boards, entry.Board.ID)
		assert.Equal(t, device.ID, *entry.Participant.GuestDeviceID)
	}
	assert.ElementsMatch(t, []uuid.UUID{g.board.Header.ID, second.Header.ID}, boards)
}

func TestClaim_AGuestSeatPassesToAUserWithNoneAndKeepsItsCards(t *testing.T) {
	// given a guest who wrote a card, then signed in as a user with no seat on the board
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)
	seat := daotest.CreateGuestParticipant(t, g.repos, g.board.Header, device.ID)
	card := g.card(t, seat.ID, "Deploys were smooth")
	user := daotest.CreateUser(t, g.repos)

	// when
	claimed, err := g.repos.Board.ClaimGuestParticipant(t.Context(), spec.GuestClaimWrite{BoardWrite: g.write(t), GuestParticipantID: seat.ID, UserID: user.ID})

	// then the seat, and so the card, now belong to the user, and the device holds nothing here
	require.NoError(t, err)
	assert.Equal(t, seat.ID, claimed.Value.ID)
	require.NotNil(t, claimed.Value.UserID)
	assert.Equal(t, user.ID, *claimed.Value.UserID)
	assert.Nil(t, claimed.Value.GuestDeviceID)
	assert.Empty(t, claimed.Value.DisplayName)

	kept, err := g.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: g.board.Header.ID, CardID: card.ID})
	require.NoError(t, err)
	assert.Equal(t, seat.ID, *kept.AuthorParticipantID)

	_, guestErr := g.repos.Board.GetParticipantForGuest(t.Context(), spec.BoardParticipantGuest{BoardID: g.board.Header.ID, GuestDeviceID: device.ID})
	require.ErrorIs(t, guestErr, dao.ErrNotFound)
}

func TestClaim_AGuestsCardsMoveToTheUsersExistingSeat(t *testing.T) {
	// given the creator joined the board as a guest from another browser too
	g := newGuestBoard(t)
	device, _ := daotest.CreateGuest(t, g.repos)
	guestSeat := daotest.CreateGuestParticipant(t, g.repos, g.board.Header, device.ID)
	card := g.card(t, guestSeat.ID, "Standups ran long")
	ownerSeat := g.board.Participants[0]
	before := g.header(t).Version

	// when
	claimed, err := g.repos.Board.ClaimGuestParticipant(t.Context(), spec.GuestClaimWrite{BoardWrite: g.write(t), GuestParticipantID: guestSeat.ID, UserID: g.owner.ID, UserParticipantID: new(ownerSeat.ID)})

	// then a named-board claim keeps the guest's cards, under the user's own seat, in one version
	require.NoError(t, err)
	assert.Equal(t, ownerSeat.ID, claimed.Value.ID)
	assert.Equal(t, before+1, claimed.Version)

	kept, err := g.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: g.board.Header.ID, CardID: card.ID})
	require.NoError(t, err)
	assert.Equal(t, ownerSeat.ID, *kept.AuthorParticipantID)

	participants, err := g.repos.Board.ListParticipants(t.Context(), g.board.Header.ID)
	require.NoError(t, err)
	for _, participant := range participants {
		assert.NotEqual(t, guestSeat.ID, participant.ID, "the guest seat is gone")
	}
}

func TestClaim_AFailedClaimBurnsNoVersion(t *testing.T) {
	// given a claim that names a seat the board does not hold
	g := newGuestBoard(t)
	before := g.header(t).Version

	// when
	_, err := g.repos.Board.ClaimGuestParticipant(t.Context(), spec.GuestClaimWrite{BoardWrite: g.write(t), GuestParticipantID: uuid.New(), UserID: g.owner.ID, UserParticipantID: new(g.board.Participants[0].ID)})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, before, g.header(t).Version)
}

func TestGuest_CrossTenantSeatsAndSharingAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given a guest seated on board A, and organisation B with its own board
	bt := newBoardTenants(t)
	device, _ := daotest.CreateGuest(t, bt.repos)
	seat := daotest.CreateGuestParticipant(t, bt.repos, bt.boardA.Header, device.ID)
	throughB := bt.boardAThroughB()
	hash := "$2a$10$planted"

	// when
	_, seatErr := bt.repos.Board.GetParticipantForGuest(t.Context(), spec.BoardParticipantGuest{BoardID: bt.boardB.Header.ID, GuestDeviceID: device.ID})
	_, accessErr := bt.repos.Board.SetBoardLinkAccess(t.Context(), spec.BoardLinkAccessWrite{BoardWrite: throughB, LinkAccess: model.LinkAccessGuests})
	_, passwordErr := bt.repos.Board.SetBoardPassword(t.Context(), spec.BoardPasswordWrite{BoardWrite: bt.boardAThroughB(), PasswordHash: &hash})
	_, rotateErr := bt.repos.Board.RotateBoardLink(t.Context(), spec.BoardLinkRotationWrite{BoardWrite: bt.boardAThroughB(), ShareSlug: "PLANTEDSLUGPLANTEDSLUG23"})
	_, claimErr := bt.repos.Board.ClaimGuestParticipant(t.Context(), spec.GuestClaimWrite{BoardWrite: bt.boardAThroughB(), GuestParticipantID: seat.ID, UserID: bt.alice.ID})
	moved, moveErr := bt.repos.Card.ReassignAuthor(t.Context(), spec.CardAuthorReassignment{BoardID: bt.boardB.Header.ID, FromParticipantID: bt.boardA.Participants[0].ID, ToParticipantID: seat.ID})
	_, reassignErr := bt.repos.Board.ReassignParticipant(t.Context(), spec.ParticipantReassignment{BoardID: bt.boardB.Header.ID, ParticipantID: seat.ID, UserID: bt.alice.ID})
	deleteErr := bt.repos.Board.DeleteParticipant(t.Context(), spec.BoardParticipantKey{BoardID: bt.boardB.Header.ID, ParticipantID: seat.ID})

	// then nothing of board A moves through organisation B or board B
	require.ErrorIs(t, seatErr, dao.ErrNotFound)
	require.ErrorIs(t, accessErr, dao.ErrNotFound)
	require.ErrorIs(t, passwordErr, dao.ErrNotFound)
	require.ErrorIs(t, rotateErr, dao.ErrNotFound)
	require.ErrorIs(t, claimErr, dao.ErrNotFound)
	require.NoError(t, moveErr)
	assert.Zero(t, moved)
	require.ErrorIs(t, reassignErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	bt.boardAUnchangedButForTheGuest(t, seat.ID, device.ID)
}

func (bt boardTenants) boardAUnchangedButForTheGuest(t *testing.T, seatID uuid.UUID, deviceID uuid.UUID) {
	t.Helper()

	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, model.LinkAccessOff, header.LinkAccess)
	assert.Nil(t, header.PasswordHash)
	assert.Equal(t, 0, header.AccessEpoch)
	assert.Equal(t, bt.boardA.Header.ShareSlug, header.ShareSlug)
	assert.Equal(t, int64(1), header.Version)

	seat, err := bt.repos.Board.GetParticipantForGuest(t.Context(), spec.BoardParticipantGuest{BoardID: bt.boardA.Header.ID, GuestDeviceID: deviceID})
	require.NoError(t, err)
	assert.Equal(t, seatID, seat.ID)
	assert.Nil(t, seat.UserID)

	cards, err := bt.repos.Card.List(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, []model.Card{bt.cardA}, cards)
}
