package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"

	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
)

type (
	BodyLimit struct {
		maxBytes atomic.Int64
	}
)

const (
	minBodyLimit = 64 << 10
)

var (
	_ settings.Listener  = (*BodyLimit)(nil)
	_ settings.Validator = ValidateBodyLimit
)

func NewBodyLimit(initial int64) *BodyLimit {
	limit := &BodyLimit{}
	limit.maxBytes.Store(initial)

	return limit
}

func (b *BodyLimit) Handle(c *gin.Context) {
	if c.FullPath() == boardMediaRoute {
		return
	}

	limit := b.maxBytes.Load()
	if c.Request.ContentLength > limit {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, dto.ErrorResponse{Error: "request body too large"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
}

func (b *BodyLimit) OnSettingChanged(key config.SiteSettingKey, value string) {
	if key != config.SettingMaxBodySize.Key {
		return
	}

	size, err := ParseBodyLimit(value)
	if err != nil {
		logger.Log.Error().Err(err).Str("max_body_size", value).Int64("kept", b.maxBytes.Load()).Msg("failed to change the request body limit")
		return
	}

	b.maxBytes.Store(size)

	logger.Log.Info().Int64("max_body_size", size).Msg("request body limit changed")
}

func ParseBodyLimit(value string) (int64, error) {
	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("max_body_size must be a whole number of bytes: %w", err)
	}

	if size < minBodyLimit {
		return 0, fmt.Errorf("max_body_size must be at least %d bytes, got %d", minBodyLimit, size)
	}

	return size, nil
}

func ValidateBodyLimit(_ context.Context, value string) error {
	_, err := ParseBodyLimit(value)

	return err
}
