package auth

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/token"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	testMocks struct {
		users         *user.MockService
		authz         *authz.MockService
		settings      *settings.MockService
		email         *email.MockService
		userRepo      *repository.MockUserRepository
		sessionRepo   *repository.MockSessionRepository
		verifications *repository.MockEmailVerificationRepository
		resets        *repository.MockPasswordResetRepository
		invitations   *repository.MockInvitationRepository
		auditLog      *repository.MockAuditLogRepository
		organisations *org.MockService
		disconnector  *session.MockDisconnector
		twoFactor     *twofactor.MockService
		sent          []email.Message
	}
)

const (
	testBaseURL      = "https://retrotide.test/"
	testSiteName     = "RetroTide"
	testEmail        = "alice@example.com"
	testNewEmail     = "alice@work.example"
	testPassword     = "correct horse battery"
	testNewPassword  = "a brand new passphrase"
	testDisplayName  = "Alice"
	testToken        = "5a4b3c2d1e0f5a4b3c2d1e0f5a4b3c2d1e0f5a4b3c2d1e0f5a4b3c2d1e0f5a4b"
	testSessionDays  = 30
	testPasswordHash = "$2a$10$pretendhashforthetestsonlypretendhashforthetestsonly"
	tokenPattern     = `#([0-9a-f]{64})`
	testDeviceToken  = "9e8d7c6b5a4f3e2d1c0b9e8d7c6b5a4f3e2d1c0b9e8d7c6b5a4f3e2d1c0b9e8d"
)

var (
	testSignIn = spec.PasswordSignIn{UserCredentials: spec.UserCredentials{Email: testEmail, Password: testPassword}, Remember: true}
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	svc, m := newBareTestService(t)
	m.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn).Return(true, nil).Maybe()

	return svc, m
}

func newPasswordSignInService(t *testing.T, allowed bool, readErr error) (*service, *testMocks) {
	t.Helper()

	svc, m := newBareTestService(t)
	m.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn).Return(allowed, readErr)

	return svc, m
}

func newBareTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		users:         user.NewMockService(t),
		authz:         authz.NewMockService(t),
		settings:      settings.NewMockService(t),
		email:         email.NewMockService(t),
		userRepo:      repository.NewMockUserRepository(t),
		sessionRepo:   repository.NewMockSessionRepository(t),
		verifications: repository.NewMockEmailVerificationRepository(t),
		resets:        repository.NewMockPasswordResetRepository(t),
		invitations:   repository.NewMockInvitationRepository(t),
		auditLog:      repository.NewMockAuditLogRepository(t),
		organisations: org.NewMockService(t),
		disconnector:  session.NewMockDisconnector(t),
		twoFactor:     twofactor.NewMockService(t),
	}

	sessions := session.NewManager(m.sessionRepo, m.settings)
	sessions.SetDisconnector(m.disconnector)

	svc := NewService(Deps{
		Users:         m.users,
		Authz:         m.authz,
		Sessions:      sessions,
		Settings:      m.settings,
		Email:         m.email,
		UserRepo:      m.userRepo,
		Verifications: m.verifications,
		Resets:        m.resets,
		Invitations:   m.invitations,
		AuditLog:      m.auditLog,
		Organisations: m.organisations,
		TwoFactor:     m.twoFactor,
	}).(*service)

	return svc, m
}

func newRegistrationService(t *testing.T, mode config.SiteMode) (*service, *testMocks) {
	t.Helper()

	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(mode), nil)

	if mode == config.SiteModePublic {
		m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	}

	return svc, m
}

func newMaillessRegistrationService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)

	return svc, m
}

func newEstablishedRegistrationService(t *testing.T, mode config.SiteMode) (*service, *testMocks) {
	t.Helper()

	svc, m := newRegistrationService(t, mode)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)

	return svc, m
}

func newFreshRegistrationService(t *testing.T, mode config.SiteMode) (*service, *testMocks) {
	t.Helper()

	svc, m := newRegistrationService(t, mode)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, nil)

	return svc, m
}

func newMailingService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	svc, m := newTestService(t)
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)

	return svc, m
}

func (m *testMocks) site() {
	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
}

func (m *testMocks) accountReady(address string) spec.NewUser {
	account := spec.NewUser{Email: address, PasswordHash: new(testPasswordHash), DisplayName: testDisplayName}

	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: address}).Return(false, nil)
	m.users.EXPECT().NewAccountSpec(mock.Anything, spec.NewAccount{Email: address, Password: testPassword, DisplayName: testDisplayName}).Return(account, nil)

	return account
}

func (m *testMocks) registers(created *model.User) *spec.NewRegistration {
	captured := &spec.NewRegistration{}
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, registration spec.NewRegistration, _ ...*sql.Tx) (*model.User, error) {
		*captured = registration

		return created, nil
	})

	return captured
}

func delivers(m *testMocks) {
	m.email.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, message email.Message) error {
		m.sent = append(m.sent, message)

		return nil
	})
}

func linkedToken(t *testing.T, message email.Message, page string) string {
	t.Helper()

	match := regexp.MustCompile(regexp.QuoteMeta("https://retrotide.test/"+page) + tokenPattern).FindStringSubmatch(message.HTML)
	require.NotNil(t, match, "the email to %s carries no %s link", message.To, page)

	return match[1]
}

func validRegistration() spec.AccountRegistration {
	return spec.AccountRegistration{Email: testEmail, Password: testPassword, DisplayName: testDisplayName}
}

func newUser(address string) *model.User {
	return &model.User{ID: uuid.New(), Email: address, PasswordHash: new(testPasswordHash), DisplayName: testDisplayName, CreatedAt: time.Now()}
}

func newVerifiedUser(address string) *model.User {
	created := newUser(address)
	created.EmailVerified = true

	return created
}

func verifiedAccount() *dto.UserResponse {
	return &dto.UserResponse{ID: uuid.New(), Email: testEmail, EmailVerified: true}
}

func (m *testMocks) storesSession() *spec.NewSession {
	stored := &spec.NewSession{}
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testSessionDays, nil)
	m.sessionRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewSession, _ ...*sql.Tx) error {
		*stored = s

		return nil
	})

	return stored
}

func TestRegistrationOpen_IsOpenOnAPublicSiteThatCanSendEmail(t *testing.T) {
	// given
	svc, _ := newRegistrationService(t, config.SiteModePublic)

	// when
	open, err := svc.RegistrationOpen(t.Context())

	// then no account lookup is needed, which the strict repository mock would report
	require.NoError(t, err)
	assert.True(t, open)
}

