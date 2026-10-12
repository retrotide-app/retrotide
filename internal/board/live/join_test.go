package live

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type (
	joinMocks struct {
		liveMocks

		settings *settings.MockService
		guests   *MockGuestIssuer
	}
)

const (
	joinSlug       = "K3V9X2QPLMNORSTUVWXYZ234"
	joinPassword   = "correct horse"
	joinGuestToken = "6b8d0f2a4c6e8b1d3f5a7c9e2b4d6f8a1c3e5b7d9f2a4c6e8b1d3f5a7c9e2b4d"
	joinCap        = 150
)

func newJoinLive(t *testing.T) (*service, *joinMocks) {
	t.Helper()

	m := &joinMocks{
		boards: repository.NewMockBoardRepository(t), orgAuthz: orgauthz.NewMockService(t),
		settings: settings.NewMockService(t),
		guests:   NewMockGuestIssuer(t),
	}

	svc := New(Deps{Boards: m.boards, OrgAuthz: m.orgAuthz, Settings: m.settings, Guests: m.guests}).(*service)

	return svc, m
}

func sharedHeader(t *testing.T, access model.LinkAccess, password string) model.BoardHeader {
	t.Helper()

	header := liveHeader()
	header.ShareSlug = joinSlug
	header.LinkAccess = access
	header.OrgAllowsGuests = true

	if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		require.NoError(t, err)
		header.PasswordHash = new(string(hashed))
	}

	return header
}

func (m *joinMocks) shared(header model.BoardHeader) {
	m.boards.EXPECT().GetHeaderBySlug(mock.Anything, joinSlug).Return(&header, nil)
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil).Maybe()
}

func (m *joinMocks) seats(header model.BoardHeader, seated int) {
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingMaxBoardParticipants).Return(joinCap, nil)
	m.boards.EXPECT().CountLiveParticipants(mock.Anything, spec.BoardSeatCount{BoardID: header.ID, AccessEpoch: header.AccessEpoch}).Return(seated, nil)
}

func (m *joinMocks) guestRow(header model.BoardHeader, deviceID uuid.UUID, row *model.BoardParticipant) {
	key := spec.BoardParticipantGuest{BoardID: header.ID, GuestDeviceID: deviceID}
	if row == nil {
		m.boards.EXPECT().GetParticipantForGuest(mock.Anything, key).Return(nil, errNoRow)
		return
	}

	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, key).Return(row, nil)
}

func guestSeat(header model.BoardHeader, deviceID uuid.UUID) *model.BoardParticipant {
	return &model.BoardParticipant{
		ID:            uuid.New(),
		BoardID:       header.ID,
		GuestDeviceID: new(deviceID),
		DisplayName:   "Gil",
		Role:          model.BoardRoleParticipant,
		AccessEpoch:   header.AccessEpoch,
		JoinedVia:     model.JoinPathLink,
	}
}

func TestPreviewLink_AnswersAVisitorWithoutNamingTheBoard(t *testing.T) {
	tests := []struct {
		name     string
		access   model.LinkAccess
		password string
		guests   bool
		want     dto.JoinPreview
	}{
		{name: "an open guest link", access: model.LinkAccessGuests, guests: true, want: dto.JoinPreview{CanJoin: true}},
		{name: "a guest link behind a password", access: model.LinkAccessGuests, password: joinPassword, guests: true, want: dto.JoinPreview{RequiresPassword: true, CanJoin: true}},
		{name: "an accounts-only link", access: model.LinkAccessAccounts, guests: true, want: dto.JoinPreview{RequiresAccount: true}},
		{name: "an organisation that turned guests off", access: model.LinkAccessGuests, guests: false, want: dto.JoinPreview{RequiresAccount: true}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newJoinLive(t)
			header := sharedHeader(t, tc.access, tc.password)
			header.OrgAllowsGuests = tc.guests
			m.shared(header)

			// when
			preview, err := svc.PreviewLink(t.Context(), LinkQuery{Slug: joinSlug})

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, *preview)
		})
	}
}

