package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultSettings() map[SiteSettingKey]string {
	all := make(map[SiteSettingKey]string, len(AllSiteSettings))
	for _, def := range AllSiteSettings {
		all[def.Key] = def.Default
	}

	return all
}

func prepareLoad(t *testing.T) {
	t.Helper()

	t.Chdir(t.TempDir())

	keys := []string{
		"LISTEN_ADDR",
		"DATABASE_URL",
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_DB",
		"POSTGRES_SSL_MODE",
		"DEPLOYMENT_MODE",
	}
	for _, def := range AllSiteSettings {
		keys = append(keys, strings.ToUpper(string(def.Key)))
	}

	for _, key := range keys {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}

	savedCfg := Cfg
	savedDefaults := make(map[*SiteSettingDef]string, len(AllSiteSettings))
	for _, def := range AllSiteSettings {
		savedDefaults[def] = def.Default
	}

	t.Cleanup(func() {
		Cfg = savedCfg
		for def, value := range savedDefaults {
			def.Default = value
		}
	})
}

func TestAllSiteSettings_DefaultsPassValidation(t *testing.T) {
	for _, def := range AllSiteSettings {
		t.Run(string(def.Key), func(t *testing.T) {
			// given
			value := def.Default

			// when
			err := ValidateSettingValue(def, value)

			// then
			require.NoError(t, err)
		})
	}
}

func TestSettingByKey_FindsEveryDefinition(t *testing.T) {
	for _, def := range AllSiteSettings {
		t.Run(string(def.Key), func(t *testing.T) {
			// given
			key := def.Key

			// when
			found, ok := SettingByKey(key)

			// then
			require.True(t, ok)
			assert.Same(t, def, found)
		})
	}
}

func TestSettingByKey_MissesUnknownKey(t *testing.T) {
	// given
	key := SiteSettingKey("no_such_setting")

	// when
	def, ok := SettingByKey(key)

	// then
	assert.False(t, ok)
	assert.Nil(t, def)
}