func TestRegistrationOpen_OnAPublicSiteWithoutEmailFollowsTheFirstAccount(t *testing.T) {
	tests := []struct {
		name        string
		established bool
		want        bool
	}{
		{name: "an empty site lets its first account register", established: false, want: true},
		{name: "a site with an account is closed", established: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newMaillessRegistrationService(t)
			m.userRepo.EXPECT().AnyExists(mock.Anything).Return(tc.established, nil)

			// when
			open, err := svc.RegistrationOpen(t.Context())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, open)
		})
	}
}

func TestRegistrationOpen_ReopensOnceAnEmailProviderIsSaved(t *testing.T) {
	// given a public site with an account, whose email provider is saved between the two reads
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	m.email.EXPECT().Enabled(mock.Anything).Return(false, nil).Once()
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil).Once()
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil).Once()

	// when
	before, beforeErr := svc.RegistrationOpen(t.Context())
	after, afterErr := svc.RegistrationOpen(t.Context())

	// then
	require.NoError(t, beforeErr)
	require.NoError(t, afterErr)
	assert.False(t, before)
	assert.True(t, after)
}

func TestRegistrationOpen_OnAPrivateSiteFollowsTheFirstAccount(t *testing.T) {
	tests := []struct {
		name        string
		established bool
		want        bool
	}{
		{name: "an empty site lets its first account register", established: false, want: true},
		{name: "a site with an account is closed", established: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newRegistrationService(t, config.SiteModePrivate)
			m.userRepo.EXPECT().AnyExists(mock.Anything).Return(tc.established, nil)

			// when
			open, err := svc.RegistrationOpen(t.Context())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, open)
		})
	}
}

func TestRegistrationOpen_ReportsWhatItCannotRead(t *testing.T) {
	tests := []struct {
		name   string
		expect func(m *testMocks)
	}{
		{
			name: "the site mode",
			expect: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)
			},
		},
		{
			name: "whether any account exists",
			expect: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
				m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, errDatabaseDown)
			},
		},
		{
			name: "the email provider",
			expect: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
				m.email.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.expect(m)

			// when
			open, err := svc.RegistrationOpen(t.Context())

			// then a failed read is never taken for open
			require.ErrorIs(t, err, errDatabaseDown)
			assert.False(t, open)
		})
	}
}

func TestRegister_APublicSiteCreatesTheAccountAndTheVerificationButNoSession(t *testing.T) {
	// given
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	account := m.accountReady(testEmail)
	created := newUser(testEmail)
	registration := m.registers(created)
	m.site()
	delivers(m)

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then the answer says to check the inbox, and no session is issued, which the strict session mock would report
	require.NoError(t, err)
	assert.Equal(t, &dto.RegistrationResponse{Email: testEmail, VerificationRequired: true}, response)

	assert.Equal(t, account, registration.User)
	assert.False(t, registration.FirstAccount)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), registration.VerificationExpiresAt, time.Minute)

	require.Len(t, m.sent, 1)
	assert.Equal(t, testEmail, m.sent[0].To)
	assert.Contains(t, m.sent[0].HTML, "If you did not create this account, ignore this email.")
	assert.Equal(t, token.Hash(linkedToken(t, m.sent[0], "verify-email")), registration.VerificationHash)
}

func TestRegister_TrimsTheEmailAddress(t *testing.T) {
	// given
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	m.accountReady(testEmail)
	m.registers(newUser(testEmail))
	m.site()
	delivers(m)

	registration := validRegistration()
	registration.Email = "  " + testEmail + "\t"

	// when
	_, err := svc.Register(t.Context(), registration)

	// then
	require.NoError(t, err)
}

func TestRegister_APrivateSiteRefusesRegistrationOnceAnAccountExists(t *testing.T) {
	// given
	svc, _ := newEstablishedRegistrationService(t, config.SiteModePrivate)

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then nothing is written and no email is sent, which the strict mocks would report
	require.ErrorIs(t, err, ErrRegistrationPrivate)
	assert.Nil(t, response)
}

func TestRegister_APublicSiteWithoutEmailRefusesRegistrationOnceAnAccountExists(t *testing.T) {
	// given
	svc, m := newMaillessRegistrationService(t)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then nothing is written and no email is sent, which the strict mocks would report
	require.ErrorIs(t, err, ErrRegistrationClosed)
	assert.Nil(t, response)
}

func TestRegister_APublicSiteWithoutEmailStillCreatesItsFirstAccount(t *testing.T) {
	// given no account exists yet, and the repository creates the first one verified
	svc, m := newMaillessRegistrationService(t)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, nil)
	m.accountReady(testEmail)
	registration := m.registers(newVerifiedUser(testEmail))

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then it can sign in at once, and the mailer is never asked to send, which its strict mock would report
	require.NoError(t, err)
	assert.Equal(t, &dto.RegistrationResponse{Email: testEmail, VerificationRequired: false}, response)
	assert.True(t, registration.FirstAccount)
}

func TestRegister_LosingTheFirstAccountRaceWithoutEmailIsRefused(t *testing.T) {
	// given another registration claimed the empty site between the check and the transaction
	svc, m := newMaillessRegistrationService(t)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, nil)
	m.accountReady(testEmail)
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.MatchedBy(func(registration spec.NewRegistration) bool {
		return registration.FirstAccount
	})).Return(nil, fmt.Errorf("register account: %w", repository.ErrNotFirstAccount)).Once()

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then nothing more is written and no email is sent
	require.ErrorIs(t, err, ErrRegistrationClosed)
	assert.Nil(t, response)
}

func TestRegister_FailsWhenTheEmailProviderCannotBeChecked(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	m.email.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then a failed check never reads as open or closed, and nothing is written
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrRegistrationClosed)
}

func TestRegister_AnUnknownSiteModeAdmitsNobody(t *testing.T) {
	// given
	svc, _ := newRegistrationService(t, "friends")

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrRegistrationPrivate)
	assert.Contains(t, err.Error(), `unknown site mode "friends"`)
}

func TestRegister_FailsWhenTheSiteModeCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestRegister_RefusesAnInvalidEmailAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{name: "empty", address: ""},
		{name: "only spaces", address: "   "},
		{name: "no at sign", address: "alice.example.com"},
		{name: "a display name", address: "Alice <alice@example.com>"},
		{name: "two addresses", address: "alice@example.com, bob@example.com"},
		{name: "longer than 254 characters", address: strings.Repeat("a", 243) + "@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newEstablishedRegistrationService(t, config.SiteModePublic)

			registration := validRegistration()
			registration.Email = tc.address

			// when
			_, err := svc.Register(t.Context(), registration)

			// then
			require.ErrorIs(t, err, ErrInvalidEmail)
		})
	}
}

