package utils

import (
	"fmt"

	"retrotide/internal/config"
	"retrotide/internal/middleware"

	"github.com/gin-gonic/gin"
)

var (
	DefaultTrustedProxies = []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}

	clientIPHeaders = map[config.DeploymentMode]string{
		config.DeploymentHosted:     "CF-Connecting-IP",
		config.DeploymentSelfHosted: "X-Forwarded-For",
	}
)

func NewEngine(trustedProxies []string, mode config.DeploymentMode) (*gin.Engine, error) {
	clientIPHeader, known := clientIPHeaders[mode]
	if !known {
		return nil, fmt.Errorf("no client ip header for deployment mode %q", mode)
	}

	engine := gin.New()
	engine.ContextWithFallback = true
	engine.HandleMethodNotAllowed = true
	engine.RedirectTrailingSlash = false
	engine.ForwardedByClientIP = true
	engine.RemoteIPHeaders = []string{clientIPHeader}
	engine.TrustedPlatform = ""

	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	engine.Use(middleware.CacheHeaders(), middleware.Recover(), middleware.ErrorResponder())

	return engine, nil
}
