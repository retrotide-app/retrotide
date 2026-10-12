package admin

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/token"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testInvitedEmail = "carol@example.com"
)

var (
	invitationLink = regexp.MustCompile(`https://retrotide\.test/accept-invitation#([0-9a-f]{64})`)
)

func storedInvitation(expiresAt time.Time) *model.Invitation {
	return &model.Invitation{
		ID:        uuid.New(),
		Email:     testInvitedEmail,
		TokenHash: token.Hash("an-earlier-token"),
		ExpiresAt: expiresAt,
		CreatedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC),
	}
}

func invitationEntryFor(actorID uuid.UUID, action audit.Action, invitation *model.Invitation) audit.NewEntry {
	return audit.NewEntry{
		ActorID:    actorID,
		Action:     action,
		TargetType: audit.TargetInvitation,
		TargetID:   invitation.ID.String(),
		Details:    map[string]string{"email": invitation.Email},
	}
}

func (m *testMocks) invitable(address string) {
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.users.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: address}).Return(false, nil)
}

func (m *testMocks) sendsInvitation(err error) *[]email.Message {
	sent := &[]email.Message{}

	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, message email.Message) error {
		*sent = append(*sent, message)

		return err
	})

	return sent
}

func (m *testMocks) createsInvitation() *spec.NewInvitation {
	captured := &spec.NewInvitation{}

	m.invitations.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewInvitation, _ ...*sql.Tx) (*model.Invitation, error) {
		*captured = s

		return &model.Invitation{ID: uuid.New(), Email: s.Email, TokenHash: s.TokenHash, ExpiresAt: s.ExpiresAt, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	})

	return captured
}

func linkedInvitationToken(t *testing.T, message email.Message) string {
	t.Helper()

	match := invitationLink.FindStringSubmatch(message.HTML)
	require.Len(t, match, 2, "the invitation email carries no accept-invitation link")

	return match[1]
}

func TestListInvitations_ClampsThePageAndMarksTheExpiredOnes(t *testing.T) {
	// given one invitation still live and one past its seven days
	svc, m := newTestService(t)
	live := storedInvitation(time.Now().Add(time.Hour))
	lapsed := storedInvitation(time.Now().Add(-time.Hour))
	m.invitations.EXPECT().List(mock.Anything, spec.InvitationListing{Limit: maxPageSize, Offset: 0}).Return([]model.Invitation{*live, *lapsed}, nil)
	m.invitations.EXPECT().Count(mock.Anything).Return(2, nil)

	// when
	listed, err := svc.ListInvitations(t.Context(), spec.InvitationListing{Limit: 5000, Offset: -3})

	// then an expired invitation stays listed until it is resent or cancelled
	require.NoError(t, err)
	assert.Equal(t, 2, listed.Total)
	assert.Equal(t, maxPageSize, listed.Limit)
	assert.Equal(t, 0, listed.Offset)
	require.Len(t, listed.Invitations, 2)
	assert.Equal(t, live.ID, listed.Invitations[0].ID)
	assert.Equal(t, testInvitedEmail, listed.Invitations[0].Email)
	assert.False(t, listed.Invitations[0].Expired)
	assert.Equal(t, "2026-09-02T12:00:00Z", listed.Invitations[0].SentAt)
	assert.Equal(t, "2026-09-01T12:00:00Z", listed.Invitations[0].CreatedAt)
	assert.True(t, listed.Invitations[1].Expired)
}

func TestListInvitations_WorksOnAPublicSiteAndDefaultsThePage(t *testing.T) {
	// given no site mode read, which the strict settings mock would report
	svc, m := newTestService(t)
	m.invitations.EXPECT().List(mock.Anything, spec.InvitationListing{Limit: defaultInvitationPageSize, Offset: 40}).Return([]model.Invitation{}, nil)
	m.invitations.EXPECT().Count(mock.Anything).Return(0, nil)

	// when
	listed, err := svc.ListInvitations(t.Context(), spec.InvitationListing{Offset: 40})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.AdminInvitationListResponse{Invitations: []dto.AdminInvitationResponse{}, Limit: defaultInvitationPageSize, Offset: 40}, listed)
}

func TestListInvitations_ReturnsAReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitations.EXPECT().List(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	listed, err := svc.ListInvitations(t.Context(), spec.InvitationListing{})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listed)
}