func TestValidateSettingValue(t *testing.T) {
	cases := []struct {
		name    string
		def     *SiteSettingDef
		value   string
		wantErr string
	}{
		{"a whole number is accepted", SettingMaxBodySize, "2048", ""},
		{"a non-integer is refused", SettingMaxBodySize, "1MB", "max_body_size must be a whole number"},
		{"true is accepted", SettingMaintenanceMode, "true", ""},
		{"a non-boolean is refused", SettingMaintenanceMode, "yes", "maintenance_mode must be true or false"},
		{"any text is accepted for a string", SettingSiteName, "Anything at all", ""},
		{"a public site is accepted", SettingSiteMode, "public", ""},
		{"a private site is accepted", SettingSiteMode, "private", ""},
		{"an unknown site mode is refused", SettingSiteMode, "friends", "site_mode must be one of public, private"},
		{"the smtp provider is accepted", SettingEmailProvider, "smtp", ""},
		{"the cloudflare provider is accepted", SettingEmailProvider, "cloudflare", ""},
		{"an unknown email provider is refused", SettingEmailProvider, "carrier-pigeon", "email_provider must be one of smtp, cloudflare"},
		{"a one-day session is accepted", SettingSessionDurationDays, "1", ""},
		{"a zero-day session is refused", SettingSessionDurationDays, "0", "session_duration_days must be at least 1"},
		{"a negative minimum password length is refused", SettingMinPasswordLength, "-4", "min_password_length must be at least 1"},
		{"a zero cache size is not held to the positive rule", SettingCacheInMemoryMaxMB, "0", ""},
		{"two organisations per owner are accepted", SettingMaxFreeOrgsPerOwner, "2", ""},
		{"no organisations per owner is refused, because onboarding could never create one", SettingMaxFreeOrgsPerOwner, "0", "max_free_orgs_per_owner must be at least 1"},
		{"a one-day guest device is accepted", SettingGuestDeviceDays, "1", ""},
		{"a zero-day guest device is refused", SettingGuestDeviceDays, "0", "guest_device_days must be at least 1"},
		{"a board of one participant is accepted", SettingMaxBoardParticipants, "1", ""},
		{"a board of no participants is refused, because nobody could join it", SettingMaxBoardParticipants, "0", "max_board_participants must be at least 1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			def, value := tc.def, tc.value

			// when
			err := ValidateSettingValue(def, value)

			// then
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestValidateSettings_AcceptsDefaultsAndIgnoresUnknownKeys(t *testing.T) {
	// given
	all := defaultSettings()
	all["retired_setting"] = "not a number"

	// when
	err := ValidateSettings(all)

	// then
	require.NoError(t, err)
}

func TestValidateSettings_ReportsStoredNonIntegerMaxBodySize(t *testing.T) {
	// given
	all := defaultSettings()
	all[SettingMaxBodySize.Key] = "lots"

	// when
	err := ValidateSettings(all)

	// then
	require.EqualError(t, err, "max_body_size must be a whole number")
}

func TestValidateSettings_ReportsTheFirstErrorInKeyOrder(t *testing.T) {
	// given
	all := defaultSettings()
	all[SettingMaxBodySize.Key] = "lots"
	all[SettingCacheInMemoryMaxMB.Key] = "plenty"

	// when
	err := ValidateSettings(all)

	// then
	require.EqualError(t, err, "cache_in_memory_max_mb must be a whole number")
}

func TestValidateSettings_CloudflareEmailNeedsItsCredentials(t *testing.T) {
	cases := []struct {
		name    string
		missing *SiteSettingDef
	}{
		{"without an account ID", SettingCloudflareAccountID},
		{"without an API token", SettingCloudflareAPIToken},
		{"without a from address", SettingCloudflareEmailFrom},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			all := defaultSettings()
			all[SettingEmailProvider.Key] = string(EmailProviderCloudflare)
			all[SettingCloudflareAccountID.Key] = "account"
			all[SettingCloudflareAPIToken.Key] = "token"
			all[SettingCloudflareEmailFrom.Key] = "RetroTide <hello@retrotide.app>"
			all[tc.missing.Key] = ""

			// when
			err := ValidateSettings(all)

			// then
			require.EqualError(t, err, "the cloudflare email provider requires cloudflare_account_id, cloudflare_api_token and cloudflare_email_from")
		})
	}
}

func TestValidateSettings_AcceptsCloudflareEmailWithItsCredentials(t *testing.T) {
	// given
	all := defaultSettings()
	all[SettingEmailProvider.Key] = string(EmailProviderCloudflare)
	all[SettingCloudflareAccountID.Key] = "account"
	all[SettingCloudflareAPIToken.Key] = "token"
	all[SettingCloudflareEmailFrom.Key] = "RetroTide <hello@retrotide.app>"

	// when
	err := ValidateSettings(all)

	// then
	require.NoError(t, err)
}

func TestValidateSettings_SMTPIgnoresTheCloudflareCredentials(t *testing.T) {
	// given
	all := defaultSettings()
	all[SettingEmailProvider.Key] = string(EmailProviderSMTP)

	// when
	err := ValidateSettings(all)

	// then
	require.NoError(t, err)
}

func TestSecretSettings_AreMarkedSecret(t *testing.T) {
	cases := []*SiteSettingDef{
		SettingMetricsToken,
		SettingValkeyURL,
		SettingTurnstileSecretKey,
		SettingSMTPPassword,
		SettingCloudflareAPIToken,
		SettingGoogleClientSecret,
		SettingMicrosoftClientSecret,
	}

	for _, def := range cases {
		t.Run(string(def.Key), func(t *testing.T) {
			// given
			found, ok := SettingByKey(def.Key)

			// when
			secret := ok && found.Secret

			// then
			assert.True(t, secret)
		})
	}
}

func TestGoogleSettings_StartEmptySoSignInWithGoogleIsOff(t *testing.T) {
	cases := []struct {
		def    *SiteSettingDef
		secret bool
	}{
		{def: SettingGoogleClientID, secret: false},
		{def: SettingGoogleClientSecret, secret: true},
	}

	for _, tc := range cases {
		t.Run(string(tc.def.Key), func(t *testing.T) {
			// given
			key := tc.def.Key

			// when
			found, ok := SettingByKey(key)

			// then only the secret half is masked, and neither holds a value until an admin sets one
			require.True(t, ok)
			assert.Equal(t, TypeString, found.Type)
			assert.Empty(t, found.Default)
			assert.Equal(t, tc.secret, found.Secret)
		})
	}
}

func TestM12cSettings_DefaultsKeepPasswordSignInOnAndMicrosoftOff(t *testing.T) {
	cases := []struct {
		def          *SiteSettingDef
		wantType     SettingType
		wantDefault  string
		wantIsSecret bool
	}{
		{def: SettingAllowPasswordSignIn, wantType: TypeBool, wantDefault: "true"},
		{def: SettingMicrosoftClientID, wantType: TypeString, wantDefault: ""},
		{def: SettingMicrosoftClientSecret, wantType: TypeString, wantDefault: "", wantIsSecret: true},
		{def: SettingMicrosoftTenantID, wantType: TypeString, wantDefault: ""},
		{def: SettingMicrosoftPersonalAccounts, wantType: TypeBool, wantDefault: "false"},
	}

	for _, tc := range cases {
		t.Run(string(tc.def.Key), func(t *testing.T) {
			// given
			key := tc.def.Key

			// when
			found, ok := SettingByKey(key)

			// then
			require.True(t, ok)
			assert.Equal(t, tc.wantType, found.Type)
			assert.Equal(t, tc.wantDefault, found.Default)
			assert.Equal(t, tc.wantIsSecret, found.Secret)
		})
	}
}

func TestSettingSiteMode_DefaultsToPublic(t *testing.T) {
	// given
	def := SettingSiteMode

	// when
	value := SiteMode(def.Default)

	// then
	assert.Equal(t, SiteModePublic, value)
	assert.Equal(t, TypeString, def.Type)
	assert.False(t, def.Secret)
}

func TestSettingMaxFreeOrgsPerOwner_DefaultsToOne(t *testing.T) {
	// given
	def := SettingMaxFreeOrgsPerOwner

	// when
	found, ok := SettingByKey("max_free_orgs_per_owner")

	// then
	require.True(t, ok)
	assert.Same(t, def, found)
	assert.Equal(t, "1", def.Default)
	assert.Equal(t, TypeInt, def.Type)
	assert.False(t, def.Secret)
}

func TestLoad_TakesASiteModeOverride(t *testing.T) {
	// given
	prepareLoad(t)
	t.Setenv("SITE_MODE", "private")

	// when
	err := Load()

	// then
	require.NoError(t, err)
	assert.Equal(t, string(SiteModePrivate), SettingSiteMode.Default)
}

func TestLoad_RejectsAnUnknownSiteModeOverride(t *testing.T) {
	// given
	prepareLoad(t)
	before := SettingSiteMode.Default
	t.Setenv("SITE_MODE", "friends")

	// when
	err := Load()

	// then
	require.EqualError(t, err, "environment variable SITE_MODE: site_mode must be one of public, private")
	assert.Equal(t, before, SettingSiteMode.Default)
}

func TestLoad_WithoutFilesUsesDefaults(t *testing.T) {
	// given
	prepareLoad(t)

	// when
	err := Load()

	// then
	require.NoError(t, err)
	assert.Equal(t, ":4323", Cfg.ListenAddr)
	assert.Equal(t, "localhost", Cfg.Postgres.Host)
	assert.Equal(t, "5432", Cfg.Postgres.Port)
	assert.Equal(t, "disable", Cfg.Postgres.SSLMode)
	assert.Empty(t, Cfg.DatabaseURL)
	assert.Equal(t, DeploymentHosted, Cfg.DeploymentMode)
}

func TestLoad_DeploymentMode(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  DeploymentMode
	}{
		{name: "hosted is accepted", value: "hosted", want: DeploymentHosted},
		{name: "self hosted is accepted", value: "self_hosted", want: DeploymentSelfHosted},
		{name: "an empty value falls back to hosted", value: "", want: DeploymentHosted},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			prepareLoad(t)
			t.Setenv("DEPLOYMENT_MODE", tc.value)

			// when
			err := Load()

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, Cfg.DeploymentMode)
		})
	}
}

