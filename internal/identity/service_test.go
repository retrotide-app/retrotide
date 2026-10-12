package identity

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/auth"
	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
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
		provider     *MockProvider
		users        *user.MockService
		authz        *authz.MockService
		settings     *settings.MockService
		email        *email.MockService
		userRepo     *repository.MockUserRepository
		identities   *repository.MockUserIdentityRepository
		flows        *repository.MockOIDCFlowRepository
		auditLog     *repository.MockAuditLogRepository
		sessionRepo  *repository.MockSessionRepository
		disconnector *session.MockDisconnector
		twoFactor    *twofactor.MockGate
		signIns      *auth.MockSessionStarter

		sent    []email.Message
		audited []audit.NewEntry
		events  []string
	}
)

const (
	testProvider model.IdentityProvider = "example"

	testProviderName    = "Example ID"
	testBaseURL         = "https://retrotide.test/"
	testSiteName        = "RetroTide"
	testFlowToken       = "9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c5b4a39281706f5e4d3c2b1a0"
	testSessionToken    = "1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b"
	testOtherSession    = "7777777777777777777777777777777777777777777777777777777777777777"
	testIssuedSession   = "3c5e7a9b1d3f5a7c9e1b3d5f7a9c1e3b5d7f9a1c3e5b7d9f1a3c5e7b9d1f3a5c"
	testState           = "5a7e0d2c4b6f8a1c3e5d7f9b1a3c5e7d9f1b3d5f7a9c1e3b5d7f9a1c3e5b7d9f"
	testNonce           = "8c1e3a5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5f7a9c1e3b5d7f9a1c"
	testVerifier        = "2d4f6a8c1e3b5d7f9a1c3e5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5f"
	testCode            = "a-code-from-the-provider"
	testSubject         = "110169484474386276334"
	testOtherSubject    = "108765432109876543210"
	testAddress         = "alice@mail.example"
	testWorkAddress     = "alice@acme.example"
	testOtherAddress    = "alice@example.com"
	testName            = "Alice Example"
	testPassword        = "correct horse battery"
	testPasswordHash    = "$2a$10$pretendhashforthetestsonlypretendhashforthetestsonly"
	testAuthURL         = "https://id.example.test/authorize?client_id=retrotide"
	testOtherAuthURL    = "https://other.example.test/authorize?client_id=retrotide"
	testIP              = "198.51.100.7"
	testSessionDays     = 30
	testCreatedOn       = "3 March 2026"
	eventDisconnected   = "disconnected"
	eventSessionDeleted = "session deleted"
	eventSessionCreated = "session created"
)

var (
	testCreatedAt = time.Date(2026, time.March, 3, 9, 30, 0, 0, time.UTC)
)

func newProvider(t *testing.T, id model.IdentityProvider, name string) *MockProvider {
	t.Helper()

	provider := NewMockProvider(t)
	provider.EXPECT().ID().Return(id).Maybe()
	provider.EXPECT().Name().Return(name).Maybe()

	return provider
}

func newTestService(t *testing.T, extra ...Provider) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		provider:     newProvider(t, testProvider, testProviderName),
		users:        user.NewMockService(t),
		authz:        authz.NewMockService(t),
		settings:     settings.NewMockService(t),
		email:        email.NewMockService(t),
		userRepo:     repository.NewMockUserRepository(t),
		identities:   repository.NewMockUserIdentityRepository(t),
		flows:        repository.NewMockOIDCFlowRepository(t),
		auditLog:     repository.NewMockAuditLogRepository(t),
		sessionRepo:  repository.NewMockSessionRepository(t),
		disconnector: session.NewMockDisconnector(t),
		twoFactor:    twofactor.NewMockGate(t),
		signIns:      auth.NewMockSessionStarter(t),
	}

	sessions := session.NewManager(m.sessionRepo, m.settings)
	sessions.SetDisconnector(m.disconnector)

	svc, err := NewService(Deps{
		Providers:  append([]Provider{m.provider}, extra...),
		Users:      m.users,
		Authz:      m.authz,
		Sessions:   sessions,
		Settings:   m.settings,
		Email:      m.email,
		UserRepo:   m.userRepo,
		Identities: m.identities,
		Flows:      m.flows,
		AuditLog:   m.auditLog,
		TwoFactor:  m.twoFactor,
		SignIns:    m.signIns,
	})
	require.NoError(t, err)

	return svc.(*service), m
}

