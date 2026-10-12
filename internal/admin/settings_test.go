package admin

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/email"
	"retrotide/internal/identity"
	"retrotide/internal/identity/google"
	"retrotide/internal/identity/microsoft"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func storedSettings() map[config.SiteSettingKey]string {
	all := make(map[config.SiteSettingKey]string, len(config.AllSiteSettings))
	for _, def := range config.AllSiteSettings {
		all[def.Key] = def.Default
	}

	return all
}

func settingsUpdate(actorID uuid.UUID, values map[string]string) spec.StaffSettingsUpdate {
	return spec.StaffSettingsUpdate{StaffAction: staffAction(actorID), Values: values}
}

func settingsEntry(actorID uuid.UUID, keys string) audit.NewEntry {
	return audit.NewEntry{
		ActorID:    actorID,
		Action:     audit.ActionUpdateSettings,
		TargetType: audit.TargetSettings,
		Details:    map[string]string{"keys": keys},
	}
}

func TestGetSettings_MasksEverySecretThatIsSet(t *testing.T) {
	// given
	svc, m := newTestService(t)
	stored := storedSettings()
	stored[config.SettingSMTPPassword.Key] = "hunter2"
	stored[config.SettingMetricsToken.Key] = "scrape-me"
	stored[config.SettingSiteName.Key] = "Team Retros"
	m.settings.EXPECT().GetAll(mock.Anything).Return(stored, nil)

	// when
	got, err := svc.GetSettings(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, config.SecretMask, got.Settings["smtp_password"])
	assert.Equal(t, config.SecretMask, got.Settings["metrics_token"])
	assert.Equal(t, "Team Retros", got.Settings["site_name"])
	assert.Len(t, got.Settings, len(config.AllSiteSettings))
}

func TestGetSettings_LeavesAnUnsetSecretEmptySoItShowsAsNotSet(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)

	// when
	got, err := svc.GetSettings(t.Context())

	// then
	require.NoError(t, err)
	assert.Empty(t, got.Settings["turnstile_secret_key"])
	assert.Empty(t, got.Settings["cloudflare_api_token"])
}

func TestGetSettings_ReturnsAReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().GetAll(mock.Anything).Return(nil, errDatabaseDown)

	// when
	got, err := svc.GetSettings(t.Context())

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, got)
}

func TestUpdateSettings_WritesOnlyTheChangedValuesAndAuditsTheirKeys(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	stored := storedSettings()
	stored[config.SettingSMTPPassword.Key] = "hunter2"
	m.settings.EXPECT().GetAll(mock.Anything).Return(stored, nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, map[config.SiteSettingKey]string{
		config.SettingSiteName.Key: "Team Retros",
		config.SettingSiteMode.Key: "private",
	}, actorID).Return(nil)
	m.audited(settingsEntry(actorID, "site_mode,site_name"))

	// when the form sends every value back, the secret still masked
	err := svc.UpdateSettings(t.Context(), settingsUpdate(actorID, map[string]string{
		"site_name":     "Team Retros",
		"site_mode":     "private",
		"smtp_password": config.SecretMask,
		"base_url":      stored[config.SettingBaseURL.Key],
	}))

	// then the masked secret is left alone, and no value reaches the audit log
	assert.NoError(t, err)
}

func TestUpdateSettings_ReplacesASecretThatIsNotTheMask(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, map[config.SiteSettingKey]string{config.SettingSMTPPassword.Key: "new-secret"}, actorID).Return(nil)
	m.audited(settingsEntry(actorID, "smtp_password"))

	// when
	err := svc.UpdateSettings(t.Context(), settingsUpdate(actorID, map[string]string{"smtp_password": "new-secret"}))

	// then
	assert.NoError(t, err)
}

func TestUpdateSettings_NothingChangedWritesNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)

	// when
	err := svc.UpdateSettings(t.Context(), settingsUpdate(uuid.New(), map[string]string{"site_name": config.SettingSiteName.Default}))

	// then neither a write nor an audit entry is made
	assert.NoError(t, err)
}