func TestPreviewLink_RefusesWhatNoVisitorMayReach(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(t *testing.T, m *joinMocks)
		want    error
	}{
		{name: "an unknown slug", arrange: func(_ *testing.T, m *joinMocks) {
			m.boards.EXPECT().GetHeaderBySlug(mock.Anything, joinSlug).Return(nil, errNotFound)
		}, want: ErrNoAccess},
		{name: "a deleted board", arrange: func(t *testing.T, m *joinMocks) {
			header := sharedHeader(t, model.LinkAccessGuests, "")
			header.DeletedAt = new(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
			m.shared(header)
		}, want: ErrNoAccess},
		{name: "an archived board", arrange: func(t *testing.T, m *joinMocks) {
			header := sharedHeader(t, model.LinkAccessGuests, "")
			header.ArchivedAt = new(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
			m.shared(header)
		}, want: ErrArchived},
		{name: "a link that is off", arrange: func(t *testing.T, m *joinMocks) {
			m.shared(sharedHeader(t, model.LinkAccessOff, ""))
		}, want: ErrLinkDisabled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newJoinLive(t)
			tc.arrange(t, m)

			// when
			preview, err := svc.PreviewLink(t.Context(), LinkQuery{Slug: joinSlug})

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, preview)
		})
	}
}

func TestPreviewLink_LetsASeatedGuestStraightBackIn(t *testing.T) {
	// given a guest already seated at the current epoch of a password-protected board
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
	m.shared(header)
	deviceID := uuid.New()
	m.guestRow(header, deviceID, guestSeat(header, deviceID))
	principal := model.Principal{Kind: model.PrincipalGuest, ID: deviceID}

	// when
	preview, err := svc.PreviewLink(t.Context(), LinkQuery{Principal: &principal, Slug: joinSlug})

	// then the password is not asked again
	require.NoError(t, err)
	assert.Equal(t, dto.JoinPreview{CanJoin: true}, *preview)
}

func TestPreviewLink_TellsARemovedGuestSo(t *testing.T) {
	// given
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, "")
	m.shared(header)
	deviceID := uuid.New()
	removed := guestSeat(header, deviceID)
	removed.RemovedAt = new(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	m.guestRow(header, deviceID, removed)
	principal := model.Principal{Kind: model.PrincipalGuest, ID: deviceID}

	// when
	_, err := svc.PreviewLink(t.Context(), LinkQuery{Principal: &principal, Slug: joinSlug})

	// then
	require.ErrorIs(t, err, ErrRemoved)
}

func TestJoinByLink_LetsAVisitorInAsANewGuest(t *testing.T) {
	// given a visitor with no principal on an open guest link
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
	m.shared(header)
	m.seats(header, 12)
	deviceID := uuid.New()
	m.guests.EXPECT().Create(mock.Anything).Return(model.GuestCredential{Token: joinGuestToken, DeviceID: deviceID}, nil)
	seated := guestSeat(header, deviceID)
	seated.DisplayName = "A name that runs well past forty runes of"
	want := spec.LinkParticipantJoin{
		OrgID: header.OrgID, BoardID: header.ID, Version: header.Version,
		GuestDeviceID: new(deviceID),
		DisplayName:   "A name that runs well past forty runes o",
		AccessEpoch:   header.AccessEpoch,
	}
	m.boards.EXPECT().JoinLinkParticipant(mock.Anything, want).Return(&model.Versioned[model.BoardParticipant]{Value: *seated, Version: header.Version + 1}, nil)
	m.guestRow(header, deviceID, seated)

	// when
	joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug, DisplayName: "  A name that runs well past forty runes of text  ", Password: joinPassword})

	// then the visitor holds a guest device from now on, and the gate is released
	require.NoError(t, err)
	assert.Equal(t, joinGuestToken, joined.GuestToken)
	assert.Equal(t, dto.LinkJoinResponse{BoardID: header.ID}, joined.Response)
	assert.Empty(t, svc.gates.byID)
}