func TestInviteUser_StoresOnlyTheTokensHashForSevenDaysAndEmailsTheLink(t *testing.T) {
	// given a private site
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.invitable(testInvitedEmail)
	created := m.createsInvitation()
	sent := m.sendsInvitation(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, entry audit.NewEntry, _ ...*sql.Tx) error {
		assert.Equal(t, audit.ActionInviteUser, entry.Action)
		assert.Equal(t, audit.TargetInvitation, entry.TargetType)
		assert.Equal(t, audit.ActorStaff, entry.ActorKind)
		assert.Equal(t, actorID, entry.ActorID)
		assert.Equal(t, testIP, entry.IP)
		assert.Equal(t, map[string]string{"email": testInvitedEmail}, entry.Details)

		return nil
	})

	// when
	invited, err := svc.InviteUser(t.Context(), spec.StaffInvitation{StaffAction: staffAction(actorID), Email: " " + testInvitedEmail + " "})

	// then the address is trimmed, and the token itself exists only in the email
	require.NoError(t, err)
	assert.Equal(t, testInvitedEmail, created.Email)
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), created.ExpiresAt, time.Minute)
	assert.Equal(t, testInvitedEmail, invited.Email)
	assert.False(t, invited.Expired)

	require.Len(t, *sent, 1)
	assert.Equal(t, testInvitedEmail, (*sent)[0].To)
	assert.Equal(t, created.TokenHash, token.Hash(linkedInvitationToken(t, (*sent)[0])))
}

func TestInviteUser_RefusesBeforeWritingAnything(t *testing.T) {
	tests := []struct {
		name    string
		address string
		arrange func(m *testMocks)
		want    error
	}{
		{
			name:    "an invalid address",
			address: "not an address",
			arrange: func(*testMocks) {},
			want:    ErrInvalidEmail,
		},
		{
			name:    "a public site, where anyone can register instead",
			address: testInvitedEmail,
			arrange: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
			},
			want: ErrSitePublic,
		},
		{
			name:    "no email provider",
			address: testInvitedEmail,
			arrange: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
				m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)
			},
			want: ErrEmailUnavailable,
		},
		{
			name:    "an address that already has an account",
			address: testInvitedEmail,
			arrange: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testInvitedEmail}).Return(true, nil)
			},
			want: ErrAccountExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.arrange(m)

			// when
			invited, err := svc.InviteUser(t.Context(), spec.StaffInvitation{StaffAction: staffAction(uuid.New()), Email: tc.address})

			// then no invitation, email or audit entry is made, which the strict mocks would report
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, invited)
		})
	}
}

func TestInviteUser_ReportsWhatItCannotCheck(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(m *testMocks)
	}{
		{
			name: "the site mode",
			arrange: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)
			},
		},
		{
			name: "the email provider",
			arrange: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
				m.email.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)
			},
		},
		{
			name: "whether the address has an account",
			arrange: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().EmailInUse(mock.Anything, mock.Anything).Return(false, errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.arrange(m)

			// when
			_, err := svc.InviteUser(t.Context(), spec.StaffInvitation{StaffAction: staffAction(uuid.New()), Email: testInvitedEmail})

			// then a failed check is never read as a pass
			require.ErrorIs(t, err, errDatabaseDown)
		})
	}
}

func TestInviteUser_AnAddressWithAPendingInvitationIsRefused(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitable(testInvitedEmail)
	m.invitations.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("create invitation: %w", dao.ErrConflict))

	// when
	invited, err := svc.InviteUser(t.Context(), spec.StaffInvitation{StaffAction: staffAction(uuid.New()), Email: testInvitedEmail})

	// then staff resend the one that exists instead
	require.ErrorIs(t, err, ErrInvitationPending)
	assert.Nil(t, invited)
}

func TestInviteUser_AFailedEmailIsReportedAfterTheInvitationIsSaved(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.invitable(testInvitedEmail)
	m.createsInvitation()
	m.sendsInvitation(errDatabaseDown)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	// when
	invited, err := svc.InviteUser(t.Context(), spec.StaffInvitation{StaffAction: staffAction(actorID), Email: testInvitedEmail})

	// then the invitation is listed and audited, and staff are told to resend it
	require.ErrorIs(t, err, ErrInvitationNotSent)
	require.ErrorIs(t, err, errDatabaseDown)
	require.NotNil(t, invited)
	assert.Equal(t, testInvitedEmail, invited.Email)
}

func TestInviteUser_AFailedAuditEntryStillSendsTheEmail(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.invitable(testInvitedEmail)
	m.createsInvitation()
	sent := m.sendsInvitation(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	invited, err := svc.InviteUser(t.Context(), spec.StaffInvitation{StaffAction: staffAction(uuid.New()), Email: testInvitedEmail})

	// then
	require.ErrorIs(t, err, ErrNotAudited)
	assert.NotNil(t, invited)
	assert.Len(t, *sent, 1)
}

func TestResendInvitation_DrawsANewTokenAndExpiryAndEmailsIt(t *testing.T) {
	// given an invitation that lapsed
	svc, m := newTestService(t)
	actorID := uuid.New()
	pending := storedInvitation(time.Now().Add(-time.Hour))
	m.invitations.EXPECT().GetByID(mock.Anything, pending.ID).Return(pending, nil)
	m.invitable(testInvitedEmail)

	var renewal spec.InvitationRenewal
	m.invitations.EXPECT().Renew(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.InvitationRenewal, _ ...*sql.Tx) (*model.Invitation, error) {
		renewal = s
		renewed := *pending
		renewed.TokenHash = s.TokenHash
		renewed.ExpiresAt = s.ExpiresAt

		return &renewed, nil
	})
	sent := m.sendsInvitation(nil)
	m.audited(invitationEntryFor(actorID, audit.ActionResendInvitation, pending))

	// when
	err := svc.ResendInvitation(t.Context(), spec.StaffInvitationAction{StaffAction: staffAction(actorID), InvitationID: pending.ID})

	// then the earlier link stops working, because its hash is replaced
	require.NoError(t, err)
	assert.Equal(t, pending.ID, renewal.ID)
	assert.NotEqual(t, pending.TokenHash, renewal.TokenHash)
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), renewal.ExpiresAt, time.Minute)
	require.Len(t, *sent, 1)
	assert.Equal(t, renewal.TokenHash, token.Hash(linkedInvitationToken(t, (*sent)[0])))
}

