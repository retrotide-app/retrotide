package utils

import (
	"fmt"

	"retrotide/internal/middleware"

	"github.com/gin-gonic/gin"
)

var (
	DefaultTrustedProxies = []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}
)

func NewEngine(trustedProxies []string) (*gin.Engine, error) {
	engine := gin.New()
	engine.ContextWithFallback = true
	engine.HandleMethodNotAllowed = true
	engine.RedirectTrailingSlash = false
	engine.ForwardedByClientIP = true
	engine.RemoteIPHeaders = []string{"CF-Connecting-IP"}
	engine.TrustedPlatform = ""

	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	engine.Use(middleware.CacheHeaders(), middleware.Recover(), middleware.ErrorResponder())

	return engine, nil
}