func TestUpdateSettings_RefusesBeforeWriting(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   error
		text   string
	}{
		{name: "an unknown key", values: map[string]string{"private_mode": "true"}, want: ErrUnknownSetting, text: "unknown setting: private_mode"},
		{name: "a value of the wrong type", values: map[string]string{"max_body_size": "lots"}, want: ErrInvalidSetting, text: "invalid setting: max_body_size must be a whole number"},
		{name: "a value outside its set", values: map[string]string{"site_mode": "sometimes"}, want: ErrInvalidSetting},
		{name: "a provider without its credentials", values: map[string]string{"email_provider": "cloudflare"}, want: ErrInvalidSetting},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)

			// when
			err := svc.UpdateSettings(t.Context(), settingsUpdate(uuid.New(), tc.values))

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.text != "" {
				assert.EqualError(t, err, tc.text)
			}
		})
	}
}

func TestUpdateSettings_AStaleCacheIsReportedButTheChangeIsStillAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	stale := fmt.Errorf("%w: valkey unreachable", repository.ErrStaleSettingsCache)
	m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, mock.Anything, actorID).Return(stale)
	m.audited(settingsEntry(actorID, "site_name"))

	// when
	err := svc.UpdateSettings(t.Context(), settingsUpdate(actorID, map[string]string{"site_name": "Team Retros"}))

	// then the values are saved, so the change is recorded, and the admin still learns of the stale cache
	assert.ErrorIs(t, err, ErrSettingsCacheStale)
	assert.ErrorIs(t, err, repository.ErrStaleSettingsCache)
	assert.NotErrorIs(t, err, ErrNotAudited)
}

func TestUpdateSettings_AFailedWriteIsNotAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	err := svc.UpdateSettings(t.Context(), settingsUpdate(uuid.New(), map[string]string{"site_name": "Team Retros"}))

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrSettingsCacheStale)
}

func TestUpdateSettings_AValueItsRegisteredCheckRefusesIsInvalidAndNotAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	refused := fmt.Errorf("%w: valkey_url: cache: cannot reach valkey", settings.ErrInvalidValue)
	m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, mock.Anything, mock.Anything).Return(refused)

	// when
	err := svc.UpdateSettings(t.Context(), settingsUpdate(uuid.New(), map[string]string{"valkey_url": "redis://nothing-here:6379"}))

	// then the admin sees why, as a refused value rather than a failed write
	assert.ErrorIs(t, err, ErrInvalidSetting)
	assert.EqualError(t, err, "invalid setting: valkey_url: cache: cannot reach valkey")
}

func TestUpdateSettings_TheMergedValuesAreCheckedTogether(t *testing.T) {
	// given the cloudflare credentials are already stored
	svc, m := newTestService(t)
	actorID := uuid.New()
	stored := storedSettings()
	stored[config.SettingCloudflareAccountID.Key] = "account"
	stored[config.SettingCloudflareAPIToken.Key] = "token"
	stored[config.SettingCloudflareEmailFrom.Key] = "retros@example.com"
	m.settings.EXPECT().GetAll(mock.Anything).Return(stored, nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, map[config.SiteSettingKey]string{config.SettingEmailProvider.Key: "cloudflare"}, actorID).Return(nil)
	m.audited(settingsEntry(actorID, "email_provider"))

	// when only the provider is switched
	err := svc.UpdateSettings(t.Context(), settingsUpdate(actorID, map[string]string{"email_provider": "cloudflare"}))

	// then
	assert.NoError(t, err)
}

func TestSendTestEmail_GoesToTheStaffMembersOwnAddress(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, actorID).Return(account(actorID), nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return("Retros <&> Co", nil)

	var sent email.Message
	m.email.EXPECT().SendTest(mock.Anything, mock.Anything).Run(func(_ context.Context, message email.Message) {
		sent = message
	}).Return(nil)
	m.audited(audit.NewEntry{ActorID: actorID, Action: audit.ActionSendTestEmail, TargetType: audit.TargetSettings})

	// when
	err := svc.SendTestEmail(t.Context(), staffAction(actorID))

	// then
	require.NoError(t, err)
	assert.Equal(t, testEmail, sent.To)
	assert.Equal(t, "Test email from Retros <&> Co", sent.Subject)
	assert.Contains(t, sent.HTML, "Retros &lt;&amp;&gt; Co")
	assert.NotContains(t, sent.HTML, "<&>")
}