func TestRegister_RefusesAnEmailAddressAlreadyInUse(t *testing.T) {
	// given
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testEmail}).Return(true, nil)

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then
	require.ErrorIs(t, err, ErrEmailTaken)
}

func TestRegister_FailsWhenTheEmailCannotBeChecked(t *testing.T) {
	// given
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testEmail}).Return(false, errDatabaseDown)

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrEmailTaken)
}

func TestRegister_AnEmailTakenMeanwhileIsATakenEmail(t *testing.T) {
	// given
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	m.accountReady(testEmail)
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("register account: %w", dao.ErrConflict))

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then
	require.ErrorIs(t, err, ErrEmailTaken)
}

func TestRegister_ReturnsAWriteFailure(t *testing.T) {
	// given
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	m.accountReady(testEmail)
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("register account: %w", errDatabaseDown))

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then no email is sent, which the strict mailer mock would report
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrEmailTaken)
	assert.Nil(t, response)
}

func TestRegister_LeavesThePasswordRulesToTheUserService(t *testing.T) {
	// given
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testEmail}).Return(false, nil)
	m.users.EXPECT().NewAccountSpec(mock.Anything, mock.Anything).Return(spec.NewUser{}, user.ErrPasswordTooShort)

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then nothing is written for a password that breaks the rules
	require.ErrorIs(t, err, user.ErrPasswordTooShort)
}

func TestRegister_ReturnsTheAnswerWhenTheVerificationEmailFails(t *testing.T) {
	// given the account is committed before the email goes out
	svc, m := newEstablishedRegistrationService(t, config.SiteModePublic)
	m.accountReady(testEmail)
	m.registers(newUser(testEmail))
	m.site()
	m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errSMTPRefused)

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then the caller still reports the account, and learns why the email is missing
	require.ErrorIs(t, err, ErrEmailNotSent)
	require.ErrorIs(t, err, errSMTPRefused)
	assert.Equal(t, &dto.RegistrationResponse{Email: testEmail, VerificationRequired: true}, response)
}

func TestRegister_TheFirstAccountClaimsSuperAdminWhateverTheSiteMode(t *testing.T) {
	tests := []struct {
		name string
		mode config.SiteMode
	}{
		{name: "a public site", mode: config.SiteModePublic},
		{name: "a private site", mode: config.SiteModePrivate},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given no account exists yet, and the repository creates the first one verified
			svc, m := newFreshRegistrationService(t, tc.mode)
			account := m.accountReady(testEmail)
			registration := m.registers(newVerifiedUser(testEmail))

			// when
			response, err := svc.Register(t.Context(), validRegistration())

			// then it can sign in at once, and the mailer is never used, which its strict mock would report
			require.NoError(t, err)
			assert.Equal(t, &dto.RegistrationResponse{Email: testEmail, VerificationRequired: false}, response)
			assert.Equal(t, account, registration.User)
			assert.True(t, registration.FirstAccount)
		})
	}
}

func TestRegister_FailsWhenItCannotTellWhetherAnyAccountExists(t *testing.T) {
	// given a private site, which admits only its first account
	svc, m := newRegistrationService(t, config.SiteModePrivate)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, errDatabaseDown)

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then a failed check never reads as an empty site, and nothing is written
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrRegistrationPrivate)
}

func TestRegister_LosingTheFirstAccountRaceOnAPublicSiteRegistersAnOrdinaryAccount(t *testing.T) {
	// given another registration claimed the empty site between the check and the transaction
	svc, m := newFreshRegistrationService(t, config.SiteModePublic)
	m.accountReady(testEmail)
	created := newUser(testEmail)

	var attempts []spec.NewRegistration
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, registration spec.NewRegistration, _ ...*sql.Tx) (*model.User, error) {
		attempts = append(attempts, registration)

		if registration.FirstAccount {
			return nil, fmt.Errorf("register account: %w", repository.ErrNotFirstAccount)
		}

		return created, nil
	})
	m.site()
	delivers(m)

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then the second attempt is an ordinary account, which must verify its address before it signs in
	require.NoError(t, err)
	assert.Equal(t, &dto.RegistrationResponse{Email: created.Email, VerificationRequired: true}, response)
	require.Len(t, attempts, 2)
	assert.True(t, attempts[0].FirstAccount)
	assert.False(t, attempts[1].FirstAccount)
	assert.Equal(t, attempts[0].VerificationHash, attempts[1].VerificationHash)
	require.Len(t, m.sent, 1)
	assert.Equal(t, token.Hash(linkedToken(t, m.sent[0], "verify-email")), attempts[1].VerificationHash)
}

func TestRegister_LosingTheFirstAccountRaceOnAPrivateSiteIsRefused(t *testing.T) {
	// given another registration claimed the empty site between the check and the transaction
	svc, m := newFreshRegistrationService(t, config.SiteModePrivate)
	m.accountReady(testEmail)
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.MatchedBy(func(registration spec.NewRegistration) bool {
		return registration.FirstAccount
	})).Return(nil, fmt.Errorf("register account: %w", repository.ErrNotFirstAccount)).Once()

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then nothing more is written and no email is sent
	require.ErrorIs(t, err, ErrRegistrationPrivate)
	assert.Nil(t, response)
}

func TestRegister_LosingTheFirstAccountRaceToTheSameAddressIsATakenEmail(t *testing.T) {
	// given the winner registered the same address
	svc, m := newFreshRegistrationService(t, config.SiteModePublic)
	m.accountReady(testEmail)
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.MatchedBy(func(registration spec.NewRegistration) bool {
		return registration.FirstAccount
	})).Return(nil, fmt.Errorf("register account: %w", repository.ErrNotFirstAccount)).Once()
	m.userRepo.EXPECT().RegisterAccount(mock.Anything, mock.MatchedBy(func(registration spec.NewRegistration) bool {
		return !registration.FirstAccount
	})).Return(nil, fmt.Errorf("register account: %w", dao.ErrConflict)).Once()

	// when
	_, err := svc.Register(t.Context(), validRegistration())

	// then
	require.ErrorIs(t, err, ErrEmailTaken)
}

func TestRegister_RefusesWhilePasswordSignInIsOff(t *testing.T) {
	// given
	svc, _ := newPasswordSignInService(t, false, nil)

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then the site mode and the address are never read, which the strict mocks would report
	require.ErrorIs(t, err, ErrPasswordSignInDisabled)
	assert.Nil(t, response)
}

func TestRegister_FailsClosedWhenTheSwitchCannotBeRead(t *testing.T) {
	// given
	svc, _ := newPasswordSignInService(t, true, errDatabaseDown)

	// when
	response, err := svc.Register(t.Context(), validRegistration())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, response)
}

