package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type (
	cacheHeaderWriter struct {
		gin.ResponseWriter
		route   string
		path    string
		decided bool
	}
)

func CacheHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		writer := &cacheHeaderWriter{ResponseWriter: c.Writer, route: c.FullPath(), path: c.Request.URL.Path}
		c.Writer = writer

		c.Next()

		if !writer.Written() {
			writer.WriteHeaderNow()
		}
	}
}

func (w *cacheHeaderWriter) decide() {
	if w.decided {
		return
	}

	w.decided = true

	status := w.Status()
	header := w.Header()

	switch {
	case status < http.StatusOK:
		return
	case status >= http.StatusBadRequest:
		header.Set("Cache-Control", "private, no-store")
	case w.route == assetsRoute:
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	case w.route == uploadsRoute:
		header.Set("Cache-Control", "private, max-age=31536000, immutable")
	case strings.HasPrefix(w.path, "/api/") || w.route == robotsRoute || w.route == sitemapRoute:
		header.Set("Cache-Control", "no-cache, must-revalidate")
	default:
		header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		header.Set("Pragma", "no-cache")
		header.Set("Expires", "0")
	}
}

func (w *cacheHeaderWriter) WriteHeaderNow() {
	w.decide()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *cacheHeaderWriter) Write(data []byte) (int, error) {
	w.decide()

	return w.ResponseWriter.Write(data)
}

func (w *cacheHeaderWriter) WriteString(s string) (int, error) {
	w.decide()

	return w.ResponseWriter.WriteString(s)
}

func (w *cacheHeaderWriter) Flush() {
	w.decide()
	w.ResponseWriter.Flush()
}

func (w *cacheHeaderWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
