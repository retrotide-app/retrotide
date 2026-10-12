package settings

import (
	"context"
	"fmt"

	"retrotide/internal/config"
)

func ReadSiteMode(ctx context.Context, svc Service) (config.SiteMode, error) {
	stored, err := svc.Get(ctx, config.SettingSiteMode)
	if err != nil {
		return "", fmt.Errorf("read the site mode: %w", err)
	}

	mode := config.SiteMode(stored)
	if mode != config.SiteModePublic && mode != config.SiteModePrivate {
		return "", fmt.Errorf("unknown site mode %q", stored)
	}

	return mode, nil
}