func TestLogin_CreatesASessionForValidCredentials(t *testing.T) {
	// given a verified account, whose role is never read, which the strict authz mock would report
	svc, m := newTestService(t)
	account := verifiedAccount()
	m.users.EXPECT().ValidateCredentials(mock.Anything, spec.UserCredentials{Email: testEmail, Password: testPassword}).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.twoFactor.EXPECT().Challenge(mock.Anything, spec.TwoFactorPending{UserID: account.ID, Remember: true}).Return("", nil)
	stored := m.storesSession()
	signIn := testSignIn
	signIn.Email = " " + testEmail + " "

	// when
	result, err := svc.Login(t.Context(), signIn)

	// then
	require.NoError(t, err)
	assert.Equal(t, account, result.User)
	assert.Equal(t, result.SessionToken, stored.Token)
	assert.Empty(t, result.ChallengeToken)
	assert.Equal(t, account.ID, stored.UserID)
	assert.WithinDuration(t, time.Now().Add(testSessionDays*24*time.Hour), stored.ExpiresAt, time.Minute)
}

func TestLogin_WithoutRememberMeTheSessionLastsADay(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := verifiedAccount()
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.twoFactor.EXPECT().Challenge(mock.Anything, spec.TwoFactorPending{UserID: account.ID}).Return("", nil)

	stored := &spec.NewSession{}
	m.sessionRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewSession, _ ...*sql.Tx) error {
		*stored = s

		return nil
	})

	signIn := testSignIn
	signIn.Remember = false

	// when
	result, err := svc.Login(t.Context(), signIn)

	// then the configured duration is never read, which the strict settings mock would report
	require.NoError(t, err)
	assert.Equal(t, result.SessionToken, stored.Token)
	assert.WithinDuration(t, time.Now().Add(session.TransientLifetime), stored.ExpiresAt, time.Minute)
}

func TestLogin_AnAccountWithTwoFactorGetsAChallengeAndNoSession(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := verifiedAccount()
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.twoFactor.EXPECT().Challenge(mock.Anything, spec.TwoFactorPending{UserID: account.ID, Remember: true, TrustedDeviceToken: testDeviceToken}).Return(testToken, nil)
	signIn := testSignIn
	signIn.TrustedDeviceToken = testDeviceToken

	// when
	result, err := svc.Login(t.Context(), signIn)

	// then no session is created, which the strict session mock would report
	require.NoError(t, err)
	assert.Equal(t, account, result.User)
	assert.Equal(t, testToken, result.ChallengeToken)
	assert.Empty(t, result.SessionToken)
}

func TestLogin_FailsWhenTheSecondFactorCannotBeChecked(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := verifiedAccount()
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.twoFactor.EXPECT().Challenge(mock.Anything, mock.Anything).Return("", errDatabaseDown)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, result)
}

func TestLogin_RefusesWhilePasswordSignInIsOff(t *testing.T) {
	// given
	svc, _ := newPasswordSignInService(t, false, nil)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then the credentials are never checked, which the strict user mock would report
	require.ErrorIs(t, err, ErrPasswordSignInDisabled)
	assert.Nil(t, result)
}

func TestLogin_FailsClosedWhenTheSwitchCannotBeRead(t *testing.T) {
	// given
	svc, _ := newPasswordSignInService(t, true, errDatabaseDown)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrPasswordSignInDisabled)
	assert.Nil(t, result)
}

func TestLogin_RefusesTheRightPasswordOnAnUnverifiedAccount(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := &dto.UserResponse{ID: uuid.New(), Email: testEmail}
	m.users.EXPECT().ValidateCredentials(mock.Anything, spec.UserCredentials{Email: testEmail, Password: testPassword}).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, account.ID).Return(false, nil)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then no challenge or session is created, which the strict mocks would report
	require.ErrorIs(t, err, ErrEmailUnverified)
	assert.Nil(t, result)
}

func TestLogin_LetsAnUnverifiedSuperAdminIn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := &dto.UserResponse{ID: uuid.New(), Email: testEmail}
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, account.ID).Return(true, nil)
	m.twoFactor.EXPECT().Challenge(mock.Anything, spec.TwoFactorPending{UserID: account.ID, Remember: true}).Return("", nil)
	stored := m.storesSession()

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then
	require.NoError(t, err)
	assert.Equal(t, result.SessionToken, stored.Token)
	assert.Equal(t, account.ID, stored.UserID)
}

func TestLogin_FailsClosedWhenTheExemptionCannotBeChecked(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := &dto.UserResponse{ID: uuid.New(), Email: testEmail}
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, account.ID).Return(false, errDatabaseDown)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, result)
}

func TestLogin_RefusesInvalidCredentialsWithoutASession(t *testing.T) {
	// given an unknown email and a wrong password look the same from here
	svc, m := newTestService(t)
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(nil, user.ErrInvalidCredentials)
	signIn := testSignIn
	signIn.Password = "wrong"

	// when
	result, err := svc.Login(t.Context(), signIn)

	// then
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
	assert.Nil(t, result)
}

func TestLogin_RefusesABannedUser(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := verifiedAccount()
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(true, nil)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then
	require.ErrorIs(t, err, ErrUserBanned)
	assert.Nil(t, result)
}

func TestLogin_FailsClosedWhenTheBanCannotBeChecked(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := verifiedAccount()
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(true, errDatabaseDown)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, result)
}

func TestLogin_FailsWhenTheSessionCannotBeStored(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := verifiedAccount()
	m.users.EXPECT().ValidateCredentials(mock.Anything, mock.Anything).Return(account, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.twoFactor.EXPECT().Challenge(mock.Anything, mock.Anything).Return("", nil)
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(testSessionDays, nil)
	m.sessionRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	result, err := svc.Login(t.Context(), testSignIn)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, result)
}

