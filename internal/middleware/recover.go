package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"retrotide/internal/logger"

	"github.com/gin-gonic/gin"
)

func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}

			reqLogger := logger.Ctx(c.Request.Context())
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				reqLogger.Error().Str("route", routeLabel(c)).Str("causes", c.Errors.String()).Msg("response aborted after its first byte")
				panic(rec)
			}

			reqLogger.Error().Err(fmt.Errorf("panic: %v", rec)).Str("route", routeLabel(c)).Str("causes", c.Errors.String()).Str("stack", string(debug.Stack())).Msg("recovered from panic in request handler")

			if c.Writer.Written() {
				panic(http.ErrAbortHandler)
			}

			abortWithErrorJSON(c, http.StatusInternalServerError, "internal error")
		}()

		c.Next()
	}
}
