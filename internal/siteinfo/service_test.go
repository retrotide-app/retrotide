package siteinfo

import (
	"testing"

	"retrotide/internal/auth"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/identity"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	testDeps struct {
		settings *settings.MockService
		auth     *auth.MockService
		identity *identity.MockService
		email    *email.MockService
	}
)

const (
	testSiteName         = "Harbour Retros"
	testTurnstileSiteKey = "0x4AAAAAAAtestsitekey"
)

var (
	testProviders = []dto.IdentityProviderResponse{{ID: "example", Name: "Example"}, {ID: "sample", Name: "Sample ID"}}
)

func newTestService(t *testing.T, mode config.DeploymentMode) (*service, testDeps) {
	t.Helper()

	deps := testDeps{settings: settings.NewMockService(t), auth: auth.NewMockService(t), identity: identity.NewMockService(t), email: email.NewMockService(t)}

	return NewService(Deps{Settings: deps.settings, Auth: deps.auth, Identity: deps.identity, Email: deps.email, DeploymentMode: mode}).(*service), deps
}

func expectSettings(deps testDeps, mode config.SiteMode, registrationOpen bool, providers []dto.IdentityProviderResponse) {
	deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(mode), nil)
	deps.auth.EXPECT().RegistrationOpen(mock.Anything).Return(registrationOpen, nil)
	deps.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(true, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(true, nil)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingTurnstileSiteKey).Return(testTurnstileSiteKey, nil)
	deps.identity.EXPECT().EnabledProviders(mock.Anything).Return(providers, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn).Return(true, nil)
}

func TestGet_ReportsTheSettingsTheShellNeeds(t *testing.T) {
	// given
	svc, deps := newTestService(t, config.DeploymentHosted)
	expectSettings(deps, config.SiteModePublic, true, testProviders)

	// when
	info, err := svc.Get(t.Context())

	// then the providers keep the order the identity service lists them in
	require.NoError(t, err)
	assert.Equal(t, dto.SiteInfoResponse{
		SiteName:          testSiteName,
		DeploymentMode:    "hosted",
		SiteMode:          "public",
		RegistrationOpen:  true,
		EmailEnabled:      true,
		MaintenanceMode:   true,
		TurnstileEnabled:  true,
		TurnstileSiteKey:  testTurnstileSiteKey,
		IdentityProviders: testProviders,
		PasswordSignIn:    true,
	}, info)
}

func TestGet_ReportsWhetherPasswordSignInIsAllowed(t *testing.T) {
	// given password sign-in is switched off and a provider is set up
	svc, deps := newTestService(t, config.DeploymentHosted)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	deps.auth.EXPECT().RegistrationOpen(mock.Anything).Return(true, nil)
	deps.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(false, nil)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingTurnstileSiteKey).Return("", nil)
	deps.identity.EXPECT().EnabledProviders(mock.Anything).Return(testProviders, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn).Return(false, nil)

	// when
	info, err := svc.Get(t.Context())

	// then the SPA hides the password forms
	require.NoError(t, err)
	assert.False(t, info.PasswordSignIn)
	assert.Equal(t, testProviders, info.IdentityProviders)
}

func TestGet_ReportsAPrivateSiteAndWhetherItsRegistrationIsOpen(t *testing.T) {
	tests := []struct {
		name             string
		registrationOpen bool
	}{
		{name: "an empty site still takes its first account", registrationOpen: true},
		{name: "a site with an account takes no registration", registrationOpen: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the auth service owns the first-account rule, and the identity service lists no provider on a private site
			svc, deps := newTestService(t, config.DeploymentSelfHosted)
			expectSettings(deps, config.SiteModePrivate, tc.registrationOpen, []dto.IdentityProviderResponse{})

			// when
			info, err := svc.Get(t.Context())

			// then
			require.NoError(t, err)
			assert.Equal(t, "private", info.SiteMode)
			assert.Equal(t, tc.registrationOpen, info.RegistrationOpen)
			assert.Empty(t, info.IdentityProviders)
		})
	}
}

func TestGet_ReportsWhetherTheSiteCanSendEmail(t *testing.T) {
	// given
	svc, deps := newTestService(t, config.DeploymentHosted)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	deps.auth.EXPECT().RegistrationOpen(mock.Anything).Return(false, nil)
	deps.email.EXPECT().Enabled(mock.Anything).Return(false, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(false, nil)
	deps.settings.EXPECT().Get(mock.Anything, config.SettingTurnstileSiteKey).Return("", nil)
	deps.identity.EXPECT().EnabledProviders(mock.Anything).Return([]dto.IdentityProviderResponse{}, nil)
	deps.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn).Return(true, nil)

	// when
	info, err := svc.Get(t.Context())

	// then the SPA can say why sign-up and invites are closed
	require.NoError(t, err)
	assert.False(t, info.EmailEnabled)
}