func TestSendTestEmail_ReportsAMissingProvider(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, actorID).Return(account(actorID), nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().SendTest(mock.Anything, mock.Anything).Return(fmt.Errorf("send the test email: %w", email.ErrNotConfigured))

	// when
	err := svc.SendTestEmail(t.Context(), staffAction(actorID))

	// then nothing is audited
	assert.ErrorIs(t, err, ErrEmailUnavailable)
}

func TestSendTestEmail_ReportsAFailingProviderAsUnavailableWithItsCause(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	cause := errors.New("invalid port")
	m.users.EXPECT().GetByID(mock.Anything, actorID).Return(account(actorID), nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().SendTest(mock.Anything, mock.Anything).Return(fmt.Errorf("send the test email: %w: %w", email.ErrFailing, cause))

	// when
	err := svc.SendTestEmail(t.Context(), staffAction(actorID))

	// then nothing is audited, and the cause stays on the error
	assert.ErrorIs(t, err, ErrEmailUnavailable)
	assert.ErrorIs(t, err, cause)
}

func TestSendTestEmail_ReturnsTheProvidersRefusal(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, actorID).Return(account(actorID), nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().SendTest(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	err := svc.SendTestEmail(t.Context(), staffAction(actorID))

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrEmailUnavailable)
}

func withSignInAdapters(svc *service, m *testMocks) {
	svc.signInProviders = []identity.Provider{google.New(m.settings), microsoft.New(m.settings)}
}

func withGoogleCredentials(stored map[config.SiteSettingKey]string) {
	stored[config.SettingGoogleClientID.Key] = "client.apps.googleusercontent.com"
	stored[config.SettingGoogleClientSecret.Key] = "google-secret"
}

func withMicrosoftCredentials(stored map[config.SiteSettingKey]string) {
	stored[config.SettingMicrosoftClientID.Key] = "6731de76-14a6-49ae-97bc-6eba6914391e"
	stored[config.SettingMicrosoftClientSecret.Key] = "microsoft-secret"
}

func TestUpdateSettings_PasswordSignInGoesOffOnlyWithAProviderOnAPublicSite(t *testing.T) {
	tests := []struct {
		name   string
		stored func(stored map[config.SiteSettingKey]string)
	}{
		{name: "google configured", stored: withGoogleCredentials},
		{name: "microsoft configured", stored: withMicrosoftCredentials},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a public site with one provider set up
			svc, m := newTestService(t)
			withSignInAdapters(svc, m)
			actorID := uuid.New()
			stored := storedSettings()
			tc.stored(stored)
			m.settings.EXPECT().GetAll(mock.Anything).Return(stored, nil)
			m.settings.EXPECT().SetMultiple(mock.Anything, map[config.SiteSettingKey]string{config.SettingAllowPasswordSignIn.Key: "false"}, actorID).Return(nil)
			m.audited(settingsEntry(actorID, "allow_password_sign_in"))

			// when
			err := svc.UpdateSettings(t.Context(), settingsUpdate(actorID, map[string]string{"allow_password_sign_in": "false"}))

			// then
			assert.NoError(t, err)
		})
	}
}

func TestUpdateSettings_PasswordSignInGoesOffTogetherWithTheProviderCredentials(t *testing.T) {
	// given a public site with no provider yet
	svc, m := newTestService(t)
	withSignInAdapters(svc, m)
	actorID := uuid.New()
	m.settings.EXPECT().GetAll(mock.Anything).Return(storedSettings(), nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, map[config.SiteSettingKey]string{
		config.SettingAllowPasswordSignIn.Key:   "false",
		config.SettingMicrosoftClientID.Key:     "6731de76-14a6-49ae-97bc-6eba6914391e",
		config.SettingMicrosoftClientSecret.Key: "microsoft-secret",
	}, actorID).Return(nil)
	m.audited(settingsEntry(actorID, "allow_password_sign_in,microsoft_client_id,microsoft_client_secret"))

	// when the credentials arrive in the same save
	err := svc.UpdateSettings(t.Context(), settingsUpdate(actorID, map[string]string{
		"allow_password_sign_in":  "false",
		"microsoft_client_id":     "6731de76-14a6-49ae-97bc-6eba6914391e",
		"microsoft_client_secret": "microsoft-secret",
	}))

	// then the merged values carry a provider
	assert.NoError(t, err)
}

