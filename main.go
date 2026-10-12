package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/logger"
	"retrotide/internal/telemetry"
	"retrotide/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func main() {
	if err := run(); err != nil {
		logger.Log.Error().Err(err).Msg("server stopped with an error")
		os.Exit(1)
	}
}

func run() error {
	started := time.Now()

	if err := config.Load(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := logger.Init(config.SettingLogLevel.Default); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = logger.NewLevelWriter(zerolog.DebugLevel)
	gin.DefaultErrorWriter = logger.NewLevelWriter(zerolog.ErrorLevel)

	defer func() {
		if err := telemetry.Shutdown(); err != nil {
			logger.Log.Error().Err(err).Msg("tracing shutdown failed")
		}
	}()
	defer func() {
		if err := telemetry.ShutdownProfiling(); err != nil {
			logger.Log.Error().Err(err).Msg("profiling shutdown failed")
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Log.Info().
		Str("db_host", config.Cfg.Postgres.Host).
		Str("db_name", config.Cfg.Postgres.DB).
		Msg("starting")

	srv, routeCount, cleanup, err := initServer()
	if err != nil {
		return err
	}
	defer cleanup()

	announce := func(addr net.Addr) error {
		startup := time.Since(started)

		if logger.JSONOutput() {
			logger.Log.Info().Str("addr", addr.String()).Int("routes", routeCount).Str("version", config.Version).Dur("startup", startup).Msg("server started")
			return nil
		}

		return utils.WriteBanner(os.Stdout, utils.BannerInfo{
			Version: config.Version,
			Addr:    addr,
			Routes:  routeCount,
			PID:     os.Getpid(),
			Startup: startup,
		})
	}

	return utils.Serve(ctx, srv, announce)
}