func TestJoinByLink_RefusesBeforeAnyGuestDeviceIsIssued(t *testing.T) {
	tests := []struct {
		name     string
		access   model.LinkAccess
		guests   bool
		password string
		typed    string
		want     error
	}{
		{name: "a link that is off", access: model.LinkAccessOff, guests: true, want: ErrLinkDisabled},
		{name: "an accounts-only link", access: model.LinkAccessAccounts, guests: true, want: ErrAccountRequired},
		{name: "an organisation that turned guests off", access: model.LinkAccessGuests, guests: false, want: ErrGuestsNotAllowed},
		{name: "a missing password", access: model.LinkAccessGuests, guests: true, password: joinPassword, want: ErrPasswordRequired},
		{name: "a wrong password", access: model.LinkAccessGuests, guests: true, password: joinPassword, typed: "wrong horse", want: ErrWrongPassword},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the strict guest issuer and repository prove nothing is issued or written
			svc, m := newJoinLive(t)
			header := sharedHeader(t, tc.access, tc.password)
			header.OrgAllowsGuests = tc.guests
			m.shared(header)

			// when
			joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug, Password: tc.typed})

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, joined)
		})
	}
}

func TestJoinByLink_SpentPasswordBudgetRefusesWithARetryDelayWithoutComparing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a slug whose wrong guesses have spent its budget
		svc, m := newJoinLive(t)
		header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
		m.shared(header)

		for range PasswordFailureBurst {
			_, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug, Password: "wrong horse"})
			require.ErrorIs(t, err, ErrWrongPassword)
		}

		// when even the right password arrives
		_, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug, Password: joinPassword})

		// then
		retry, ok := errors.AsType[*RetryError](err)
		require.True(t, ok)
		require.ErrorIs(t, err, ErrPasswordAttempts)
		assert.InDelta(t, PasswordFailureRefill.Seconds(), retry.RetryAfter.Seconds(), 0.001)
	})
}

func TestJoinByLink_ARightPasswordChargesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a whole team joining a password-protected board at once
		svc, m := newJoinLive(t)
		header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
		m.shared(header)
		m.settings.EXPECT().GetInt(mock.Anything, config.SettingMaxBoardParticipants).Return(joinCap, nil)
		m.boards.EXPECT().CountLiveParticipants(mock.Anything, spec.BoardSeatCount{BoardID: header.ID, AccessEpoch: header.AccessEpoch}).Return(1, nil)
		m.guests.EXPECT().Create(mock.Anything).RunAndReturn(func(context.Context) (model.GuestCredential, error) {
			return model.GuestCredential{Token: joinGuestToken, DeviceID: uuid.New()}, nil
		})
		m.boards.EXPECT().JoinLinkParticipant(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, join spec.LinkParticipantJoin, _ ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
			return &model.Versioned[model.BoardParticipant]{Value: *guestSeat(header, *join.GuestDeviceID), Version: header.Version + 1}, nil
		})
		m.boards.EXPECT().GetParticipantForGuest(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key spec.BoardParticipantGuest, _ ...*sql.Tx) (*model.BoardParticipant, error) {
			return guestSeat(header, key.GuestDeviceID), nil
		})

		// when
		for range PasswordFailureBurst + 5 {
			_, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug, Password: joinPassword})
			require.NoError(t, err)
		}

		// then no successful join drew on the failure budget
		for range PasswordFailureBurst {
			assert.Zero(t, svc.passwords.take(header.ID, time.Now()))
		}
	})
}

