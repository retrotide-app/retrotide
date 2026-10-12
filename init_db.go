package main

import (
	"context"
	"fmt"
	"os"

	"retrotide/internal/cache"
	"retrotide/internal/config"
	"retrotide/internal/db"
	"retrotide/internal/logger"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/store"
	"retrotide/internal/telemetry"
)

const (
	serviceName = "retrotide"
)

func initDatabase(cacheMgr *cache.Manager) (*repository.Repositories, settings.Service, error) {
	ctx := context.Background()

	if err := telemetry.Init(ctx, serviceName, config.Version, ""); err != nil {
		logger.Log.Warn().Err(err).Msg("otel init failed; traces disabled")
	}

	database, err := db.Open(config.Cfg.PostgresDSN())
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.RegisterStatsCollector(database); err != nil {
		return nil, nil, err
	}

	if err := db.Migrate(database); err != nil {
		return nil, nil, err
	}

	repos := store.New(database, cacheMgr)

	settingsSvc := settings.NewService(repos.Settings)
	if err := settingsSvc.Refresh(ctx); err != nil {
		return nil, nil, fmt.Errorf("load settings: %w", err)
	}

	logLevel, err := settingsSvc.Get(ctx, config.SettingLogLevel)
	if err != nil {
		return nil, nil, err
	}

	if err := logger.SetLevel(logLevel); err != nil {
		return nil, nil, fmt.Errorf("apply the stored log level: %w", err)
	}

	otlpEndpoint, err := settingsSvc.Get(ctx, config.SettingOTLPEndpoint)
	if err != nil {
		return nil, nil, err
	}

	if err := telemetry.Apply(otlpEndpoint); err != nil {
		logger.Log.Warn().Err(err).Msg("otel apply failed")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, nil, fmt.Errorf("read hostname: %w", err)
	}

	pyroscopeURL, err := settingsSvc.Get(ctx, config.SettingPyroscopeURL)
	if err != nil {
		return nil, nil, err
	}

	if err := telemetry.InitProfiling(serviceName, hostname, pyroscopeURL); err != nil {
		logger.Log.Warn().Err(err).Msg("pyroscope init failed; profiling disabled")
	}

	return repos, settingsSvc, nil
}