func TestGet_ListsNoProviderWhileTheIdentityServiceEnablesNone(t *testing.T) {
	// given every provider's settings are empty, which each provider decides for itself
	svc, deps := newTestService(t, config.DeploymentHosted)
	expectSettings(deps, config.SiteModePublic, true, []dto.IdentityProviderResponse{})

	// when
	info, err := svc.Get(t.Context())

	// then the SPA shows email and password only
	require.NoError(t, err)
	assert.NotNil(t, info.IdentityProviders)
	assert.Empty(t, info.IdentityProviders)
}

func TestGet_ReportsTheDeploymentModeItWasBuiltWith(t *testing.T) {
	// given the mode comes from DEPLOYMENT_MODE at boot, never from a site setting
	svc, deps := newTestService(t, config.DeploymentSelfHosted)
	expectSettings(deps, config.SiteModePublic, true, []dto.IdentityProviderResponse{})

	// when
	info, err := svc.Get(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, "self_hosted", info.DeploymentMode)
}

func TestGet_ReportsASettingThatCannotBeRead(t *testing.T) {
	readsSiteMode := func(deps testDeps) {
		deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
		deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	}

	readsRegistration := func(deps testDeps) {
		readsSiteMode(deps)
		deps.auth.EXPECT().RegistrationOpen(mock.Anything).Return(true, nil)
	}

	readsEmail := func(deps testDeps) {
		readsRegistration(deps)
		deps.email.EXPECT().Enabled(mock.Anything).Return(false, nil)
	}

	readsMaintenance := func(deps testDeps) {
		readsEmail(deps)
		deps.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, nil)
	}

	readsTurnstile := func(deps testDeps) {
		readsMaintenance(deps)
		deps.settings.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(true, nil)
	}

	readsTurnstileKey := func(deps testDeps) {
		readsTurnstile(deps)
		deps.settings.EXPECT().Get(mock.Anything, config.SettingTurnstileSiteKey).Return(testTurnstileSiteKey, nil)
	}

	readsProviders := func(deps testDeps) {
		readsTurnstileKey(deps)
		deps.identity.EXPECT().EnabledProviders(mock.Anything).Return(testProviders, nil)
	}

	tests := []struct {
		name   string
		expect func(deps testDeps)
		want   string
	}{
		{
			name: "the site name",
			expect: func(deps testDeps) {
				deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return("", errDatabaseDown)
			},
			want: "read the site name",
		},
		{
			name: "the site mode",
			expect: func(deps testDeps) {
				deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
				deps.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)
			},
			want: "read the site mode",
		},
		{
			name: "whether registration is open",
			expect: func(deps testDeps) {
				readsSiteMode(deps)
				deps.auth.EXPECT().RegistrationOpen(mock.Anything).Return(false, errDatabaseDown)
			},
			want: "check whether registration is open",
		},
		{
			name: "whether the site can send email",
			expect: func(deps testDeps) {
				readsRegistration(deps)
				deps.email.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)
			},
			want: "check whether the site can send email",
		},
		{
			name: "the maintenance switch",
			expect: func(deps testDeps) {
				readsEmail(deps)
				deps.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, errDatabaseDown)
			},
			want: "read the maintenance switch",
		},
		{
			name: "the turnstile switch",
			expect: func(deps testDeps) {
				readsMaintenance(deps)
				deps.settings.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(false, errDatabaseDown)
			},
			want: "read whether turnstile is enabled",
		},
		{
			name: "the turnstile site key",
			expect: func(deps testDeps) {
				readsTurnstile(deps)
				deps.settings.EXPECT().Get(mock.Anything, config.SettingTurnstileSiteKey).Return("", errDatabaseDown)
			},
			want: "read the turnstile site key",
		},
		{
			name: "a sign-in provider's switch",
			expect: func(deps testDeps) {
				readsTurnstileKey(deps)
				deps.identity.EXPECT().EnabledProviders(mock.Anything).Return(nil, errDatabaseDown)
			},
			want: "read the enabled sign-in providers",
		},
		{
			name: "the password sign-in switch",
			expect: func(deps testDeps) {
				readsProviders(deps)
				deps.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn).Return(false, errDatabaseDown)
			},
			want: "read whether password sign-in is allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, deps := newTestService(t, config.DeploymentHosted)
			tc.expect(deps)

			// when
			_, err := svc.Get(t.Context())

			// then a failed read is never answered with a default
			require.ErrorIs(t, err, errDatabaseDown)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