func TestJoinByLink_ConcurrentWrongGuessesNeverOutrunTheBudget(t *testing.T) {
	// given a fresh board whose budget holds PasswordFailureBurst tokens
	budget := newPasswordBudget()
	now := time.Now()
	board := uuid.New()

	// when every guess takes its token before any comparison ends
	granted := 0
	for range PasswordFailureBurst * 3 {
		if budget.take(board, now) == 0 {
			granted++
		}
	}

	// then only the burst is compared, and a right password gives its token back
	assert.Equal(t, PasswordFailureBurst, granted)
	budget.refund(board, now)
	assert.Zero(t, budget.take(board, now))
	assert.Positive(t, budget.take(board, now))
}

func TestJoinByLink_ANewLinkKeepsTheSpentPasswordBudget(t *testing.T) {
	// given a board whose wrong guesses spent its budget under the old link
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
	for range PasswordFailureBurst {
		svc.passwords.take(header.ID, time.Now())
	}
	header.ShareSlug = "ROTATEDSLUGROTATEDSLUG2345"
	m.boards.EXPECT().GetHeaderBySlug(mock.Anything, header.ShareSlug).Return(&header, nil)
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil).Maybe()

	// when someone guesses through the new link
	_, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: header.ShareSlug, Password: "guess"})

	// then rotating the link never resets the backoff
	require.ErrorIs(t, err, ErrPasswordAttempts)
}

func TestJoinByLink_AVisitorMeetsALinkNarrowedDuringTheJoin(t *testing.T) {
	tests := []struct {
		name   string
		narrow func(header *model.BoardHeader)
		want   error
	}{
		{name: "the link switched off", narrow: func(header *model.BoardHeader) {
			header.LinkAccess = model.LinkAccessOff
		}, want: ErrLinkDisabled},
		{name: "the link limited to accounts", narrow: func(header *model.BoardHeader) {
			header.LinkAccess = model.LinkAccessAccounts
		}, want: ErrAccountRequired},
		{name: "the organisation turned guests off", narrow: func(header *model.BoardHeader) {
			header.OrgAllowsGuests = false
		}, want: ErrGuestsNotAllowed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given an open link read before the gate, and a narrowing committed before the gate was taken
			svc, m := newJoinLive(t)
			header := sharedHeader(t, model.LinkAccessGuests, "")
			m.boards.EXPECT().GetHeaderBySlug(mock.Anything, joinSlug).Return(&header, nil)
			narrowed := header
			tc.narrow(&narrowed)
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&narrowed, nil)

			// when
			joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug})

			// then the strict guest issuer and repository prove no device and no seat were written
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, joined)
		})
	}
}

func TestJoinByLink_RefusesANewSeatOnAFullBoard(t *testing.T) {
	// given
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, "")
	m.shared(header)
	m.seats(header, joinCap)

	// when
	joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug})

	// then no guest device is issued for a seat that cannot exist
	require.ErrorIs(t, err, ErrBoardFull)
	assert.Nil(t, joined)
}

func TestJoinByLink_ASeatedGuestRejoinsWithoutAWrite(t *testing.T) {
	// given
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
	m.shared(header)
	deviceID := uuid.New()
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: header.ID, GuestDeviceID: deviceID}).Return(guestSeat(header, deviceID), nil)
	principal := model.Principal{Kind: model.PrincipalGuest, ID: deviceID}

	// when
	joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Principal: &principal, Slug: joinSlug})

	// then the strict repository proves nothing was written and no cookie is issued
	require.NoError(t, err)
	assert.Empty(t, joined.GuestToken)
	assert.Equal(t, header.ID, joined.Response.BoardID)
}