func newSiteService(t *testing.T, extra ...Provider) (*service, *testMocks) {
	t.Helper()

	svc, m := newTestService(t, extra...)
	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)

	return svc, m
}

func mails(m *testMocks) {
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, message email.Message) error {
		m.sent = append(m.sent, message)

		return nil
	})
}

func audits(m *testMocks) {
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, entry audit.NewEntry, _ ...*sql.Tx) error {
		m.audited = append(m.audited, entry)

		return nil
	})
}

func issuesSession(m *testMocks) *spec.NewSession {
	issued := &spec.NewSession{}

	m.signIns.EXPECT().StartSession(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, pending spec.TwoFactorPending) (*auth.SessionStart, error) {
		*issued = spec.NewSession{UserID: pending.UserID, Token: testIssuedSession}
		m.events = append(m.events, eventSessionCreated)

		return &auth.SessionStart{SessionToken: testIssuedSession}, nil
	})

	return issued
}

func deletesPresentedSession(m *testMocks, token string) {
	m.sessionRepo.EXPECT().Delete(mock.Anything, token).RunAndReturn(func(_ context.Context, _ string, _ ...*sql.Tx) error {
		m.events = append(m.events, eventSessionDeleted)

		return nil
	})
}

func disconnects(m *testMocks, userID uuid.UUID) {
	m.disconnector.EXPECT().DisconnectUser(userID).RunAndReturn(func(uuid.UUID) int {
		m.events = append(m.events, eventDisconnected)

		return 1
	})
}

func newAccount(address string, verified bool) *model.User {
	return &model.User{ID: uuid.New(), Email: address, PasswordHash: new(testPasswordHash), DisplayName: testName, EmailVerified: verified, CreatedAt: testCreatedAt}
}

func storedIdentity(userID uuid.UUID, address string) *model.UserIdentity {
	return &model.UserIdentity{Provider: testProvider, Subject: testSubject, UserID: userID, Email: address, CreatedAt: testCreatedAt}
}

func captureFlow(m *testMocks) *spec.NewOIDCFlow {
	stored := &spec.NewOIDCFlow{}

	m.flows.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewOIDCFlow, _ ...*sql.Tx) error {
		*stored = s

		return nil
	})

	return stored
}

func begins(m *testMocks) *Authorisation {
	drawn := &Authorisation{}

	m.provider.EXPECT().Begin(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, authorisation Authorisation) (string, error) {
		*drawn = authorisation

		return testAuthURL, nil
	})

	return drawn
}

func linker(m *testMocks, verified bool) uuid.UUID {
	userID := uuid.New()

	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testAddress, EmailVerified: verified, HasPassword: true}, nil)
	m.authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)

	return userID
}

func passwordlessLinker(m *testMocks) uuid.UUID {
	userID := uuid.New()

	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testAddress, EmailVerified: true, HasPassword: false}, nil)
	m.authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)

	return userID
}

func signInStart() spec.IdentityFlowStart {
	return spec.IdentityFlowStart{Provider: testProvider, Purpose: model.OIDCFlowSignIn}
}

func linkStart(userID uuid.UUID) spec.IdentityFlowStart {
	return spec.IdentityFlowStart{Provider: testProvider, Purpose: model.OIDCFlowLink, UserID: userID, SessionToken: testSessionToken, CurrentPassword: testPassword}
}

func reauthenticatingStart(userID uuid.UUID) spec.IdentityFlowStart {
	return spec.IdentityFlowStart{Provider: testProvider, Purpose: model.OIDCFlowLink, UserID: userID, SessionToken: testSessionToken, ReturnTo: "/account", ReauthenticateWith: otherProvider}
}