func TestLoad_RejectsAnUnknownDeploymentMode(t *testing.T) {
	// given
	prepareLoad(t)
	before := Cfg
	t.Setenv("DEPLOYMENT_MODE", "on_prem")

	// when
	err := Load()

	// then the process refuses to boot rather than guessing a mode
	require.EqualError(t, err, "environment variable DEPLOYMENT_MODE must be one of hosted, self_hosted")
	assert.Equal(t, before, Cfg)
}

func TestLoad_EnvFilesSupplyUnsetVariables(t *testing.T) {
	// given
	prepareLoad(t)
	require.NoError(t, os.WriteFile(".env", []byte("LISTEN_ADDR=:5000\n"), 0o600))
	require.NoError(t, os.WriteFile("postgres.env", []byte("POSTGRES_DB=retrotide\n"), 0o600))

	// when
	err := Load()

	// then
	require.NoError(t, err)
	assert.Equal(t, ":5000", Cfg.ListenAddr)
	assert.Equal(t, "retrotide", Cfg.Postgres.DB)
}

func TestLoad_ListenAddrFromEnvironmentWins(t *testing.T) {
	// given
	prepareLoad(t)
	require.NoError(t, os.WriteFile(".env", []byte("LISTEN_ADDR=:5000\n"), 0o600))
	t.Setenv("LISTEN_ADDR", ":4333")

	// when
	err := Load()

	// then
	require.NoError(t, err)
	assert.Equal(t, ":4333", Cfg.ListenAddr)
}

