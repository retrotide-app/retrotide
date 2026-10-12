package middleware

import (
	"mime"
	"net/http"
	"strings"

	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

const (
	plainTextMediaType = "text/plain"
	identityEncoding   = "identity"
)

func RejectPlainTextBodies() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength == 0 {
			return
		}

		if contentType := c.GetHeader("Content-Type"); contentType != "" {
			mediaType, _, err := mime.ParseMediaType(contentType)
			if err != nil || mediaType == plainTextMediaType {
				c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, dto.ErrorResponse{Error: "unsupported content type"})
				return
			}
		}

		encoding := c.GetHeader("Content-Encoding")
		if encoding != "" && !strings.EqualFold(encoding, identityEncoding) {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, dto.ErrorResponse{Error: "unsupported content encoding"})
			return
		}
	}
}