func TestNewService_RefusesTwoProvidersWithOneID(t *testing.T) {
	// given two adapters that both answer to the same route segment
	first := newProvider(t, testProvider, testProviderName)
	second := newProvider(t, testProvider, "Another Example")

	// when
	svc, err := NewService(Deps{Providers: []Provider{first, second}})

	// then the composition fails at boot rather than one provider silently shadowing the other
	require.ErrorIs(t, err, ErrDuplicateProvider)
	assert.ErrorContains(t, err, string(testProvider))
	assert.Nil(t, svc)
}

func TestEnabled_AsksTheProviderForItsOwnSwitch(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "switched on", want: true},
		{name: "switched off", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the provider owns and reads its own settings
			svc, m := newTestService(t)
			m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
			m.provider.EXPECT().Enabled(mock.Anything).Return(tc.want, nil)

			// when
			enabled, err := svc.Enabled(t.Context(), testProvider)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, enabled)
		})
	}
}

func TestEnabled_APrivateSiteSwitchesEveryProviderOffWithoutAskingIt(t *testing.T) {
	// given a private site, which signs in by password only
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)

	// when
	enabled, err := svc.Enabled(t.Context(), testProvider)

	// then the start and the callback answer 404, and the strict provider mock sees no Enabled call
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestEnabled_ReportsASiteModeThatCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)

	// when
	enabled, err := svc.Enabled(t.Context(), testProvider)

	// then a failed read neither opens a private site to a provider nor hides the cause
	require.ErrorIs(t, err, errDatabaseDown)
	assert.False(t, enabled)
}

func TestEnabled_AnUnknownProviderIsOffWithoutAskingAnyProvider(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	enabled, err := svc.Enabled(t.Context(), otherProvider)

	// then the strict provider mock sees no Enabled call
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestEnabled_ReportsASwitchThatCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	m.provider.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)

	// when
	enabled, err := svc.Enabled(t.Context(), testProvider)

	// then a failed read is never an off switch
	require.ErrorIs(t, err, errDatabaseDown)
	assert.False(t, enabled)
}

func TestEnabledProviders_ListsTheEnabledProvidersInTheirOrder(t *testing.T) {
	// given three providers, the middle one switched off
	first := newProvider(t, "first", "First ID")
	first.EXPECT().Enabled(mock.Anything).Return(true, nil)
	off := newProvider(t, "off", "Switched Off")
	off.EXPECT().Enabled(mock.Anything).Return(false, nil)
	last := newProvider(t, "last", "Last ID")
	last.EXPECT().Enabled(mock.Anything).Return(true, nil)

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)

	svc, err := NewService(Deps{Providers: []Provider{first, off, last}, Settings: settingsSvc})
	require.NoError(t, err)

	// when
	listed, err := svc.EnabledProviders(t.Context())

	// then each carries the id the routes use and the name the page shows
	require.NoError(t, err)
	assert.Equal(t, []dto.IdentityProviderResponse{{ID: "first", Name: "First ID"}, {ID: "last", Name: "Last ID"}}, listed)
}

func TestEnabledProviders_ListsNoneAsAnEmptyList(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	m.provider.EXPECT().Enabled(mock.Anything).Return(false, nil)

	// when
	listed, err := svc.EnabledProviders(t.Context())

	// then the list is empty rather than null on the wire
	require.NoError(t, err)
	assert.NotNil(t, listed)
	assert.Empty(t, listed)
}

func TestEnabledProviders_ListsNoneOnAPrivateSiteWithoutAskingAnyProvider(t *testing.T) {
	// given a private site, whatever each provider's own switch says
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)

	// when
	listed, err := svc.EnabledProviders(t.Context())

	// then the site info lists no provider, and the strict provider mock sees no Enabled call
	require.NoError(t, err)
	assert.NotNil(t, listed)
	assert.Empty(t, listed)
}

