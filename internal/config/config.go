package config

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type (
	Config struct {
		Postgres       PostgresConfig
		DatabaseURL    string
		ListenAddr     string
		DeploymentMode DeploymentMode
	}

	PostgresConfig struct {
		Host     string
		Port     string
		User     string
		Password string
		DB       string
		SSLMode  string
	}

	SettingType int

	SiteSettingKey string

	SiteSettingDef struct {
		Key     SiteSettingKey
		Default string
		Type    SettingType
		Secret  bool
	}

	SiteMode string

	EmailProvider string

	DeploymentMode string
)

const (
	TypeString SettingType = iota
	TypeBool
	TypeInt
)

const (
	SiteModePublic  SiteMode = "public"
	SiteModePrivate SiteMode = "private"

	EmailProviderSMTP       EmailProvider = "smtp"
	EmailProviderCloudflare EmailProvider = "cloudflare"

	DeploymentHosted     DeploymentMode = "hosted"
	DeploymentSelfHosted DeploymentMode = "self_hosted"

	SecretMask = "********"

	deploymentModeEnv = "DEPLOYMENT_MODE"
)

var (
	Cfg Config

	Version = "dev"

	SettingBaseURL            = &SiteSettingDef{"base_url", "http://localhost:4323", TypeString, false}
	SettingSiteName           = &SiteSettingDef{"site_name", "RetroTide", TypeString, false}
	SettingLogLevel           = &SiteSettingDef{"log_level", "info", TypeString, false}
	SettingOTLPEndpoint       = &SiteSettingDef{"otlp_endpoint", "", TypeString, false}
	SettingPyroscopeURL       = &SiteSettingDef{"pyroscope_url", "", TypeString, false}
	SettingMaxBodySize        = &SiteSettingDef{"max_body_size", "1048576", TypeInt, false}
	SettingMaintenanceMode    = &SiteSettingDef{"maintenance_mode", "false", TypeBool, false}
	SettingMetricsToken       = &SiteSettingDef{"metrics_token", "", TypeString, true}
	SettingValkeyURL          = &SiteSettingDef{"valkey_url", "", TypeString, true}
	SettingCacheInMemoryMaxMB = &SiteSettingDef{"cache_in_memory_max_mb", "128", TypeInt, false}

	SettingSiteMode            = &SiteSettingDef{"site_mode", string(SiteModePublic), TypeString, false}
	SettingMinPasswordLength   = &SiteSettingDef{"min_password_length", "8", TypeInt, false}
	SettingSessionDurationDays = &SiteSettingDef{"session_duration_days", "30", TypeInt, false}
	SettingAllowPasswordSignIn = &SiteSettingDef{"allow_password_sign_in", "true", TypeBool, false}

	SettingTurnstileEnabled   = &SiteSettingDef{"turnstile_enabled", "false", TypeBool, false}
	SettingTurnstileSiteKey   = &SiteSettingDef{"turnstile_site_key", "", TypeString, false}
	SettingTurnstileSecretKey = &SiteSettingDef{"turnstile_secret_key", "", TypeString, true}

	SettingDroneBLEnabled        = &SiteSettingDef{"dronebl_enabled", "false", TypeBool, false}
	SettingDroneBLAllowlist      = &SiteSettingDef{"dronebl_allowlist", "", TypeString, false}
	SettingDroneBLIgnoredClasses = &SiteSettingDef{"dronebl_ignored_classes", "", TypeString, false}

	SettingEmailProvider       = &SiteSettingDef{"email_provider", string(EmailProviderSMTP), TypeString, false}
	SettingSMTPHost            = &SiteSettingDef{"smtp_host", "", TypeString, false}
	SettingSMTPPort            = &SiteSettingDef{"smtp_port", "25", TypeInt, false}
	SettingSMTPFrom            = &SiteSettingDef{"smtp_from", "", TypeString, false}
	SettingSMTPUsername        = &SiteSettingDef{"smtp_username", "", TypeString, false}
	SettingSMTPPassword        = &SiteSettingDef{"smtp_password", "", TypeString, true}
	SettingCloudflareAccountID = &SiteSettingDef{"cloudflare_account_id", "", TypeString, false}
	SettingCloudflareAPIToken  = &SiteSettingDef{"cloudflare_api_token", "", TypeString, true}
	SettingCloudflareEmailFrom = &SiteSettingDef{"cloudflare_email_from", "", TypeString, false}

	SettingGoogleClientID     = &SiteSettingDef{"google_client_id", "", TypeString, false}
	SettingGoogleClientSecret = &SiteSettingDef{"google_client_secret", "", TypeString, true}

	SettingMicrosoftClientID         = &SiteSettingDef{"microsoft_client_id", "", TypeString, false}
	SettingMicrosoftClientSecret     = &SiteSettingDef{"microsoft_client_secret", "", TypeString, true}
	SettingMicrosoftTenantID         = &SiteSettingDef{"microsoft_tenant_id", "", TypeString, false}
	SettingMicrosoftPersonalAccounts = &SiteSettingDef{"microsoft_personal_accounts", "false", TypeBool, false}

	SettingMaxFreeOrgsPerOwner = &SiteSettingDef{"max_free_orgs_per_owner", "1", TypeInt, false}

	SettingGuestDeviceDays      = &SiteSettingDef{"guest_device_days", "90", TypeInt, false}
	SettingMaxBoardParticipants = &SiteSettingDef{"max_board_participants", "150", TypeInt, false}

	AllSiteSettings = []*SiteSettingDef{
		SettingBaseURL,
		SettingSiteName,
		SettingLogLevel,
		SettingOTLPEndpoint,
		SettingPyroscopeURL,
		SettingMaxBodySize,
		SettingMaintenanceMode,
		SettingMetricsToken,
		SettingValkeyURL,
		SettingCacheInMemoryMaxMB,
		SettingSiteMode,
		SettingMinPasswordLength,
		SettingSessionDurationDays,
		SettingAllowPasswordSignIn,
		SettingTurnstileEnabled,
		SettingTurnstileSiteKey,
		SettingTurnstileSecretKey,
		SettingDroneBLEnabled,
		SettingDroneBLAllowlist,
		SettingDroneBLIgnoredClasses,
		SettingEmailProvider,
		SettingSMTPHost,
		SettingSMTPPort,
		SettingSMTPFrom,
		SettingSMTPUsername,
		SettingSMTPPassword,
		SettingCloudflareAccountID,
		SettingCloudflareAPIToken,
		SettingCloudflareEmailFrom,
		SettingGoogleClientID,
		SettingGoogleClientSecret,
		SettingMicrosoftClientID,
		SettingMicrosoftClientSecret,
		SettingMicrosoftTenantID,
		SettingMicrosoftPersonalAccounts,
		SettingMaxFreeOrgsPerOwner,
		SettingGuestDeviceDays,
		SettingMaxBoardParticipants,
	}

	allowedValues = map[SiteSettingKey][]string{
		SettingSiteMode.Key:      {string(SiteModePublic), string(SiteModePrivate)},
		SettingEmailProvider.Key: {string(EmailProviderSMTP), string(EmailProviderCloudflare)},
	}

	positiveSettings = []SiteSettingKey{
		SettingMinPasswordLength.Key,
		SettingSessionDurationDays.Key,
		SettingMaxFreeOrgsPerOwner.Key,
		SettingGuestDeviceDays.Key,
		SettingMaxBoardParticipants.Key,
	}

	cloudflareEmailSettings = []*SiteSettingDef{
		SettingCloudflareAccountID,
		SettingCloudflareAPIToken,
		SettingCloudflareEmailFrom,
	}

	deploymentModes = []DeploymentMode{DeploymentHosted, DeploymentSelfHosted}

	settingsByKey = indexSettings()
)

