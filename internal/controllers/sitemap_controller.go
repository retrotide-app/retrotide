package controllers

import (
	"net/http"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/sitemap"

	"github.com/gin-gonic/gin"
)

const (
	robotsContentType  = "text/plain; charset=utf-8"
	sitemapContentType = "application/xml; charset=utf-8"
)

func (s *Service) getAllSitemapRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: sitemap.RobotsPath, Handler: s.serveRobots},
		{Method: http.MethodGet, Path: sitemap.SitemapPath, Handler: s.serveSitemap},
	}
}

func (s *Service) serveRobots(c *gin.Context) {
	body, err := s.SitemapService.Robots(c.Request.Context())
	if err != nil {
		utils.InternalError(c, "failed to build robots.txt", err)
		return
	}

	c.Data(http.StatusOK, robotsContentType, body)
}

func (s *Service) serveSitemap(c *gin.Context) {
	body, err := s.SitemapService.Sitemap(c.Request.Context())
	if err != nil {
		utils.InternalError(c, "failed to build the sitemap", err)
		return
	}

	c.Data(http.StatusOK, sitemapContentType, body)
}