func TestEnabledProviders_ReportsASiteModeThatCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)

	// when
	listed, err := svc.EnabledProviders(t.Context())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listed)
}

func TestEnabledProviders_ReportsASwitchThatCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	m.provider.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)

	// when
	listed, err := svc.EnabledProviders(t.Context())

	// then a failed read never drops the provider from the list
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listed)
}

func TestStart_SignInStoresAFreshSingleUseFlow(t *testing.T) {
	// given
	svc, m := newTestService(t)
	drawn := begins(m)
	stored := captureFlow(m)

	start := signInStart()
	start.ReturnTo = "/invite?from=example"

	// when
	started, err := svc.Start(t.Context(), start)

	// then the provider is handed the state, the nonce and the verifier the flow stores
	require.NoError(t, err)
	assert.Equal(t, testAuthURL, started.AuthURL)
	assert.Regexp(t, `^[0-9a-f]{64}$`, started.FlowToken)

	assert.Equal(t, token.Hash(started.FlowToken), stored.TokenHash)
	assert.NotEqual(t, started.FlowToken, stored.TokenHash)
	assert.Equal(t, testProvider, stored.Provider)
	assert.Equal(t, model.OIDCFlowSignIn, stored.Purpose)
	assert.Equal(t, drawn.State, stored.State)
	assert.Equal(t, drawn.Nonce, stored.Nonce)
	assert.Equal(t, drawn.Verifier, stored.CodeVerifier)
	assert.Regexp(t, `^[0-9a-f]{64}$`, stored.State)
	assert.Regexp(t, `^[0-9a-f]{64}$`, stored.Nonce)
	assert.Regexp(t, `^[0-9a-f]{64}$`, stored.CodeVerifier)
	assert.NotEqual(t, stored.State, stored.Nonce)
	assert.NotEqual(t, stored.State, stored.CodeVerifier)
	assert.NotEqual(t, stored.Nonce, stored.CodeVerifier)
	assert.Equal(t, uuid.Nil, stored.UserID)
	assert.Empty(t, stored.SessionTokenHash)
	assert.Equal(t, "/invite?from=example", stored.ReturnTo)
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), stored.ExpiresAt, time.Minute)
}

func TestStart_DrawsNewSecretsForEveryFlow(t *testing.T) {
	// given
	svc, m := newTestService(t)
	begins(m)

	var stored []spec.NewOIDCFlow
	m.flows.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewOIDCFlow, _ ...*sql.Tx) error {
		stored = append(stored, s)

		return nil
	})

	// when
	first, firstErr := svc.Start(t.Context(), signInStart())
	second, secondErr := svc.Start(t.Context(), signInStart())

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Len(t, stored, 2)
	assert.NotEqual(t, first.FlowToken, second.FlowToken)
	assert.NotEqual(t, stored[0].State, stored[1].State)
	assert.NotEqual(t, stored[0].Nonce, stored[1].Nonce)
	assert.NotEqual(t, stored[0].CodeVerifier, stored[1].CodeVerifier)
}

func TestStart_DropsAReturnToThatFailsTheCheck(t *testing.T) {
	// given
	svc, m := newTestService(t)
	begins(m)
	stored := captureFlow(m)

	start := signInStart()
	start.ReturnTo = "https://evil.example/"

	// when
	_, err := svc.Start(t.Context(), start)

	// then the flow still starts, and lands on the default page
	require.NoError(t, err)
	assert.Empty(t, stored.ReturnTo)
}

func TestStart_LinkBindsTheFlowToTheUserAndTheSession(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := linker(m, true)
	m.users.EXPECT().CheckPassword(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: testPassword}).Return(nil)
	begins(m)
	stored := captureFlow(m)

	// when
	started, err := svc.Start(t.Context(), linkStart(userID))

	// then
	require.NoError(t, err)
	assert.Equal(t, testAuthURL, started.AuthURL)
	assert.Equal(t, model.OIDCFlowLink, stored.Purpose)
	assert.Equal(t, userID, stored.UserID)
	assert.Equal(t, token.Hash(testSessionToken), stored.SessionTokenHash)
}

