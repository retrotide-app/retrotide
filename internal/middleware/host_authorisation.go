package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
)

var (
	hostAuthorisationExempt = map[string]struct{}{
		"/health":    {},
		"/livez":     {},
		metricsRoute: {},
	}
)

func HostAuthorisation(settingsSvc settings.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exempt := hostAuthorisationExempt[c.FullPath()]; exempt {
			return
		}

		ctx := c.Request.Context()
		host := (&url.URL{Host: c.Request.Host}).Hostname()

		raw, err := settingsSvc.Get(ctx, config.SettingBaseURL)
		if err != nil {
			abortUnreadableSetting(c, err)
			return
		}

		base, err := url.Parse(raw)
		if err != nil {
			logger.Ctx(ctx).Error().Err(err).Str("host", c.Request.Host).Str("method", c.Request.Method).Str("route", routeLabel(c)).Msg("host authorisation refused the request because base_url does not parse")
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "forbidden"})
			return
		}

		expected := base.Hostname()
		if expected == "" {
			logger.Ctx(ctx).Error().Str("host", c.Request.Host).Str("method", c.Request.Method).Str("route", routeLabel(c)).Msg("host authorisation refused the request because base_url has no hostname")
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "forbidden"})
			return
		}

		if strings.EqualFold(host, expected) {
			return
		}

		logger.Ctx(ctx).Warn().Str("host", c.Request.Host).Str("method", c.Request.Method).Str("route", routeLabel(c)).Msg("host authorisation rejected the request")
		c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "forbidden"})
	}
}
