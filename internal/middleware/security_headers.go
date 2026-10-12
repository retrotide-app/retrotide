package middleware

import (
	"slices"
	"strings"

	"retrotide/internal/logger"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
)

type (
	responseHeader struct {
		name  string
		value string
	}
)

const (
	strictTransportSecurity = "max-age=15552000; includeSubDomains; preload"
	robotsTag               = "noindex, nofollow"
)

var (
	cspEnforcedDirectives = []string{
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"img-src 'self' data:",
	}

	cspReportOnlyDirectives = []string{
		"default-src 'self'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"script-src 'self' https://challenges.cloudflare.com",
		"style-src 'self' 'unsafe-inline'",
		"font-src 'self'",
		"img-src 'self' data:",
		"connect-src 'self'",
		"frame-src https://challenges.cloudflare.com",
		"worker-src 'self'",
	}

	securityHeaders = []responseHeader{
		{name: "X-Frame-Options", value: "DENY"},
		{name: "X-Content-Type-Options", value: "nosniff"},
		{name: "Referrer-Policy", value: "strict-origin-when-cross-origin"},
		{name: "Permissions-Policy", value: "geolocation=(), camera=(), microphone=(), display-capture=()"},
		{name: "Cross-Origin-Embedder-Policy", value: "unsafe-none"},
		{name: "Cross-Origin-Opener-Policy", value: "same-origin"},
		{name: "Cross-Origin-Resource-Policy", value: "same-origin"},
		{name: "Origin-Agent-Cluster", value: "?1"},
		{name: "X-DNS-Prefetch-Control", value: "off"},
		{name: "X-Download-Options", value: "noopen"},
		{name: "X-Permitted-Cross-Domain-Policies", value: "none"},
		{name: "X-XSS-Protection", value: "0"},
		{name: "Content-Security-Policy", value: strings.Join(cspEnforcedDirectives, "; ")},
		{name: "Content-Security-Policy-Report-Only", value: strings.Join(cspReportOnlyDirectives, "; ")},
	}

	robotsExcludedPaths = []string{"/b", "/boards", "/o", "/invite", "/api/v1/b", "/uploads"}
)

func SecurityHeaders(settingsSvc settings.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		for _, h := range securityHeaders {
			header.Set(h.name, h.value)
		}

		path := strings.ToLower(c.Request.URL.Path)
		excluded := slices.ContainsFunc(robotsExcludedPaths, func(prefix string) bool {
			return path == prefix || strings.HasPrefix(path, prefix+"/")
		})
		if excluded {
			header.Set("X-Robots-Tag", robotsTag)
		}

		base, err := baseURL(c.Request.Context(), settingsSvc)
		if err != nil {
			logger.Ctx(c.Request.Context()).Error().Err(err).Str("route", routeLabel(c)).Msg("strict transport security left off the response")
			return
		}

		if base.Scheme == "https" {
			header.Set("Strict-Transport-Security", strictTransportSecurity)
		}
	}
}