func TestStart_LinkWithAWrongPasswordWritesNoFlow(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := linker(m, true)
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(user.ErrIncorrectPassword)

	start := linkStart(userID)
	start.CurrentPassword = "wrong"

	// when
	started, err := svc.Start(t.Context(), start)

	// then neither the provider nor the flow store is reached, which the strict mocks would report
	require.ErrorIs(t, err, user.ErrIncorrectPassword)
	assert.Nil(t, started)
}

func TestStart_ALinkWithAPasswordIgnoresAReauthenticationProvider(t *testing.T) {
	// given
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newTestService(t, other)
	userID := linker(m, true)
	m.users.EXPECT().CheckPassword(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: testPassword}).Return(nil)
	begins(m)
	stored := captureFlow(m)

	start := linkStart(userID)
	start.ReauthenticateWith = otherProvider

	// when
	_, err := svc.Start(t.Context(), start)

	// then the password proved the account, so the other provider is never asked, which its strict mock would report
	require.NoError(t, err)
	assert.Equal(t, testProvider, stored.Provider)
	assert.Equal(t, model.OIDCFlowLink, stored.Purpose)
	assert.Empty(t, stored.ThenLink)
}

func TestStart_APasswordlessLinkStartsAtAHeldProviderAndThenLinks(t *testing.T) {
	// given an account that signs in only with the other provider
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newTestService(t, other)
	userID := passwordlessLinker(m)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	other.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.identities.EXPECT().ListForUser(mock.Anything, userID).Return([]model.UserIdentity{{Provider: otherProvider, Subject: testOtherSubject, UserID: userID, Email: testAddress}}, nil)

	drawn := &Authorisation{}
	other.EXPECT().Begin(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, authorisation Authorisation) (string, error) {
		*drawn = authorisation

		return testOtherAuthURL, nil
	})
	stored := captureFlow(m)

	// when
	started, err := svc.Start(t.Context(), reauthenticatingStart(userID))

	// then the flow begins at the held provider and remembers which provider to link next, and no password is checked
	require.NoError(t, err)
	assert.Equal(t, testOtherAuthURL, started.AuthURL)
	assert.Equal(t, token.Hash(started.FlowToken), stored.TokenHash)
	assert.Equal(t, otherProvider, stored.Provider)
	assert.Equal(t, model.OIDCFlowReauthenticate, stored.Purpose)
	assert.Equal(t, testProvider, stored.ThenLink)
	assert.Equal(t, userID, stored.UserID)
	assert.Equal(t, token.Hash(testSessionToken), stored.SessionTokenHash)
	assert.Equal(t, "/account", stored.ReturnTo)
	assert.Equal(t, drawn.State, stored.State)
	assert.Equal(t, drawn.Nonce, stored.Nonce)
	assert.Equal(t, drawn.Verifier, stored.CodeVerifier)
}

func TestStart_APasswordlessLinkNeedsAnotherProviderTheAccountHolds(t *testing.T) {
	tests := []struct {
		name  string
		with  model.IdentityProvider
		setup func(m *testMocks, other *MockProvider, userID uuid.UUID)
	}{
		{name: "no provider named", with: ""},
		{name: "the provider being linked", with: testProvider},
		{name: "a provider no adapter claims", with: "nowhere"},
		{
			name: "a provider switched off",
			with: otherProvider,
			setup: func(m *testMocks, other *MockProvider, _ uuid.UUID) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
				other.EXPECT().Enabled(mock.Anything).Return(false, nil)
			},
		},
		{
			name: "a provider the account does not hold",
			with: otherProvider,
			setup: func(m *testMocks, other *MockProvider, userID uuid.UUID) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
				other.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.identities.EXPECT().ListForUser(mock.Anything, userID).Return([]model.UserIdentity{*storedIdentity(userID, testAddress)}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			other := newProvider(t, otherProvider, "Other ID")
			svc, m := newTestService(t, other)
			userID := passwordlessLinker(m)

			if tc.setup != nil {
				tc.setup(m, other, userID)
			}

			start := reauthenticatingStart(userID)
			start.ReauthenticateWith = tc.with

			// when
			started, err := svc.Start(t.Context(), start)

			// then no provider begins and no flow is written, which the strict mocks would report
			require.ErrorIs(t, err, ErrReauthenticationRequired)
			assert.Nil(t, started)
		})
	}
}

