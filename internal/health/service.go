package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	healthgo "github.com/hellofresh/health-go/v5"
)

const (
	checkTimeout = 3 * time.Second
)

type (
	Service interface {
		Measure(ctx context.Context) healthgo.Check
	}

	Pinger interface {
		Ping(ctx context.Context) error
	}
)

func NewService(database *sql.DB, cache Pinger, settingsService settings.Service, version string) (Service, error) {
	siteName, err := settingsService.Get(context.Background(), config.SettingSiteName)
	if err != nil {
		return nil, fmt.Errorf("read the site name for the health report: %w", err)
	}

	checker, err := healthgo.New(
		healthgo.WithComponent(healthgo.Component{
			Name:    siteName,
			Version: version,
		}),
		healthgo.WithChecks(
			healthgo.Config{
				Name:      "postgres",
				Timeout:   checkTimeout,
				SkipOnErr: false,
				Check:     database.PingContext,
			},
			healthgo.Config{
				Name:      "valkey",
				Timeout:   checkTimeout,
				SkipOnErr: true,
				Check:     cache.Ping,
			},
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build health checks: %w", err)
	}

	return checker, nil
}