func TestUpdateSettings_RefusesToLeaveNoWayToSignIn(t *testing.T) {
	tests := []struct {
		name   string
		stored func(stored map[config.SiteSettingKey]string)
		values map[string]string
	}{
		{
			name:   "password sign-in off with no provider",
			stored: func(map[config.SiteSettingKey]string) {},
			values: map[string]string{"allow_password_sign_in": "false"},
		},
		{
			name: "password sign-in off with only half a provider",
			stored: func(stored map[config.SiteSettingKey]string) {
				stored[config.SettingMicrosoftClientID.Key] = "client"
			},
			values: map[string]string{"allow_password_sign_in": "false"},
		},
		{
			name: "password sign-in off on a private site",
			stored: func(stored map[config.SiteSettingKey]string) {
				withGoogleCredentials(stored)
				stored[config.SettingSiteMode.Key] = string(config.SiteModePrivate)
			},
			values: map[string]string{"allow_password_sign_in": "false"},
		},
		{
			name: "the site going private while password sign-in is off",
			stored: func(stored map[config.SiteSettingKey]string) {
				withGoogleCredentials(stored)
				stored[config.SettingAllowPasswordSignIn.Key] = "false"
			},
			values: map[string]string{"site_mode": "private"},
		},
		{
			name: "the only provider's credentials blanked while password sign-in is off",
			stored: func(stored map[config.SiteSettingKey]string) {
				withMicrosoftCredentials(stored)
				stored[config.SettingAllowPasswordSignIn.Key] = "false"
			},
			values: map[string]string{"microsoft_client_id": "", "microsoft_client_secret": ""},
		},
		{
			name: "the only provider's secret blanked while password sign-in is off",
			stored: func(stored map[config.SiteSettingKey]string) {
				withGoogleCredentials(stored)
				stored[config.SettingAllowPasswordSignIn.Key] = "false"
			},
			values: map[string]string{"google_client_secret": ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			withSignInAdapters(svc, m)
			stored := storedSettings()
			tc.stored(stored)
			m.settings.EXPECT().GetAll(mock.Anything).Return(stored, nil)

			// when
			err := svc.UpdateSettings(t.Context(), settingsUpdate(uuid.New(), tc.values))

			// then nothing is written or audited
			assert.ErrorIs(t, err, ErrNoSignInMethod)
			assert.EqualError(t, err, "password sign-in can be turned off only on a public site with a sign-in provider configured")
		})
	}
}

func TestUpdateSettings_BlankingAProviderIsFineWhilePasswordSignInIsOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	withSignInAdapters(svc, m)
	actorID := uuid.New()
	stored := storedSettings()
	withGoogleCredentials(stored)
	m.settings.EXPECT().GetAll(mock.Anything).Return(stored, nil)
	m.settings.EXPECT().SetMultiple(mock.Anything, map[config.SiteSettingKey]string{config.SettingGoogleClientSecret.Key: ""}, actorID).Return(nil)
	m.audited(settingsEntry(actorID, "google_client_secret"))

	// when
	err := svc.UpdateSettings(t.Context(), settingsUpdate(actorID, map[string]string{"google_client_secret": ""}))

	// then
	assert.NoError(t, err)
}

func TestUpdateSettings_AProviderThatCannotCheckItsSettingsNeverCounts(t *testing.T) {
	// given a provider that does not say whether the values configure it
	svc, m := newTestService(t)
	stored := storedSettings()
	withGoogleCredentials(stored)
	m.settings.EXPECT().GetAll(mock.Anything).Return(stored, nil)

	// when
	err := svc.UpdateSettings(t.Context(), settingsUpdate(uuid.New(), map[string]string{"allow_password_sign_in": "false"}))

	// then
	assert.ErrorIs(t, err, ErrNoSignInMethod)
}
