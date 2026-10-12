package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllSocketRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/ws", Handler: gin.WrapH(s.WSHandler)},
	}
}