func TestLoad_RejectsInvalidOverride(t *testing.T) {
	// given
	prepareLoad(t)
	before := SettingMaxBodySize.Default
	t.Setenv("MAX_BODY_SIZE", "abc")

	// when
	err := Load()

	// then
	require.EqualError(t, err, "environment variable MAX_BODY_SIZE: max_body_size must be a whole number")
	assert.Equal(t, before, SettingMaxBodySize.Default)
}

func TestLoad_AppliesValidOverride(t *testing.T) {
	// given
	prepareLoad(t)
	t.Setenv("MAX_BODY_SIZE", "2097152")

	// when
	err := Load()

	// then
	require.NoError(t, err)
	assert.Equal(t, "2097152", SettingMaxBodySize.Default)
}

func TestLoad_RejectsUnparsableEnvFile(t *testing.T) {
	// given
	prepareLoad(t)
	require.NoError(t, os.WriteFile(".env", []byte("BAD-KEY=1\n"), 0o600))

	// when
	err := Load()

	// then
	require.ErrorContains(t, err, "load .env")
}

func TestPostgresDSN(t *testing.T) {
	parts := PostgresConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "retrotide",
		Password: "secret",
		DB:       "retrotide",
		SSLMode:  "disable",
	}

	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"DATABASE_URL is preferred over the parts", Config{Postgres: parts, DatabaseURL: "postgres://app@db.internal/boards"}, "postgres://app@db.internal/boards"},
		{"the parts build the DSN without DATABASE_URL", Config{Postgres: parts}, "postgres://retrotide:secret@localhost:5432/retrotide?sslmode=disable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cfg := tc.cfg

			// when
			dsn := cfg.PostgresDSN()

			// then
			assert.Equal(t, tc.want, dsn)
		})
	}
}