func TestCompleteTwoFactor_CreatesTheSessionAndAnswersTheLandingPath(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := &model.User{ID: uuid.New(), Email: testEmail, PasswordHash: new(testPasswordHash), DisplayName: testDisplayName, EmailVerified: true}
	verification := spec.TwoFactorVerification{ChallengeToken: testToken, Code: "081804", TrustDevice: true}
	passed := twofactor.Passed{UserID: account.ID, ReturnTo: "/o/acme/boards", Remember: true, TrustedDeviceToken: testDeviceToken}
	m.twoFactor.EXPECT().Verify(mock.Anything, verification).Return(&passed, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	stored := m.storesSession()

	// when
	got, err := svc.CompleteTwoFactor(t.Context(), verification)

	// then the session remembers the choice made with the password, and the trusted device token is handed on
	require.NoError(t, err)
	assert.Equal(t, account.ID, stored.UserID)
	assert.Equal(t, stored.Token, got.SessionToken)
	assert.WithinDuration(t, time.Now().Add(testSessionDays*24*time.Hour), stored.ExpiresAt, time.Minute)
	assert.Equal(t, dto.NewUserResponse(account), got.User)
	assert.Equal(t, "/o/acme/boards", got.Location)
	assert.True(t, got.Remember)
	assert.Equal(t, testDeviceToken, got.TrustedDeviceToken)
}

func TestCompleteTwoFactor_WithoutRememberMeTheSessionLastsADay(t *testing.T) {
	// given
	svc, m := newTestService(t)
	account := &model.User{ID: uuid.New(), Email: testEmail, EmailVerified: true}
	m.twoFactor.EXPECT().Verify(mock.Anything, mock.Anything).Return(&twofactor.Passed{UserID: account.ID}, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, account.ID).Return(false, nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)

	stored := &spec.NewSession{}
	m.sessionRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewSession, _ ...*sql.Tx) error {
		*stored = s

		return nil
	})

	// when
	got, err := svc.CompleteTwoFactor(t.Context(), spec.TwoFactorVerification{ChallengeToken: testToken, Code: "081804"})

	// then
	require.NoError(t, err)
	assert.False(t, got.Remember)
	assert.Empty(t, got.TrustedDeviceToken)
	assert.WithinDuration(t, time.Now().Add(session.TransientLifetime), stored.ExpiresAt, time.Minute)
}

func TestCompleteTwoFactor_RefusedCodesCreateNoSession(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "a wrong code", err: twofactor.ErrInvalidCode},
		{name: "an expired challenge", err: twofactor.ErrChallengeExpired},
		{name: "a store failure", err: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.twoFactor.EXPECT().Verify(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			got, err := svc.CompleteTwoFactor(t.Context(), spec.TwoFactorVerification{ChallengeToken: testToken, Code: "123456"})

			// then the strict mocks see no ban check and no session
			require.ErrorIs(t, err, tc.err)
			assert.Nil(t, got)
		})
	}
}

func TestCompleteTwoFactor_RefusesAnAccountBannedDuringTheChallenge(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().Verify(mock.Anything, mock.Anything).Return(&twofactor.Passed{UserID: userID}, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)

	// when
	got, err := svc.CompleteTwoFactor(t.Context(), spec.TwoFactorVerification{ChallengeToken: testToken, Code: "081804"})

	// then no session is created
	require.ErrorIs(t, err, ErrUserBanned)
	assert.Nil(t, got)
}

func TestCompleteTwoFactor_AnAccountDeletedDuringTheChallengeHasExpired(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().Verify(mock.Anything, mock.Anything).Return(&twofactor.Passed{UserID: userID}, nil)
	m.authz.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, userID).Return(nil, fmt.Errorf("lookup: %w", dao.ErrNotFound))

	// when
	got, err := svc.CompleteTwoFactor(t.Context(), spec.TwoFactorVerification{ChallengeToken: testToken, Code: "081804"})

	// then
	require.ErrorIs(t, err, twofactor.ErrChallengeExpired)
	assert.Nil(t, got)
}

func TestLogout_DeletesThePresentedSession(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.sessionRepo.EXPECT().Delete(mock.Anything, testToken).Return(nil)

	// when
	err := svc.Logout(t.Context(), testToken)

	// then
	require.NoError(t, err)
}

func TestLogout_WithoutASessionDeletesNothing(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.Logout(t.Context(), "")

	// then
	require.NoError(t, err)
}

func TestLogout_ReportsAFailedDelete(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.sessionRepo.EXPECT().Delete(mock.Anything, testToken).Return(errDatabaseDown)

	// when
	err := svc.Logout(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestVerifyEmail_ConsumesTheTokenByItsHash(t *testing.T) {
	tests := []struct {
		name    string
		pending *string
	}{
		{name: "the registration link", pending: nil},
		{name: "a change of address", pending: new(testNewEmail)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.userRepo.EXPECT().ConfirmEmailVerification(mock.Anything, token.Hash(testToken)).Return(&model.EmailVerificationToken{UserID: uuid.New(), PendingEmail: tc.pending}, nil)

			// when
			err := svc.VerifyEmail(t.Context(), testToken)

			// then no session is created, which the strict session mock would report
			require.NoError(t, err)
		})
	}
}

func TestVerifyEmail_RefusesAnEmptyToken(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.VerifyEmail(t.Context(), "")

	// then
	require.ErrorIs(t, err, ErrInvalidVerificationToken)
}

func TestVerifyEmail_RefusesAnUnknownUsedOrExpiredToken(t *testing.T) {
	tests := []string{"unknown", "already used", "expired"}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			// given the single conditional consume matches no row in each case
			svc, m := newTestService(t)
			m.userRepo.EXPECT().ConfirmEmailVerification(mock.Anything, token.Hash(testToken)).Return(nil, fmt.Errorf("confirm email verification: %w", dao.ErrNotFound))

			// when
			err := svc.VerifyEmail(t.Context(), testToken)

			// then
			require.ErrorIs(t, err, ErrInvalidVerificationToken)
		})
	}
}

func TestVerifyEmail_APendingAddressTakenMeanwhileIsATakenEmail(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.userRepo.EXPECT().ConfirmEmailVerification(mock.Anything, token.Hash(testToken)).Return(nil, fmt.Errorf("confirm email verification: %w", dao.ErrConflict))

	// when
	err := svc.VerifyEmail(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, ErrEmailTaken)
}

func TestVerifyEmail_ReportsAnUnexpectedFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.userRepo.EXPECT().ConfirmEmailVerification(mock.Anything, token.Hash(testToken)).Return(nil, errDatabaseDown)

	// when
	err := svc.VerifyEmail(t.Context(), testToken)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrInvalidVerificationToken)
}

func TestResendVerification_IssuesAFreshRegistrationLinkToAnUnverifiedAccount(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	account := newUser(testEmail)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(account, nil)

	var issued spec.NewEmailVerification
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewEmailVerification, _ ...*sql.Tx) error {
		issued = s

		return nil
	})
	m.site()
	delivers(m)

	// when
	err := svc.ResendVerification(t.Context(), " "+testEmail+" ")

	// then
	require.NoError(t, err)
	assert.Equal(t, account.ID, issued.UserID)
	assert.Nil(t, issued.PendingEmail)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), issued.ExpiresAt, time.Minute)
	require.Len(t, m.sent, 1)
	assert.Equal(t, testEmail, m.sent[0].To)
	assert.Equal(t, issued.TokenHash, token.Hash(linkedToken(t, m.sent[0], "verify-email")))
}

