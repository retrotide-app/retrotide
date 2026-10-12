package auth

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testInvitedEmail = "carol@example.com"
	testClientIP     = "198.51.100.7"
)

func pendingInvitation() *model.Invitation {
	return &model.Invitation{
		ID:        uuid.New(),
		Email:     testInvitedEmail,
		TokenHash: token.Hash(testToken),
		ExpiresAt: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
	}
}

func validAcceptance() spec.InvitationAcceptance {
	return spec.InvitationAcceptance{Token: testToken, DisplayName: testDisplayName, Password: testPassword, IP: testClientIP}
}

func (m *testMocks) invitedAccountReady() spec.NewUser {
	account := spec.NewUser{Email: testInvitedEmail, PasswordHash: new(testPasswordHash), DisplayName: testDisplayName}
	m.users.EXPECT().NewAccountSpec(mock.Anything, spec.NewAccount{Email: testInvitedEmail, Password: testPassword, DisplayName: testDisplayName}).Return(account, nil)

	return account
}

func (m *testMocks) redeems(created *model.User) *spec.InvitationRedemption {
	captured := &spec.InvitationRedemption{}
	m.invitations.EXPECT().Redeem(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, redemption spec.InvitationRedemption, _ ...*sql.Tx) (*model.User, error) {
		*captured = redemption

		return created, nil
	})

	return captured
}

func TestGetInvitation_ShowsTheInvitedAddressByTheTokensHash(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(pendingInvitation(), nil)

	// when
	invitation, err := svc.GetInvitation(t.Context(), testToken)

	// then the page shows the address it will create, which the visitor cannot change
	require.NoError(t, err)
	assert.Equal(t, &dto.InvitationResponse{Email: testInvitedEmail, ExpiresAt: "2026-10-03T12:00:00Z"}, invitation)
}

func TestGetInvitation_RefusesAnEmptyUnknownUsedOrExpiredToken(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		expect func(m *testMocks)
	}{
		{name: "empty", token: "", expect: func(*testMocks) {}},
		{
			name:  "unknown, used or expired, which the live lookup never finds",
			token: testToken,
			expect: func(m *testMocks) {
				m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(nil, fmt.Errorf("get invitation by token: %w", dao.ErrNotFound))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.expect(m)

			// when
			invitation, err := svc.GetInvitation(t.Context(), tc.token)

			// then
			require.ErrorIs(t, err, ErrInvalidInvitation)
			assert.Nil(t, invitation)
		})
	}
}

func TestGetInvitation_ReportsAFailedLookup(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(nil, errDatabaseDown)

	// when
	_, err := svc.GetInvitation(t.Context(), testToken)

	// then a failure is never mistaken for an invalid link
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrInvalidInvitation)
}

func TestAcceptInvitation_CreatesAVerifiedAccountForTheInvitedAddressWithoutASession(t *testing.T) {
	// given
	svc, m := newTestService(t)
	pending := pendingInvitation()
	m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(pending, nil)
	account := m.invitedAccountReady()
	created := newVerifiedUser(testInvitedEmail)
	redemption := m.redeems(created)

	var audited audit.NewEntry
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, entry audit.NewEntry, _ ...*sql.Tx) error {
		audited = entry

		return nil
	})

	// when
	accepted, err := svc.AcceptInvitation(t.Context(), validAcceptance())

	// then the invitation is spent in the transaction that creates the account, and no session or email follows, which the strict mocks would report
	require.NoError(t, err)
	assert.Equal(t, &dto.InvitationAcceptedResponse{Email: testInvitedEmail}, accepted)
	assert.Equal(t, spec.InvitationRedemption{TokenHash: token.Hash(testToken), User: account}, *redemption)

	assert.Equal(t, audit.NewEntry{
		ActorID:    created.ID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionInvitationAccepted,
		TargetType: audit.TargetInvitation,
		TargetID:   pending.ID.String(),
		SubjectID:  created.ID,
		Details:    map[string]string{"email": testInvitedEmail},
		IP:         testClientIP,
	}, audited)
}

func TestAcceptInvitation_RefusesAnInvalidLinkBeforeAnythingElse(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(nil, fmt.Errorf("get invitation by token: %w", dao.ErrNotFound))

	// when
	accepted, err := svc.AcceptInvitation(t.Context(), validAcceptance())

	// then no password is hashed and nothing is written
	require.ErrorIs(t, err, ErrInvalidInvitation)
	assert.Nil(t, accepted)
}

func TestAcceptInvitation_NeedsADisplayName(t *testing.T) {
	for _, name := range []string{"", "   ", "<b></b>"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(pendingInvitation(), nil)

			acceptance := validAcceptance()
			acceptance.DisplayName = name

			// when
			_, err := svc.AcceptInvitation(t.Context(), acceptance)

			// then the invitation stays unspent
			require.ErrorIs(t, err, user.ErrDisplayNameRequired)
		})
	}
}

func TestAcceptInvitation_LeavesThePasswordRulesToTheUserService(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(pendingInvitation(), nil)
	m.users.EXPECT().NewAccountSpec(mock.Anything, mock.Anything).Return(spec.NewUser{}, user.ErrPasswordTooShort)

	// when
	_, err := svc.AcceptInvitation(t.Context(), validAcceptance())

	// then the invitation stays unspent
	require.ErrorIs(t, err, user.ErrPasswordTooShort)
}

func TestAcceptInvitation_MapsWhatTheRedemptionMeets(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "the invitation was used or expired since it was read", err: fmt.Errorf("redeem invitation: consume invitation: %w", dao.ErrNotFound), want: ErrInvalidInvitation},
		{name: "an account took the address since it was invited", err: fmt.Errorf("redeem invitation: create user: %w", dao.ErrConflict), want: ErrEmailTaken},
		{name: "an unexpected failure", err: fmt.Errorf("redeem invitation: %w", errDatabaseDown), want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(pendingInvitation(), nil)
			m.invitedAccountReady()
			m.invitations.EXPECT().Redeem(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			accepted, err := svc.AcceptInvitation(t.Context(), validAcceptance())

			// then nothing is audited, which the strict audit mock would report
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, accepted)
		})
	}
}

func TestAcceptInvitation_StillAnswersWhenTheAuditEntryFails(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitations.EXPECT().GetLiveByTokenHash(mock.Anything, token.Hash(testToken)).Return(pendingInvitation(), nil)
	m.invitedAccountReady()
	m.redeems(newVerifiedUser(testInvitedEmail))
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	accepted, err := svc.AcceptInvitation(t.Context(), validAcceptance())

	// then the account exists, so the caller answers success and records the cause
	require.ErrorIs(t, err, ErrNotAudited)
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, &dto.InvitationAcceptedResponse{Email: testInvitedEmail}, accepted)
}
