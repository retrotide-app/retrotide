package controllers

import (
	"net/http"

	"retrotide/internal/controllers/utils"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllSiteInfoRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/site-info", Handler: s.siteInfo},
	}
}

func (s *Service) siteInfo(c *gin.Context) {
	info, err := s.SiteInfoService.Get(c.Request.Context())
	if err != nil {
		utils.InternalError(c, "failed to read the site info", err)
		return
	}

	c.JSON(http.StatusOK, info)
}