func TestResendVerification_AnswersTheSameWhenThereIsNothingToSend(t *testing.T) {
	tests := []struct {
		name    string
		found   *model.User
		findErr error
	}{
		{name: "an unknown address", findErr: fmt.Errorf("get user by email: %w", dao.ErrNotFound)},
		{name: "an address already verified", found: newVerifiedUser(testEmail)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newMailingService(t)
			m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(tc.found, tc.findErr)

			// when
			err := svc.ResendVerification(t.Context(), testEmail)

			// then the caller cannot tell it apart from a sent link, and nothing is issued or sent, which the strict mocks would report
			require.NoError(t, err)
		})
	}
}

func TestResendVerification_RefusesWhileNoProviderIsConfigured(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)

	// when
	err := svc.ResendVerification(t.Context(), testEmail)

	// then the answer is the same for every address, so it reveals no account
	require.ErrorIs(t, err, ErrEmailUnavailable)
}

func TestResendVerification_RefusesAnInvalidAddress(t *testing.T) {
	// given
	svc, _ := newMailingService(t)

	// when
	err := svc.ResendVerification(t.Context(), "not an address")

	// then
	require.ErrorIs(t, err, ErrInvalidEmail)
}

func TestResendVerification_ReportsAFailedLookup(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(nil, errDatabaseDown)

	// when
	err := svc.ResendVerification(t.Context(), testEmail)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestResendVerification_ReportsAFailedIssue(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(newUser(testEmail), nil)
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	err := svc.ResendVerification(t.Context(), testEmail)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrVerificationNotSent)
}

func TestResendVerification_MarksAFailedSendAsNotSent(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(newUser(testEmail), nil)
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).Return(nil)
	m.site()
	m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errSMTPRefused)

	// when
	err := svc.ResendVerification(t.Context(), testEmail)

	// then the controller can answer as it would for an unknown address and still record why
	require.ErrorIs(t, err, ErrVerificationNotSent)
	require.ErrorIs(t, err, errSMTPRefused)
}

func TestForgotPassword_SendsAOneHourResetLinkToAKnownAccount(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	account := newUser(testEmail)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(account, nil)

	var issued spec.NewPasswordReset
	m.resets.EXPECT().Issue(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewPasswordReset, _ ...*sql.Tx) error {
		issued = s

		return nil
	})
	m.site()
	delivers(m)

	// when
	err := svc.ForgotPassword(t.Context(), " "+testEmail)

	// then
	require.NoError(t, err)
	assert.Equal(t, account.ID, issued.UserID)
	assert.WithinDuration(t, time.Now().Add(time.Hour), issued.ExpiresAt, time.Minute)
	require.Len(t, m.sent, 1)
	assert.Equal(t, testEmail, m.sent[0].To)
	assert.Equal(t, issued.TokenHash, token.Hash(linkedToken(t, m.sent[0], "reset-password")))
}

func TestForgotPassword_AnswersTheSameForAnUnknownAddress(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, "nobody@example.com").Return(nil, fmt.Errorf("get user by email: %w", dao.ErrNotFound))

	// when
	err := svc.ForgotPassword(t.Context(), "nobody@example.com")

	// then the caller cannot tell it apart from a sent link, and nothing is issued or sent
	require.NoError(t, err)
}

func TestForgotPassword_RefusesWhileNoProviderIsConfigured(t *testing.T) {
	addresses := []string{testEmail, "nobody@example.com", "not an address"}

	for _, address := range addresses {
		t.Run(address, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)

			// when
			err := svc.ForgotPassword(t.Context(), address)

			// then the answer is the same for every address and no account is looked up, which the strict repository mock would report
			require.ErrorIs(t, err, ErrEmailUnavailable)
			assert.Equal(t, ErrEmailUnavailable.Error(), err.Error())
		})
	}
}

func TestForgotPassword_FailsWhenTheProviderCannotBeChecked(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.email.EXPECT().Enabled(mock.Anything).Return(false, errSMTPRefused)

	// when
	err := svc.ForgotPassword(t.Context(), testEmail)

	// then
	require.ErrorIs(t, err, errSMTPRefused)
	assert.NotErrorIs(t, err, ErrEmailUnavailable)
}

func TestForgotPassword_RefusesAnInvalidAddress(t *testing.T) {
	// given
	svc, _ := newMailingService(t)

	// when
	err := svc.ForgotPassword(t.Context(), "not an address")

	// then
	require.ErrorIs(t, err, ErrInvalidEmail)
}

func TestForgotPassword_ReportsAFailedLookup(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(nil, errDatabaseDown)

	// when
	err := svc.ForgotPassword(t.Context(), testEmail)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestForgotPassword_ReportsAFailedSend(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testEmail).Return(newUser(testEmail), nil)
	m.resets.EXPECT().Issue(mock.Anything, mock.Anything).Return(nil)
	m.site()
	m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errSMTPRefused)

	// when
	err := svc.ForgotPassword(t.Context(), testEmail)

	// then
	require.ErrorIs(t, err, errSMTPRefused)
}

func TestForgotPassword_RefusesWhilePasswordSignInIsOff(t *testing.T) {
	addresses := []string{testEmail, "nobody@example.com", "not an address"}

	for _, address := range addresses {
		t.Run(address, func(t *testing.T) {
			// given
			svc, _ := newPasswordSignInService(t, false, nil)

			// when
			err := svc.ForgotPassword(t.Context(), address)

			// then the answer is the same for every address and nothing is looked up, which the strict mocks would report
			require.ErrorIs(t, err, ErrPasswordSignInDisabled)
			assert.Equal(t, ErrPasswordSignInDisabled.Error(), err.Error())
		})
	}
}

func TestForgotPassword_FailsClosedWhenTheSwitchCannotBeRead(t *testing.T) {
	// given
	svc, _ := newPasswordSignInService(t, true, errDatabaseDown)

	// when
	err := svc.ForgotPassword(t.Context(), testEmail)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestResetPassword_RefusesWhilePasswordSignInIsOff(t *testing.T) {
	// given
	svc, _ := newPasswordSignInService(t, false, nil)

	// when
	err := svc.ResetPassword(t.Context(), spec.PasswordResetCompletion{Token: testToken, NewPassword: testNewPassword})

	// then the token is never spent, which the strict repository mock would report
	require.ErrorIs(t, err, ErrPasswordSignInDisabled)
}

func TestResetPassword_FailsClosedWhenTheSwitchCannotBeRead(t *testing.T) {
	// given
	svc, _ := newPasswordSignInService(t, true, errDatabaseDown)

	// when
	err := svc.ResetPassword(t.Context(), spec.PasswordResetCompletion{Token: testToken, NewPassword: testNewPassword})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrPasswordSignInDisabled)
}