func indexSettings() map[SiteSettingKey]*SiteSettingDef {
	byKey := make(map[SiteSettingKey]*SiteSettingDef, len(AllSiteSettings))
	for _, def := range AllSiteSettings {
		byKey[def.Key] = def
	}

	return byKey
}

func SettingByKey(key SiteSettingKey) (*SiteSettingDef, bool) {
	def, ok := settingsByKey[key]

	return def, ok
}

func ValidateSettingValue(def *SiteSettingDef, value string) error {
	switch def.Type {
	case TypeInt:
		number, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be a whole number", def.Key)
		}

		if number < 1 && slices.Contains(positiveSettings, def.Key) {
			return fmt.Errorf("%s must be at least 1", def.Key)
		}
	case TypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("%s must be true or false", def.Key)
		}
	case TypeString:
		allowed, restricted := allowedValues[def.Key]
		if restricted && !slices.Contains(allowed, value) {
			return fmt.Errorf("%s must be one of %s", def.Key, strings.Join(allowed, ", "))
		}
	}

	return nil
}

func ValidateSettings(all map[SiteSettingKey]string) error {
	for _, key := range slices.Sorted(maps.Keys(all)) {
		def, ok := settingsByKey[key]
		if !ok {
			continue
		}

		if err := ValidateSettingValue(def, all[key]); err != nil {
			return err
		}
	}

	if EmailProvider(all[SettingEmailProvider.Key]) != EmailProviderCloudflare {
		return nil
	}

	for _, def := range cloudflareEmailSettings {
		if all[def.Key] == "" {
			return fmt.Errorf("the cloudflare email provider requires %s, %s and %s", SettingCloudflareAccountID.Key, SettingCloudflareAPIToken.Key, SettingCloudflareEmailFrom.Key)
		}
	}

	return nil
}

func Load() error {
	for _, file := range []string{".env", "postgres.env"} {
		if err := godotenv.Load(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("load %s: %w", file, err)
		}
	}

	mode := DeploymentMode(cmp.Or(os.Getenv(deploymentModeEnv), string(DeploymentHosted)))
	if !slices.Contains(deploymentModes, mode) {
		return fmt.Errorf("environment variable %s must be one of %s, %s", deploymentModeEnv, DeploymentHosted, DeploymentSelfHosted)
	}

	Cfg = Config{
		Postgres: PostgresConfig{
			Host:     envOr("POSTGRES_HOST", "localhost"),
			Port:     envOr("POSTGRES_PORT", "5432"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			DB:       os.Getenv("POSTGRES_DB"),
			SSLMode:  envOr("POSTGRES_SSL_MODE", "disable"),
		},
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		ListenAddr:     envOr("LISTEN_ADDR", ":4323"),
		DeploymentMode: mode,
	}

	for _, def := range AllSiteSettings {
		envKey := strings.ToUpper(string(def.Key))
		value, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		if err := ValidateSettingValue(def, value); err != nil {
			return fmt.Errorf("environment variable %s: %w", envKey, err)
		}

		def.Default = value
	}

	return nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return fallback
}

func (c Config) PostgresDSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.Postgres.User, c.Postgres.Password),
		Host:     c.Postgres.Host + ":" + c.Postgres.Port,
		Path:     "/" + c.Postgres.DB,
		RawQuery: "sslmode=" + url.QueryEscape(c.Postgres.SSLMode),
	}

	return u.String()
}
