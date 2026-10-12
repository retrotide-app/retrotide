package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Detach() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			return
		}

		c.Request = c.Request.WithContext(context.WithoutCancel(c.Request.Context()))
	}
}
