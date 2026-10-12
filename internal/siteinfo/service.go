package siteinfo

import (
	"context"
	"fmt"

	"retrotide/internal/auth"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/identity"
	"retrotide/internal/settings"
)

type (
	Service interface {
		Get(ctx context.Context) (dto.SiteInfoResponse, error)
	}

	Deps struct {
		Settings       settings.Service
		Auth           auth.Service
		Identity       identity.Service
		Email          email.Service
		DeploymentMode config.DeploymentMode
	}

	service struct {
		settings       settings.Service
		auth           auth.Service
		identity       identity.Service
		email          email.Service
		deploymentMode config.DeploymentMode
	}
)

func NewService(deps Deps) Service {
	return &service{
		settings:       deps.Settings,
		auth:           deps.Auth,
		identity:       deps.Identity,
		email:          deps.Email,
		deploymentMode: deps.DeploymentMode,
	}
}

func (s *service) Get(ctx context.Context) (dto.SiteInfoResponse, error) {
	siteName, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("read the site name: %w", err)
	}

	mode, err := settings.ReadSiteMode(ctx, s.settings)
	if err != nil {
		return dto.SiteInfoResponse{}, err
	}

	registrationOpen, err := s.auth.RegistrationOpen(ctx)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("check whether registration is open: %w", err)
	}

	emailEnabled, err := s.email.Enabled(ctx)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("check whether the site can send email: %w", err)
	}

	maintenance, err := s.settings.GetBool(ctx, config.SettingMaintenanceMode)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("read the maintenance switch: %w", err)
	}

	turnstileEnabled, err := s.settings.GetBool(ctx, config.SettingTurnstileEnabled)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("read whether turnstile is enabled: %w", err)
	}

	turnstileSiteKey, err := s.settings.Get(ctx, config.SettingTurnstileSiteKey)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("read the turnstile site key: %w", err)
	}

	providers, err := s.identity.EnabledProviders(ctx)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("read the enabled sign-in providers: %w", err)
	}

	passwordSignIn, err := s.settings.GetBool(ctx, config.SettingAllowPasswordSignIn)
	if err != nil {
		return dto.SiteInfoResponse{}, fmt.Errorf("read whether password sign-in is allowed: %w", err)
	}

	return dto.SiteInfoResponse{
		SiteName:          siteName,
		DeploymentMode:    string(s.deploymentMode),
		SiteMode:          string(mode),
		RegistrationOpen:  registrationOpen,
		EmailEnabled:      emailEnabled,
		MaintenanceMode:   maintenance,
		TurnstileEnabled:  turnstileEnabled,
		TurnstileSiteKey:  turnstileSiteKey,
		IdentityProviders: providers,
		PasswordSignIn:    passwordSignIn,
	}, nil
}