func TestResendInvitation_RefusesBeforeRenewing(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(m *testMocks, pending *model.Invitation)
		want    error
	}{
		{
			name: "an unknown invitation",
			arrange: func(m *testMocks, pending *model.Invitation) {
				m.invitations.EXPECT().GetByID(mock.Anything, pending.ID).Return(nil, fmt.Errorf("get invitation: %w", dao.ErrNotFound))
			},
			want: ErrInvitationNotFound,
		},
		{
			name: "a public site",
			arrange: func(m *testMocks, pending *model.Invitation) {
				m.invitations.EXPECT().GetByID(mock.Anything, pending.ID).Return(pending, nil)
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
			},
			want: ErrSitePublic,
		},
		{
			name: "an address that has since registered",
			arrange: func(m *testMocks, pending *model.Invitation) {
				m.invitations.EXPECT().GetByID(mock.Anything, pending.ID).Return(pending, nil)
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testInvitedEmail}).Return(true, nil)
			},
			want: ErrAccountExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			pending := storedInvitation(time.Now().Add(time.Hour))
			tc.arrange(m, pending)

			// when
			err := svc.ResendInvitation(t.Context(), spec.StaffInvitationAction{StaffAction: staffAction(uuid.New()), InvitationID: pending.ID})

			// then nothing is renewed, sent or audited
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestResendInvitation_AnInvitationCancelledMeanwhileIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	pending := storedInvitation(time.Now().Add(time.Hour))
	m.invitations.EXPECT().GetByID(mock.Anything, pending.ID).Return(pending, nil)
	m.invitable(testInvitedEmail)
	m.invitations.EXPECT().Renew(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("renew invitation: %w", dao.ErrNotFound))

	// when
	err := svc.ResendInvitation(t.Context(), spec.StaffInvitationAction{StaffAction: staffAction(uuid.New()), InvitationID: pending.ID})

	// then
	require.ErrorIs(t, err, ErrInvitationNotFound)
}

func TestResendInvitation_AFailedEmailIsReportedAfterTheRenewal(t *testing.T) {
	// given
	svc, m := newTestService(t)
	pending := storedInvitation(time.Now().Add(time.Hour))
	m.invitations.EXPECT().GetByID(mock.Anything, pending.ID).Return(pending, nil)
	m.invitable(testInvitedEmail)
	m.invitations.EXPECT().Renew(mock.Anything, mock.Anything).Return(pending, nil)
	m.sendsInvitation(errDatabaseDown)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	// when
	err := svc.ResendInvitation(t.Context(), spec.StaffInvitationAction{StaffAction: staffAction(uuid.New()), InvitationID: pending.ID})

	// then
	require.ErrorIs(t, err, ErrInvitationNotSent)
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestCancelInvitation_DeletesAndAuditsTheAddressOnEitherSiteMode(t *testing.T) {
	// given no site mode read, which the strict settings mock would report
	svc, m := newTestService(t)
	actorID := uuid.New()
	pending := storedInvitation(time.Now().Add(time.Hour))
	m.invitations.EXPECT().Delete(mock.Anything, pending.ID).Return(pending, nil)
	m.audited(invitationEntryFor(actorID, audit.ActionCancelInvitation, pending))

	// when
	err := svc.CancelInvitation(t.Context(), spec.StaffInvitationAction{StaffAction: staffAction(actorID), InvitationID: pending.ID})

	// then its link stops working, because the row is gone
	require.NoError(t, err)
}

func TestCancelInvitation_AnUnknownInvitationIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	invitationID := uuid.New()
	m.invitations.EXPECT().Delete(mock.Anything, invitationID).Return(nil, fmt.Errorf("delete invitation: %w", dao.ErrNotFound))

	// when
	err := svc.CancelInvitation(t.Context(), spec.StaffInvitationAction{StaffAction: staffAction(uuid.New()), InvitationID: invitationID})

	// then
	require.ErrorIs(t, err, ErrInvitationNotFound)
}

func TestCancelInvitation_ReturnsADeleteFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	invitationID := uuid.New()
	m.invitations.EXPECT().Delete(mock.Anything, invitationID).Return(nil, errDatabaseDown)

	// when
	err := svc.CancelInvitation(t.Context(), spec.StaffInvitationAction{StaffAction: staffAction(uuid.New()), InvitationID: invitationID})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrInvitationNotFound)
}
