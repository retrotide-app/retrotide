package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	templateKeyParam = "key"
)

func (s *Service) getAllBoardTemplateRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/templates", Handler: s.listTemplates},
		{Method: http.MethodGet, Path: "/templates/:" + templateKeyParam, Handler: s.getTemplate},
	}
}

func (s *Service) listTemplates(c *gin.Context) {
	listing := s.BoardService.Templates()

	c.JSON(http.StatusOK, listing)
}

func (s *Service) getTemplate(c *gin.Context) {
	template, err := s.BoardService.Template(c.Param(templateKeyParam))
	if boardFailed(c, err, "failed to read the template") {
		return
	}

	c.JSON(http.StatusOK, template)
}
