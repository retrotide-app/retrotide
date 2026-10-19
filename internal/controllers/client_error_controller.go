package controllers

import (
	"net/http"
	"strings"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/logger"

	"github.com/gin-gonic/gin"
)

const (
	maxClientErrorBodyBytes = 4096

	maxClientErrorNameLength           = 128
	maxClientErrorMessageLength        = 1024
	maxClientErrorStackLength          = 2048
	maxClientErrorComponentStackLength = 1024
	maxClientErrorSourceLength         = 64
	maxClientErrorRouteLength          = 128
	maxClientErrorVersionLength        = 64

	unknownClientErrorRoute = "unknown"
	clientErrorLogMessage   = "client error"
)

func (s *Service) getAllClientErrorRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/client-errors", Before: gin.HandlersChain{s.Guards.ClientErrorLimit}, Handler: s.reportClientError},
	}
}

func (s *Service) reportClientError(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxClientErrorBodyBytes)

	req, ok := utils.BindJSON[dto.ClientErrorReportRequest](c)
	if !ok {
		return
	}

	logger.Ctx(c.Request.Context()).Error().
		Str("client_error_name", truncateClientErrorField(req.Name, maxClientErrorNameLength)).
		Str("client_error_message", truncateClientErrorField(req.Message, maxClientErrorMessageLength)).
		Str("client_error_stack", truncateClientErrorField(req.Stack, maxClientErrorStackLength)).
		Str("client_error_component_stack", truncateClientErrorField(req.ComponentStack, maxClientErrorComponentStackLength)).
		Str("client_error_source", truncateClientErrorField(req.Source, maxClientErrorSourceLength)).
		Str("client_error_route", clientErrorRoute(req.Route)).
		Str("client_error_version", truncateClientErrorField(req.Version, maxClientErrorVersionLength)).
		Int("client_error_sequence", req.Sequence).
		Msg(clientErrorLogMessage)

	c.Status(http.StatusNoContent)
}

func clientErrorRoute(route string) string {
	if !strings.HasPrefix(route, "/") || strings.ContainsAny(route, "?#") || len(route) > maxClientErrorRouteLength {
		return unknownClientErrorRoute
	}

	return route
}

func truncateClientErrorField(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	return strings.ToValidUTF8(value[:limit], "")
}
