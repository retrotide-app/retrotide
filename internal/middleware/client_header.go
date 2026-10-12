package middleware

import (
	"net/http"
	"strings"

	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

const (
	clientHeader = "X-RetroTide-Client"
)

func RequireClientHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return
		}

		if !strings.HasPrefix(strings.ToLower(c.Request.URL.Path), "/api/v1/") {
			return
		}

		if c.GetHeader(clientHeader) != "" {
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "missing client header"})
	}
}
