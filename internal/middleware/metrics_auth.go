package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
)

const (
	bearerPrefix = "Bearer "
)

func metricsToken(settingsSvc settings.Service, mode config.DeploymentMode) gin.HandlerFunc {
	return func(c *gin.Context) {
		expected, err := settingsSvc.Get(c.Request.Context(), config.SettingMetricsToken)
		if err != nil {
			abortUnreadableSetting(c, err)
			return
		}

		if expected == "" && mode != config.DeploymentSelfHosted {
			return
		}

		presented := ""
		if bearer, isBearer := strings.CutPrefix(c.GetHeader("Authorization"), bearerPrefix); isBearer {
			presented = strings.TrimSpace(bearer)
		}

		if presented == "" {
			presented = c.GetHeader("X-Metrics-Token")
		}

		if expected != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1 {
			return
		}

		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: "not found"})
	}
}
