package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	otherRoute       = "other"
	otherMethod      = "_OTHER"
	socketRoute      = "/api/v1/ws"
	boardRoute       = "/api/v1/boards/:boardID"
	boardExportRoute = "/api/v1/boards/:boardID/export"
	boardMediaRoute  = "/api/v1/boards/:boardID/media"
	uploadsRoute     = "/uploads/:boardID/:file"
	orgExportRoute   = "/api/v1/orgs/:orgID/export"
	selfExportRoute  = "/api/v1/auth/export"
	assetsRoute      = "/assets/*filepath"
	metricsRoute     = "/metrics"
	robotsRoute      = "/robots.txt"
	sitemapRoute     = "/sitemap.xml"
	pprofRoutePrefix = "/debug/pprof/"
)

var (
	knownMethods = map[string]struct{}{
		http.MethodGet:     {},
		http.MethodHead:    {},
		http.MethodPost:    {},
		http.MethodPut:     {},
		http.MethodPatch:   {},
		http.MethodDelete:  {},
		http.MethodOptions: {},
		http.MethodConnect: {},
		http.MethodTrace:   {},
	}
)

func routeLabel(c *gin.Context) string {
	route := c.FullPath()
	if route == "" {
		return otherRoute
	}

	return route
}

func methodLabel(c *gin.Context) string {
	method := c.Request.Method
	if _, known := knownMethods[method]; !known {
		return otherMethod
	}

	return method
}