func TestJoinByLink_AGuestBehindARotatedEpochRejoinsAtTheNewOne(t *testing.T) {
	// given a guest who knows the new slug after the board's password was cleared, so its seat is a step behind
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, "")
	m.shared(header)
	deviceID := uuid.New()
	stale := guestSeat(header, deviceID)
	stale.AccessEpoch = header.AccessEpoch - 1
	fresh := guestSeat(header, deviceID)
	fresh.ID = stale.ID
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: header.ID, GuestDeviceID: deviceID}).Return(stale, nil).Times(3)
	m.boards.EXPECT().JoinLinkParticipant(mock.Anything, mock.MatchedBy(func(join spec.LinkParticipantJoin) bool {
		return *join.GuestDeviceID == deviceID && join.AccessEpoch == header.AccessEpoch && join.UserID == nil
	})).Return(&model.Versioned[model.BoardParticipant]{Value: *fresh, Version: header.Version + 1}, nil)
	m.boards.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: header.ID, GuestDeviceID: deviceID}).Return(fresh, nil)
	principal := model.Principal{Kind: model.PrincipalGuest, ID: deviceID}

	// when
	joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Principal: &principal, Slug: joinSlug, DisplayName: "Gil"})

	// then the existing seat needs no space under the cap and no new device
	require.NoError(t, err)
	assert.Empty(t, joined.GuestToken)
}

func TestJoinByLink_AMemberIsSeatedThroughMembershipWithoutThePassword(t *testing.T) {
	// given a team member who opened the board's share link
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
	m.shared(header)
	principal := user()
	m.standing(header, principal, teamMember)
	m.standing(header, principal, teamMember)
	m.participant(header, principal, nil)
	m.participant(header, principal, nil)
	joinedRow := storedRow(header, principal, model.BoardRoleParticipant)
	want := spec.BoardParticipantJoin{OrgID: header.OrgID, BoardID: header.ID, Version: header.Version, UserID: principal.ID, Role: model.BoardRoleParticipant}
	m.boards.EXPECT().JoinParticipant(mock.Anything, want).Return(&model.Versioned[model.BoardParticipant]{Value: *joinedRow, Version: header.Version + 1}, nil)

	// when
	joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Principal: &principal, Slug: joinSlug})

	// then
	require.NoError(t, err)
	assert.Empty(t, joined.GuestToken)
	assert.Equal(t, header.ID, joined.Response.BoardID)
}

func TestJoinByLink_RefusesWhenTheLinkRotatesDuringTheJoin(t *testing.T) {
	// given the slug read before the gate, and a rotation committed before the gate was taken
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, "")
	m.boards.EXPECT().GetHeaderBySlug(mock.Anything, joinSlug).Return(&header, nil)
	rotated := header
	rotated.ShareSlug = "NEWSLUGNEWSLUGNEWSLUG234"
	rotated.AccessEpoch++
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&rotated, nil)

	// when
	joined, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug})

	// then the old link lets nobody in, and no device is issued
	require.ErrorIs(t, err, ErrNoAccess)
	assert.Nil(t, joined)
}

func TestJoinByLink_RefusesWhenThePasswordChangesDuringTheJoin(t *testing.T) {
	// given the password checked before the gate, and a new password committed before the gate was taken
	svc, m := newJoinLive(t)
	header := sharedHeader(t, model.LinkAccessGuests, joinPassword)
	m.boards.EXPECT().GetHeaderBySlug(mock.Anything, joinSlug).Return(&header, nil)
	changed := sharedHeader(t, model.LinkAccessGuests, "another password")
	changed.ID = header.ID
	changed.OrgID = header.OrgID
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&changed, nil)

	// when
	_, err := svc.JoinByLink(t.Context(), LinkJoinCommand{Slug: joinSlug, Password: joinPassword})

	// then
	require.ErrorIs(t, err, ErrPasswordRequired)
}

func TestCleanGuestName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "trimmed", raw: "  Gil  ", want: "Gil"},
		{name: "empty means anonymous", raw: "   ", want: ""},
		{name: "clamped by runes, not bytes", raw: "Zoë Zoë Zoë Zoë Zoë Zoë Zoë Zoë Zoë Zoë Zoë", want: "Zoë Zoë Zoë Zoë Zoë Zoë Zoë Zoë Zoë Zoë"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := cleanGuestName(tc.raw)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