func TestStart_APasswordlessLinkReportsIdentitiesThatCannotBeRead(t *testing.T) {
	// given
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newTestService(t, other)
	userID := passwordlessLinker(m)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	other.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.identities.EXPECT().ListForUser(mock.Anything, userID).Return(nil, errDatabaseDown)

	// when
	started, err := svc.Start(t.Context(), reauthenticatingStart(userID))

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrReauthenticationRequired)
	assert.Nil(t, started)
}

func TestStart_APasswordlessLinkReportsASwitchThatCannotBeRead(t *testing.T) {
	// given
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newTestService(t, other)
	userID := passwordlessLinker(m)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	other.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)

	// when
	started, err := svc.Start(t.Context(), reauthenticatingStart(userID))

	// then a failed read is never an off switch
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrReauthenticationRequired)
	assert.Nil(t, started)
}

func TestStart_LinkNeedsAVerifiedEmail(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := linker(m, false)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, userID).Return(false, nil)

	// when
	_, err := svc.Start(t.Context(), linkStart(userID))

	// then the password is never checked and no flow is written
	require.ErrorIs(t, err, ErrAccountUnverified)
}

func TestStart_AnUnverifiedSuperAdminCanLink(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := linker(m, false)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, userID).Return(true, nil)
	m.users.EXPECT().CheckPassword(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: testPassword}).Return(nil)
	begins(m)
	stored := captureFlow(m)

	// when
	started, err := svc.Start(t.Context(), linkStart(userID))

	// then
	require.NoError(t, err)
	assert.Equal(t, testAuthURL, started.AuthURL)
	assert.Equal(t, userID, stored.UserID)
}

func TestStart_AVerificationExemptionThatCannotBeCheckedFailsClosed(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := linker(m, false)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, userID).Return(false, errDatabaseDown)

	// when
	started, err := svc.Start(t.Context(), linkStart(userID))

	// then the password is never checked and no flow is written
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrAccountUnverified)
	assert.Nil(t, started)
}

func TestStart_ALockedAccountCannotLink(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testAddress, EmailVerified: true, HasPassword: true}, nil)
	m.authz.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)

	// when
	started, err := svc.Start(t.Context(), linkStart(userID))

	// then a lock refuses writes, so the password is never checked and neither the provider nor the flow store is reached
	require.ErrorIs(t, err, ErrAccountLocked)
	assert.Nil(t, started)
}

func TestStart_ALockThatCannotBeCheckedFailsClosed(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testAddress, EmailVerified: true, HasPassword: true}, nil)
	m.authz.EXPECT().IsLocked(mock.Anything, userID).Return(true, errDatabaseDown)

	// when
	started, err := svc.Start(t.Context(), linkStart(userID))

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrAccountLocked)
	assert.Nil(t, started)
}

func TestStart_LinkNeedsASignedInSession(t *testing.T) {
	tests := []struct {
		name  string
		start spec.IdentityFlowStart
	}{
		{name: "no user", start: spec.IdentityFlowStart{Provider: testProvider, Purpose: model.OIDCFlowLink, SessionToken: testSessionToken, CurrentPassword: testPassword}},
		{name: "no session", start: spec.IdentityFlowStart{Provider: testProvider, Purpose: model.OIDCFlowLink, UserID: uuid.New(), CurrentPassword: testPassword}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			_, err := svc.Start(t.Context(), tc.start)

			// then
			require.ErrorIs(t, err, ErrSignInRequired)
		})
	}
}

func TestStart_LinkReportsAnAccountThatCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(nil, errDatabaseDown)

	// when
	_, err := svc.Start(t.Context(), linkStart(userID))

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestStart_RefusesAnUnknownPurpose(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	start := signInStart()
	start.Purpose = "sign_up"

	// when
	_, err := svc.Start(t.Context(), start)

	// then
	require.ErrorIs(t, err, ErrUnknownPurpose)
}

func TestStart_AnUnwiredProviderIsNotConfigured(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	start := signInStart()
	start.Provider = otherProvider

	// when
	_, err := svc.Start(t.Context(), start)

	// then
	require.ErrorIs(t, err, ErrProviderNotConfigured)
}

func TestStart_PassesOnWhyTheProviderCouldNotBegin(t *testing.T) {
	tests := []struct {
		name  string
		cause error
	}{
		{name: "the settings are empty", cause: ErrProviderNotConfigured},
		{name: "discovery failed", cause: fmt.Errorf("%w: dial tcp: i/o timeout", ErrProviderUnavailable)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.provider.EXPECT().Begin(mock.Anything, mock.Anything).Return("", tc.cause)

			// when
			_, err := svc.Start(t.Context(), signInStart())

			// then no flow is written for a sign-in that cannot reach the provider
			require.ErrorIs(t, err, tc.cause)
		})
	}
}

func TestStart_ReportsAFlowThatCannotBeStored(t *testing.T) {
	// given
	svc, m := newTestService(t)
	begins(m)
	m.flows.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	started, err := svc.Start(t.Context(), signInStart())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, started)
}

func TestListIdentities_ListsTheProviderTheAddressAtLinkTimeAndTheDate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.identities.EXPECT().ListForUser(mock.Anything, userID).Return([]model.UserIdentity{*storedIdentity(userID, testAddress)}, nil)

	// when
	listed, err := svc.ListIdentities(t.Context(), userID)

	// then
	require.NoError(t, err)
	assert.Equal(t, []dto.IdentityResponse{{Provider: "example", ProviderName: testProviderName, Email: testAddress, CreatedAt: "2026-03-03T09:30:00Z"}}, listed.Identities)
}

func TestListIdentities_NamesAProviderNoAdapterClaimsByItsID(t *testing.T) {
	// given an identity linked through a provider that is no longer composed
	svc, m := newTestService(t)
	userID := uuid.New()
	orphaned := storedIdentity(userID, testAddress)
	orphaned.Provider = otherProvider
	m.identities.EXPECT().ListForUser(mock.Anything, userID).Return([]model.UserIdentity{*orphaned}, nil)

	// when
	listed, err := svc.ListIdentities(t.Context(), userID)

	// then it is still listed, so it can still be removed
	require.NoError(t, err)
	assert.Equal(t, []dto.IdentityResponse{{Provider: "other", ProviderName: "other", Email: testAddress, CreatedAt: "2026-03-03T09:30:00Z"}}, listed.Identities)
}

func TestListIdentities_AnAccountWithoutIdentitiesListsNone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.identities.EXPECT().ListForUser(mock.Anything, userID).Return(nil, nil)

	// when
	listed, err := svc.ListIdentities(t.Context(), userID)

	// then the list is empty rather than null on the wire
	require.NoError(t, err)
	assert.NotNil(t, listed.Identities)
	assert.Empty(t, listed.Identities)
}

func TestListIdentities_ReportsAFailedRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.identities.EXPECT().ListForUser(mock.Anything, userID).Return(nil, errDatabaseDown)

	// when
	_, err := svc.ListIdentities(t.Context(), userID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
}

