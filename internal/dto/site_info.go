package dto

type (
	SiteInfoResponse struct {
		SiteName          string                     `json:"site_name"`
		DeploymentMode    string                     `json:"deployment_mode"`
		SiteMode          string                     `json:"site_mode"`
		RegistrationOpen  bool                       `json:"registration_open"`
		PasswordSignIn    bool                       `json:"password_sign_in"`
		EmailEnabled      bool                       `json:"email_enabled"`
		MaintenanceMode   bool                       `json:"maintenance_mode"`
		TurnstileEnabled  bool                       `json:"turnstile_enabled"`
		TurnstileSiteKey  string                     `json:"turnstile_site_key"`
		IdentityProviders []IdentityProviderResponse `json:"identity_providers"`
	}
)
