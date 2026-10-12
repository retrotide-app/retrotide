package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"syscall"

	"retrotide/internal/dto"
	"retrotide/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	unavailableMessage = "service unavailable"
)

var (
	errMissingSettings   = errors.New("middleware setup needs the settings service")
	errMissingBodyLimit  = errors.New("middleware setup needs the body limit")
	errMissingDroneBL    = errors.New("middleware setup needs the dronebl checker")
	errMissingSessions   = errors.New("middleware setup needs the session manager")
	errMissingGuests     = errors.New("middleware setup needs the guest device manager")
	errMissingAuthz      = errors.New("middleware setup needs the staff permission service")
	errMissingLastSeenIP = errors.New("middleware setup needs the last seen ip recorder")

	errTurnstileSecretMissing = errors.New("turnstile_enabled is on but turnstile_secret_key is empty")

	representationHeaders = []string{"Content-Type", "Content-Length", "Content-Encoding", "Content-Disposition", "Content-Range", "ETag", "Last-Modified"}
)

func ErrorResponder() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		reqLogger := logger.Ctx(c.Request.Context())
		for _, e := range c.Errors {
			reqLogger.WithLevel(causeLevel(c, e.Err)).Err(e.Err).Str("route", routeLabel(c)).Msg("request error")
		}

		status := c.Writer.Status()
		if c.Writer.Written() || (status < http.StatusBadRequest && len(c.Errors) == 0) {
			return
		}

		if status < http.StatusBadRequest {
			status = http.StatusInternalServerError
		}

		msg := "internal error"
		if status != http.StatusInternalServerError {
			msg = strings.ToLower(http.StatusText(status))
		}

		abortWithErrorJSON(c, status, msg)
	}
}

func causeLevel(c *gin.Context, err error) zerolog.Level {
	clientGone := errors.Is(err, context.Canceled) && c.Request.Context().Err() != nil
	if clientGone || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		return zerolog.InfoLevel
	}

	return zerolog.ErrorLevel
}

func abortWithErrorJSON(c *gin.Context, status int, msg string) {
	header := c.Writer.Header()
	for _, name := range representationHeaders {
		header.Del(name)
	}

	c.AbortWithStatusJSON(status, dto.ErrorResponse{Error: msg})
}

func abortUnreadableSetting(c *gin.Context, err error) {
	c.Error(err)
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: unavailableMessage})
}
