package main

import (
	"context"
	"runtime/pprof"
	"sync"
	"time"

	"retrotide/internal/cache/engines"
	"retrotide/internal/config"
	"retrotide/internal/identity/microsoft"
	"retrotide/internal/logger"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/telemetry"
)

const (
	lastSeenIPRetention = 30 * 24 * time.Hour
	auditIPRetention    = 90 * 24 * time.Hour
	auditLogRetention   = 400 * 24 * time.Hour
)

func registerListeners(settingsSvc settings.Service, svc *services, repos *repository.Repositories, bodyLimit *middleware.BodyLimit) func(ctx context.Context) {
	settingsSvc.Subscribe(logger.NewSettingsListener())
	settingsSvc.Subscribe(telemetry.NewSettingsListener())
	settingsSvc.Subscribe(telemetry.NewProfilingSettingsListener())
	settingsSvc.Subscribe(bodyLimit)
	settingsSvc.Subscribe(svc.email)

	for _, provider := range svc.identityProviders {
		if listener, ok := provider.(settings.Listener); ok {
			settingsSvc.Subscribe(listener)
		}
	}

	for _, candidate := range svc.cache.Engines() {
		if listener, ok := candidate.(settings.Listener); ok {
			settingsSvc.Subscribe(listener)
		}
	}

	settingsSvc.RegisterValidator(config.SettingValkeyURL, engines.ProbeURL)
	settingsSvc.RegisterValidator(config.SettingMaxBodySize, middleware.ValidateBodyLimit)
	settingsSvc.RegisterValidator(config.SettingLogLevel, logger.ValidateLevel)
	settingsSvc.RegisterValidator(config.SettingMicrosoftTenantID, microsoft.ValidateTenantID)

	stop := make(chan struct{})
	wg := new(sync.WaitGroup)

	scheduleJob(stop, wg, "clean expired sessions", "cleaned expired sessions", time.Hour, func(ctx context.Context) (int64, error) {
		return repos.Session.CleanExpired(ctx)
	})
	scheduleJob(stop, wg, "clean expired guest devices", "cleaned expired guest devices", 24*time.Hour, func(ctx context.Context) (int64, error) {
		cutoff := time.Now().Add(-model.GuestDeviceUnseenLimit)

		return repos.GuestDevice.CleanExpired(ctx, cutoff)
	})
	scheduleJob(stop, wg, "clean expired sign-in flows", "cleaned expired sign-in flows", time.Hour, func(ctx context.Context) (int64, error) {
		return repos.OIDCFlow.CleanExpired(ctx)
	})
	scheduleJob(stop, wg, "clean expired two-factor challenges", "cleaned expired two-factor challenges", time.Hour, func(ctx context.Context) (int64, error) {
		return repos.TwoFactor.CleanExpiredChallenges(ctx)
	})
	scheduleJob(stop, wg, "clean expired trusted devices", "cleaned expired trusted devices", 24*time.Hour, func(ctx context.Context) (int64, error) {
		return repos.TwoFactor.CleanExpiredTrustedDevices(ctx)
	})
	scheduleJob(stop, wg, "delete expired organisation invites", "deleted expired organisation invites", time.Hour, svc.invites.DeleteExpired)
	scheduleJob(stop, wg, "purge deleted organisations", "purged deleted organisations", 24*time.Hour, func(ctx context.Context) (int64, error) {
		cutoff := time.Now().Add(-model.OrganisationRestoreWindow)

		return repos.Organisation.PurgeDeletedBefore(ctx, cutoff)
	})
	scheduleJob(stop, wg, "clear last seen ips", "cleared last seen ips", 24*time.Hour, func(ctx context.Context) (int64, error) {
		cutoff := time.Now().Add(-lastSeenIPRetention)

		return repos.User.ClearLastSeenIPsBefore(ctx, cutoff)
	})
	scheduleJob(stop, wg, "clear audit log ips", "cleared audit log ips", 24*time.Hour, func(ctx context.Context) (int64, error) {
		cutoff := time.Now().Add(-auditIPRetention)

		return repos.AuditLog.ClearIPsBefore(ctx, cutoff)
	})
	scheduleJob(stop, wg, "end idle retros", "ended idle retros", time.Hour, svc.board.EndIdle)
	scheduleJob(stop, wg, "prune old notifications", "pruned old notifications", 24*time.Hour, svc.notification.PruneOld)
	scheduleJob(stop, wg, "purge unattached media", "purged unattached media", 24*time.Hour, svc.media.PurgeUnattached)
	scheduleJob(stop, wg, "purge deleted boards", "purged deleted boards", 24*time.Hour, svc.boardManagement.PurgeDeleted)
	scheduleJob(stop, wg, "prune old audit log", "pruned old audit log", 24*time.Hour, func(ctx context.Context) (int64, error) {
		cutoff := time.Now().Add(-auditLogRetention)

		return repos.AuditLog.DeleteCreatedBefore(ctx, cutoff)
	})

	return func(ctx context.Context) {
		close(stop)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Ctx(ctx).Info().Msg("background jobs stopped")
		case <-ctx.Done():
			logger.Ctx(ctx).Warn().Err(context.Cause(ctx)).Msg("background jobs did not stop in time")
		}
	}
}

func scheduleJob(stop <-chan struct{}, wg *sync.WaitGroup, name string, successMsg string, interval time.Duration, fn func(ctx context.Context) (int64, error)) {
	logger.Log.Info().Str("interval", interval.String()).Msgf("registered job: %s", name)

	wg.Go(func() {
		jobCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			<-stop
			cancel()
		}()

		pprof.Do(jobCtx, pprof.Labels("job", name), func(ctx context.Context) {
			run := func() {
				count, err := fn(ctx)
				if err != nil {
					logger.Ctx(ctx).Error().Err(err).Msgf("%s failed", name)
					return
				}

				if count > 0 {
					logger.Ctx(ctx).Info().Int64("count", count).Msg(successMsg)
				}
			}

			run()

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					run()
				}
			}
		})
	})
}