func TestUnlink_BesideAPasswordSucceedsAuditsAndNotifies(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testOtherAddress, true)
	key := spec.UserIdentityKey{UserID: account.ID, Provider: testProvider}
	m.identities.EXPECT().Unlink(mock.Anything, key).Return(storedIdentity(account.ID, testAddress), nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	audits(m)
	mails(m)

	// when
	err := svc.Unlink(t.Context(), spec.IdentityUnlinking{UserID: account.ID, Provider: testProvider, IP: testIP})

	// then
	require.NoError(t, err)

	require.Len(t, m.audited, 1)
	assert.Equal(t, audit.NewEntry{
		ActorID:    account.ID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionIdentityUnlinked,
		TargetType: audit.TargetIdentity,
		TargetID:   "example",
		SubjectID:  account.ID,
		Details:    map[string]string{"provider": "example", "provider_name": testProviderName},
		IP:         testIP,
	}, m.audited[0])

	require.Len(t, m.sent, 1)
	assert.Equal(t, testOtherAddress, m.sent[0].To)
	assert.Equal(t, "Example ID sign-in was removed from your RetroTide account", m.sent[0].Subject)
	assert.Contains(t, m.sent[0].HTML, testAddress)
	assert.Contains(t, m.sent[0].HTML, `href="https://retrotide.test/forgot-password"`)
}

func TestUnlink_RefusesTheLastWayToSignIn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.identities.EXPECT().Unlink(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("unlink: %w", dao.ErrLastSignInMethod))

	// when
	err := svc.Unlink(t.Context(), spec.IdentityUnlinking{UserID: userID, Provider: testProvider})

	// then nothing is audited or sent
	require.ErrorIs(t, err, ErrLastSignInMethod)
}

func TestUnlink_AnAccountWithoutThatIdentityIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.identities.EXPECT().Unlink(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("unlink: %w", dao.ErrNotFound))

	// when
	err := svc.Unlink(t.Context(), spec.IdentityUnlinking{UserID: uuid.New(), Provider: testProvider})

	// then a repeated unlink is a 404 rather than a 409
	require.ErrorIs(t, err, ErrIdentityNotFound)
	assert.NotErrorIs(t, err, ErrLastSignInMethod)
}

func TestUnlink_ReportsAFailedUnlink(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.identities.EXPECT().Unlink(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	err := svc.Unlink(t.Context(), spec.IdentityUnlinking{UserID: uuid.New(), Provider: testProvider})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrNotAudited)
	assert.NotErrorIs(t, err, ErrNoticeNotSent)
}

func TestUnlink_AFailedAuditStillSendsTheNotice(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testOtherAddress, true)
	m.identities.EXPECT().Unlink(mock.Anything, mock.Anything).Return(storedIdentity(account.ID, testAddress), nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabaseDown)
	mails(m)

	// when
	err := svc.Unlink(t.Context(), spec.IdentityUnlinking{UserID: account.ID, Provider: testProvider})

	// then the unlink committed, so the caller answers success and records the cause
	require.ErrorIs(t, err, ErrNotAudited)
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrNoticeNotSent)
	require.Len(t, m.sent, 1)
}

func TestUnlink_AFailedNoticeIsReported(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testOtherAddress, true)
	m.identities.EXPECT().Unlink(mock.Anything, mock.Anything).Return(storedIdentity(account.ID, testAddress), nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	audits(m)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errSMTPRefused)

	// when
	err := svc.Unlink(t.Context(), spec.IdentityUnlinking{UserID: account.ID, Provider: testProvider})

	// then
	require.ErrorIs(t, err, ErrNoticeNotSent)
	require.ErrorIs(t, err, errSMTPRefused)
	assert.NotErrorIs(t, err, ErrNotAudited)
	require.Len(t, m.audited, 1)
}

func TestDrawFlowSecrets_DrawsLongDistinctValues(t *testing.T) {
	// given
	seen := make(map[string]struct{})

	for range 20 {
		// when
		drawn, err := drawFlowSecrets()

		// then each is a fresh 32-byte token, which also satisfies the flow table's length checks and PKCE's verifier alphabet
		require.NoError(t, err)

		for _, value := range []string{drawn.cookieToken, drawn.authorisation.State, drawn.authorisation.Nonce, drawn.authorisation.Verifier} {
			assert.Regexp(t, `^[0-9a-f]{64}$`, value)

			_, repeated := seen[value]
			assert.False(t, repeated)
			seen[value] = struct{}{}
		}
	}
}