func TestResetPassword_SetsThePasswordRevokesEverySessionAndDisconnects(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().HashPassword(mock.Anything, testNewPassword).Return(testPasswordHash, nil)
	m.userRepo.EXPECT().ResetPassword(mock.Anything, spec.PasswordReset{TokenHash: token.Hash(testToken), PasswordHash: testPasswordHash}).Return(userID, nil)
	m.disconnector.EXPECT().DisconnectUser(userID).Return(2)

	// when
	err := svc.ResetPassword(t.Context(), spec.PasswordResetCompletion{Token: testToken, NewPassword: testNewPassword})

	// then the repository composite deletes every session in the same transaction, and the sockets drop
	require.NoError(t, err)
}

func TestResetPassword_RefusesAnEmptyToken(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.ResetPassword(t.Context(), spec.PasswordResetCompletion{NewPassword: testNewPassword})

	// then
	require.ErrorIs(t, err, ErrInvalidResetToken)
}

func TestResetPassword_AShortPasswordLeavesTheTokenUnspent(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.users.EXPECT().HashPassword(mock.Anything, "short").Return("", user.ErrPasswordTooShort)

	// when
	err := svc.ResetPassword(t.Context(), spec.PasswordResetCompletion{Token: testToken, NewPassword: "short"})

	// then the token is never consumed, so the same link still works for a longer password
	require.ErrorIs(t, err, user.ErrPasswordTooShort)
}

func TestResetPassword_RefusesAnUnknownUsedOrExpiredToken(t *testing.T) {
	tests := []string{"unknown", "already used", "expired"}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.users.EXPECT().HashPassword(mock.Anything, testNewPassword).Return(testPasswordHash, nil)
			m.userRepo.EXPECT().ResetPassword(mock.Anything, mock.Anything).Return(uuid.Nil, fmt.Errorf("reset password: %w", dao.ErrNotFound))

			// when
			err := svc.ResetPassword(t.Context(), spec.PasswordResetCompletion{Token: testToken, NewPassword: testNewPassword})

			// then nobody is disconnected
			require.ErrorIs(t, err, ErrInvalidResetToken)
		})
	}
}

func TestResetPassword_ReportsAFailedReset(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.users.EXPECT().HashPassword(mock.Anything, testNewPassword).Return(testPasswordHash, nil)
	m.userRepo.EXPECT().ResetPassword(mock.Anything, mock.Anything).Return(uuid.Nil, errDatabaseDown)

	// when
	err := svc.ResetPassword(t.Context(), spec.PasswordResetCompletion{Token: testToken, NewPassword: testNewPassword})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrInvalidResetToken)
}

func TestChangePassword_KeepsOnlyTheCurrentSessionAndDisconnects(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().CheckPassword(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: testPassword}).Return(nil)
	m.users.EXPECT().HashPassword(mock.Anything, testNewPassword).Return(testPasswordHash, nil)
	m.userRepo.EXPECT().ChangePassword(mock.Anything, spec.PasswordChange{UserID: userID, PasswordHash: testPasswordHash, KeepToken: testToken}).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(userID).Return(3)

	// when
	err := svc.ChangePassword(t.Context(), spec.PasswordUpdate{UserID: userID, CurrentPassword: testPassword, NewPassword: testNewPassword, KeepToken: testToken})

	// then
	require.NoError(t, err)
}

func TestChangePassword_RefusesAWrongCurrentPassword(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(user.ErrIncorrectPassword)

	// when
	err := svc.ChangePassword(t.Context(), spec.PasswordUpdate{UserID: uuid.New(), CurrentPassword: "wrong", NewPassword: testNewPassword, KeepToken: testToken})

	// then nothing is hashed, written or revoked
	require.ErrorIs(t, err, user.ErrIncorrectPassword)
}

func TestChangePassword_RefusesAShortNewPassword(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.users.EXPECT().HashPassword(mock.Anything, "short").Return("", user.ErrPasswordTooShort)

	// when
	err := svc.ChangePassword(t.Context(), spec.PasswordUpdate{UserID: uuid.New(), CurrentPassword: testPassword, NewPassword: "short", KeepToken: testToken})

	// then
	require.ErrorIs(t, err, user.ErrPasswordTooShort)
}

func TestChangePassword_AMissingUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.users.EXPECT().HashPassword(mock.Anything, testNewPassword).Return(testPasswordHash, nil)
	m.userRepo.EXPECT().ChangePassword(mock.Anything, mock.Anything).Return(fmt.Errorf("change password: %w", dao.ErrNotFound))

	// when
	err := svc.ChangePassword(t.Context(), spec.PasswordUpdate{UserID: uuid.New(), CurrentPassword: testPassword, NewPassword: testNewPassword, KeepToken: testToken})

	// then
	require.ErrorIs(t, err, user.ErrUserNotFound)
}

func TestChangePassword_ReportsAFailedChangeWithoutDisconnecting(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.users.EXPECT().HashPassword(mock.Anything, testNewPassword).Return(testPasswordHash, nil)
	m.userRepo.EXPECT().ChangePassword(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	err := svc.ChangePassword(t.Context(), spec.PasswordUpdate{UserID: uuid.New(), CurrentPassword: testPassword, NewPassword: testNewPassword, KeepToken: testToken})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestSetEmail_HoldsTheNewAddressAsPendingAndNotifiesThePreviousOne(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	userID := uuid.New()
	m.users.EXPECT().CheckPassword(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: testPassword}).Return(nil)
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail, EmailVerified: true}, nil)
	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testNewEmail, ExcludeUserID: userID}).Return(false, nil)

	var issued spec.NewEmailVerification
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewEmailVerification, _ ...*sql.Tx) error {
		issued = s

		return nil
	})
	m.site()
	delivers(m)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: userID, Email: testNewEmail, Password: testPassword})

	// then users.email is never written here, which the strict user repository mock would report
	require.NoError(t, err)
	assert.Equal(t, userID, issued.UserID)
	require.NotNil(t, issued.PendingEmail)
	assert.Equal(t, testNewEmail, *issued.PendingEmail)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), issued.ExpiresAt, time.Minute)

	require.Len(t, m.sent, 2)
	assert.Equal(t, testEmail, m.sent[0].To)
	assert.Contains(t, m.sent[0].HTML, testNewEmail)
	assert.Contains(t, m.sent[0].HTML, "https://retrotide.test/forgot-password")
	assert.Equal(t, testNewEmail, m.sent[1].To)
	assert.Contains(t, m.sent[1].Subject, "new email address")
	assert.Equal(t, issued.TokenHash, token.Hash(linkedToken(t, m.sent[1], "verify-email")))
}

