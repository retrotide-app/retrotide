package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dronebl"
	"retrotide/internal/dto"
	"retrotide/internal/guest"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type (
	Deps struct {
		Settings   settings.Service
		BodyLimit  *BodyLimit
		DroneBL    dronebl.Service
		Sessions   *session.Manager
		Guests     *guest.Manager
		Authz      authz.Service
		LastSeenIP *LastSeenIP
	}
)

var (
	maintenanceOpenRoutes = map[string]struct{}{
		"/api/v1/site-info":    {},
		"/api/v1/auth/login":   {},
		"/api/v1/auth/session": {},
		socketRoute:            {},
	}
)

func Setup(engine *gin.Engine, deps Deps) error {
	if deps.Settings == nil {
		return errMissingSettings
	}

	if deps.BodyLimit == nil {
		return errMissingBodyLimit
	}

	if deps.DroneBL == nil {
		return errMissingDroneBL
	}

	if deps.Sessions == nil {
		return errMissingSessions
	}

	if deps.Guests == nil {
		return errMissingGuests
	}

	if deps.Authz == nil {
		return errMissingAuthz
	}

	if deps.LastSeenIP == nil {
		return errMissingLastSeenIP
	}

	crossOrigin := corsConfig(deps.Settings)
	if err := crossOrigin.Validate(); err != nil {
		return fmt.Errorf("validate cors config: %w", err)
	}

	engine.Use(
		Tracing(deps.Settings),
		Detach(),
		HostAuthorisation(deps.Settings),
		SecurityHeaders(deps.Settings),
		cors.New(crossOrigin),
		RejectPlainTextBodies(),
		RequireClientHeader(),
		AccessLog(),
		Maintenance(deps.Settings, deps.Sessions, deps.Authz),
		PrivateSite(deps.Settings, deps.Sessions, deps.Guests, deps.Authz),
		Metrics(),
		RequireCleanIP(deps.DroneBL, deps.Sessions),
		deps.LastSeenIP.Handle,
		ETag(),
		deps.BodyLimit.Handle,
	)

	return nil
}

func corsConfig(settingsSvc settings.Service) cors.Config {
	return cors.Config{
		AllowOriginWithContextFunc: func(c *gin.Context, origin string) bool {
			allowed, err := settingsSvc.Get(c.Request.Context(), config.SettingBaseURL)
			if err != nil {
				c.Error(err)
				return false
			}

			return origin == strings.TrimSuffix(allowed, "/")
		},
		AllowCredentials: true,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", clientHeader},
		ExposeHeaders:    []string{traceIDHeader},
	}
}

func Maintenance(settingsSvc settings.Service, sessions *session.Manager, authzSvc authz.Service) gin.HandlerFunc {
	staff := userGuard{sessions: sessions, authz: authzSvc}

	return func(c *gin.Context) {
		if !strings.HasPrefix(strings.ToLower(c.Request.URL.Path), "/api/") {
			return
		}

		if _, open := maintenanceOpenRoutes[c.FullPath()]; open {
			return
		}

		maintenance, err := settingsSvc.GetBool(c.Request.Context(), config.SettingMaintenanceMode)
		if err != nil {
			abortUnreadableSetting(c, err)
			return
		}

		if !maintenance {
			return
		}

		userID, state := staff.resolve(c)
		if state == sessionUnchecked {
			return
		}

		if state == sessionLive {
			allowed, err := authzSvc.Can(c.Request.Context(), userID, authz.PermManageSettings)
			if err != nil {
				abortUnavailable(c, fmt.Errorf("check whether user %s may pass maintenance: %w", userID, err))
				return
			}

			if allowed {
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: "site is under maintenance"})
	}
}

func baseURL(ctx context.Context, settingsSvc settings.Service) (*url.URL, error) {
	raw, err := settingsSvc.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse base_url: %w", err)
	}

	return parsed, nil
}
