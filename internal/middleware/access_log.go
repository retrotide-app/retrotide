package middleware

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"retrotide/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	redactedValue = "<redacted>"
)

var (
	redactedQueryKeys = map[string]struct{}{
		"token":        {},
		"access_token": {},
		"session":      {},
		"key":          {},
		"secret":       {},
		"password":     {},
		"code":         {},
		"state":        {},
	}

	ipDroppedRoutePrefixes = []string{"/api/v1/b/", "/api/v1/boards/"}

	redactedPathPrefixes = []string{"/b/", "/boards/"}
)

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		template := c.FullPath()
		route, keepIP := accessLogRoute(template, c.Request.URL.Path)
		query := redactQuery(c.Request.URL.RawQuery)

		ip := ""
		if keepIP {
			ip = c.ClientIP()
		}

		c.Next()

		status := c.Writer.Status()
		quiet := template == assetsRoute || (c.Request.Method == http.MethodGet && status < http.StatusBadRequest)
		if zerolog.GlobalLevel() > zerolog.DebugLevel && quiet {
			return
		}

		event := logger.Ctx(c.Request.Context()).WithLevel(accessLogLevel(status)).
			Int("status", status).
			Int("size", max(c.Writer.Size(), 0)).
			Dur("latency", time.Since(start)).
			Str("method", c.Request.Method).
			Str("route", route)

		if query != "" {
			event = event.Str("query", query)
		}

		if keepIP {
			event = event.Str("ip", ip)
		}

		event.Msg("request")
	}
}

func accessLogRoute(template string, path string) (string, bool) {
	if template != "" {
		private := template == socketRoute || slices.ContainsFunc(ipDroppedRoutePrefixes, func(prefix string) bool {
			return strings.HasPrefix(template, prefix)
		})

		return template, !private
	}

	lower := strings.ToLower(path)
	for _, prefix := range redactedPathPrefixes {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}

		redacted := path[:len(prefix)] + redactedValue
		if _, tail, hasTail := strings.Cut(path[len(prefix):], "/"); hasTail {
			redacted += "/" + tail
		}

		return redacted, false
	}

	if strings.HasPrefix(lower, "/api/") {
		return otherRoute, true
	}

	return path, true
}

func redactQuery(rawQuery string) string {
	var (
		b   strings.Builder
		sep string
	)

	for pair := range strings.SplitSeq(rawQuery, "&") {
		b.WriteString(sep)
		sep = "&"

		key, _, hasValue := strings.Cut(pair, "=")
		if !hasValue || !sensitiveQueryKey(key) {
			b.WriteString(pair)
			continue
		}

		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(redactedValue)
	}

	return b.String()
}

func sensitiveQueryKey(rawKey string) bool {
	key, err := url.QueryUnescape(rawKey)
	if err != nil {
		return true
	}

	_, sensitive := redactedQueryKeys[strings.ToLower(key)]

	return sensitive
}

func accessLogLevel(status int) zerolog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return zerolog.ErrorLevel
	case status >= http.StatusBadRequest:
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}