func TestSetEmail_RefusesAnInvalidAddress(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: uuid.New(), Email: "nope", Password: testPassword})

	// then
	require.ErrorIs(t, err, ErrInvalidEmail)
}

func TestSetEmail_RefusesWhileNoProviderIsConfigured(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: uuid.New(), Email: testNewEmail, Password: testPassword})

	// then a change that could never be confirmed is not started
	require.ErrorIs(t, err, ErrEmailUnavailable)
}

func TestSetEmail_RefusesAWrongPassword(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(user.ErrIncorrectPassword)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: uuid.New(), Email: testNewEmail, Password: "wrong"})

	// then
	require.ErrorIs(t, err, user.ErrIncorrectPassword)
}

func TestSetEmail_RefusesTheCurrentAddressInAnyCase(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	userID := uuid.New()
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail}, nil)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: userID, Email: strings.ToUpper(testEmail), Password: testPassword})

	// then
	require.ErrorIs(t, err, ErrEmailUnchanged)
}

func TestSetEmail_RefusesAnAddressAnotherAccountHolds(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	userID := uuid.New()
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail}, nil)
	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testNewEmail, ExcludeUserID: userID}).Return(true, nil)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: userID, Email: testNewEmail, Password: testPassword})

	// then
	require.ErrorIs(t, err, ErrEmailTaken)
}

func TestSetEmail_SendsNoLinkWhenThePreviousAddressCannotBeNotified(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	userID := uuid.New()
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail}, nil)
	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testNewEmail, ExcludeUserID: userID}).Return(false, nil)
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).Return(nil)
	m.site()
	m.email.EXPECT().Send(mock.Anything, mock.MatchedBy(func(message email.Message) bool {
		return message.To == testEmail
	})).Return(errSMTPRefused)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: userID, Email: testNewEmail, Password: testPassword})

	// then the new address never receives a link that could complete an unannounced change
	require.ErrorIs(t, err, errSMTPRefused)
}

func TestSetEmail_ReportsAFailedVerificationSend(t *testing.T) {
	// given
	svc, m := newMailingService(t)
	userID := uuid.New()
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail}, nil)
	m.userRepo.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testNewEmail, ExcludeUserID: userID}).Return(false, nil)
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).Return(nil)
	m.site()
	m.email.EXPECT().Send(mock.Anything, mock.MatchedBy(func(message email.Message) bool {
		return message.To == testEmail
	})).Return(nil)
	m.email.EXPECT().Send(mock.Anything, mock.MatchedBy(func(message email.Message) bool {
		return message.To == testNewEmail
	})).Return(errSMTPRefused)

	// when
	err := svc.SetEmail(t.Context(), spec.EmailChange{UserID: userID, Email: testNewEmail, Password: testPassword})

	// then
	require.ErrorIs(t, err, errSMTPRefused)
}

func TestSignOutOtherDevices_KeepsTheCurrentSessionDisconnectsAndCancelsPendingEmailChanges(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	keep := spec.SessionDeletionExcept{UserID: userID, KeepToken: testToken}
	m.sessionRepo.EXPECT().DeleteAllForUserExcept(mock.Anything, keep).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(userID).Return(4)
	m.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID).Return(nil)

	// when
	err := svc.SignOutOtherDevices(t.Context(), keep)

	// then a change of address started on a device that is now signed out can no longer be confirmed
	require.NoError(t, err)
}

func TestSignOutOtherDevices_ReportsAFailedRevocationWithoutDisconnecting(t *testing.T) {
	// given
	svc, m := newTestService(t)
	keep := spec.SessionDeletionExcept{UserID: uuid.New(), KeepToken: testToken}
	m.sessionRepo.EXPECT().DeleteAllForUserExcept(mock.Anything, keep).Return(errDatabaseDown)

	// when
	err := svc.SignOutOtherDevices(t.Context(), keep)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestSignOutOtherDevices_ReportsAFailedPendingEmailCancellation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	keep := spec.SessionDeletionExcept{UserID: userID, KeepToken: testToken}
	m.sessionRepo.EXPECT().DeleteAllForUserExcept(mock.Anything, keep).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(userID).Return(1)
	m.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID).Return(errDatabaseDown)

	// when
	err := svc.SignOutOtherDevices(t.Context(), keep)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestDeleteAccount_DeletesTheUserAndDisconnects(t *testing.T) {
	// given
	svc, m := newTestService(t)
	confirmation := spec.UserPasswordConfirmation{UserID: uuid.New(), Password: testPassword}
	m.users.EXPECT().CheckPassword(mock.Anything, confirmation).Return(nil)
	m.organisations.EXPECT().DeleteAccount(mock.Anything, confirmation.UserID).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(confirmation.UserID).Return(1)

	// when
	err := svc.DeleteAccount(t.Context(), confirmation)

	// then
	require.NoError(t, err)
}

func TestDeleteAccount_RefusesAWrongPassword(t *testing.T) {
	// given
	svc, m := newTestService(t)
	confirmation := spec.UserPasswordConfirmation{UserID: uuid.New(), Password: "wrong"}
	m.users.EXPECT().CheckPassword(mock.Anything, confirmation).Return(user.ErrIncorrectPassword)

	// when
	err := svc.DeleteAccount(t.Context(), confirmation)

	// then nothing is deleted and nobody is disconnected
	require.ErrorIs(t, err, user.ErrIncorrectPassword)
}

func TestDeleteAccount_RefusesTheSoleOwnerOfASharedOrganisation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	confirmation := spec.UserPasswordConfirmation{UserID: uuid.New(), Password: testPassword}
	m.users.EXPECT().CheckPassword(mock.Anything, confirmation).Return(nil)
	m.organisations.EXPECT().DeleteAccount(mock.Anything, confirmation.UserID).Return(org.ErrSoleOwner)

	// when
	err := svc.DeleteAccount(t.Context(), confirmation)

	// then the account stays and its sessions stay connected
	require.ErrorIs(t, err, org.ErrSoleOwner)
}

func TestDeleteAccount_ReturnsAFailedDeletion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	confirmation := spec.UserPasswordConfirmation{UserID: uuid.New(), Password: testPassword}
	m.users.EXPECT().CheckPassword(mock.Anything, confirmation).Return(nil)
	m.organisations.EXPECT().DeleteAccount(mock.Anything, confirmation.UserID).Return(errDatabaseDown)

	// when
	err := svc.DeleteAccount(t.Context(), confirmation)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}
