package admin

import (
	"context"
	"fmt"
	"time"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/settings"

	"github.com/google/uuid"
)

const (
	statsDay           = 24 * time.Hour
	statsWeek          = 7 * statsDay
	statsMonth         = 30 * statsDay
	statsSeriesDays    = 30
	recentSignupsShown = 10
)

func (s *service) GetStats(ctx context.Context, viewerID uuid.UUID) (*dto.AdminStatsResponse, error) {
	now := s.now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	totals, err := s.stats.CountTotals(ctx, now.Add(-statsWeek))
	if err != nil {
		return nil, err
	}

	activity, err := s.stats.CountActivity(ctx, spec.SiteActivityWindows{
		DaySince:   now.Add(-statsDay),
		WeekSince:  now.Add(-statsWeek),
		MonthSince: now.Add(-statsMonth),
	})
	if err != nil {
		return nil, err
	}

	daily, err := s.stats.ListDaily(ctx, spec.SiteDailySeries{FirstDay: today.AddDate(0, 0, 1-statsSeriesDays), Days: statsSeriesDays})
	if err != nil {
		return nil, err
	}

	health, err := s.siteHealth(ctx)
	if err != nil {
		return nil, err
	}

	recent, err := s.recentSignups(ctx, viewerID)
	if err != nil {
		return nil, err
	}

	return &dto.AdminStatsResponse{
		GeneratedAt:   now.Format(time.RFC3339),
		Totals:        dto.NewAdminStatTotals(*totals),
		Daily:         dto.NewAdminStatDays(daily),
		Activity:      dto.NewAdminActivity(*activity),
		RecentSignups: recent,
		Health:        *health,
	}, nil
}

func (s *service) recentSignups(ctx context.Context, viewerID uuid.UUID) (*[]dto.AdminRecentSignup, error) {
	mayViewUsers, err := s.authz.Can(ctx, viewerID, authz.PermViewUsers)
	if err != nil {
		return nil, fmt.Errorf("check whether the viewer may see user records: %w", err)
	}

	if !mayViewUsers {
		return nil, nil
	}

	rows, err := s.users.ListWithRoles(ctx, spec.UserListing{Limit: recentSignupsShown})
	if err != nil {
		return nil, fmt.Errorf("list the newest sign-ups: %w", err)
	}

	recent := make([]dto.AdminRecentSignup, 0, len(rows))
	for _, row := range rows {
		recent = append(recent, dto.NewAdminRecentSignup(row))
	}

	return &recent, nil
}

func (s *service) siteHealth(ctx context.Context) (*dto.AdminSiteHealth, error) {
	mode, err := settings.ReadSiteMode(ctx, s.settings)
	if err != nil {
		return nil, err
	}

	maintenance, err := s.settings.GetBool(ctx, config.SettingMaintenanceMode)
	if err != nil {
		return nil, fmt.Errorf("read the maintenance mode: %w", err)
	}

	emailStatus, err := s.email.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("check whether email is configured: %w", err)
	}

	googleConfigured, err := s.providerConfigured(ctx, model.ProviderGoogle)
	if err != nil {
		return nil, err
	}

	return &dto.AdminSiteHealth{
		SiteMode:               mode,
		MaintenanceMode:        maintenance,
		EmailConfigured:        emailStatus == email.StatusReady,
		EmailStatus:            string(emailStatus),
		GoogleSignInConfigured: googleConfigured,
	}, nil
}

func (s *service) providerConfigured(ctx context.Context, id model.IdentityProvider) (bool, error) {
	for _, provider := range s.signInProviders {
		if provider.ID() != id {
			continue
		}

		configured, err := provider.Enabled(ctx)
		if err != nil {
			return false, fmt.Errorf("check whether %s sign-in is configured: %w", id, err)
		}

		return configured, nil
	}

	return false, nil
}
