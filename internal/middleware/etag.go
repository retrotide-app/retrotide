package middleware

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type (
	etagWriter struct {
		gin.ResponseWriter
		status      int
		body        bytes.Buffer
		started     bool
		passThrough bool
		flushErr    error
	}
)

const (
	unwrittenSize   = -1
	eventStreamType = "text/event-stream"
	weakETagPrefix  = "W/"
)

var (
	etagTable = crc32.MakeTable(0xD5828281)

	etagSkippedRoutes = map[string]struct{}{
		socketRoute:      {},
		assetsRoute:      {},
		metricsRoute:     {},
		boardExportRoute: {},
		orgExportRoute:   {},
		selfExportRoute:  {},
		uploadsRoute:     {},
	}

	notModifiedDroppedHeaders = []string{"Content-Type", "Content-Length", "Content-Encoding"}
)

func ETag() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			return
		}

		route := c.FullPath()
		if _, skipped := etagSkippedRoutes[route]; skipped || strings.HasPrefix(route, pprofRoutePrefix) {
			return
		}

		original := c.Writer
		writer := &etagWriter{ResponseWriter: original, status: original.Status()}
		c.Writer = writer
		defer func() {
			c.Writer = original
		}()

		c.Next()

		if err := writer.finish(c.GetHeader("If-None-Match")); err != nil {
			c.Error(err)
		}
	}
}

func (w *etagWriter) finish(ifNoneMatch string) error {
	if w.passThrough {
		return w.flushErr
	}

	if !w.started {
		w.ResponseWriter.WriteHeader(w.status)
		return nil
	}

	body := w.body.Bytes()
	header := w.ResponseWriter.Header()

	if w.status == http.StatusOK && len(body) > 0 && !isEventStream(header.Get("Content-Type")) && header.Get("ETag") == "" {
		tag := fmt.Sprintf(`"%d-%d"`, len(body), crc32.Checksum(body, etagTable))
		header.Set("ETag", tag)

		if noneMatch(ifNoneMatch, tag) {
			for _, name := range notModifiedDroppedHeaders {
				header.Del(name)
			}

			w.ResponseWriter.WriteHeader(http.StatusNotModified)
			w.ResponseWriter.WriteHeaderNow()

			return nil
		}
	}

	w.ResponseWriter.WriteHeader(w.status)
	if len(body) == 0 {
		w.ResponseWriter.WriteHeaderNow()
		return nil
	}

	if _, err := w.ResponseWriter.Write(body); err != nil {
		return fmt.Errorf("write buffered response: %w", err)
	}

	return nil
}

func isEventStream(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")

	return strings.EqualFold(strings.TrimSpace(mediaType), eventStreamType)
}

func noneMatch(ifNoneMatch string, tag string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "*" {
		return true
	}

	for entry := range strings.SplitSeq(ifNoneMatch, ",") {
		if weakETagMatch(strings.TrimSpace(entry), tag) {
			return true
		}
	}

	return false
}

func weakETagMatch(a string, b string) bool {
	a = strings.TrimPrefix(a, weakETagPrefix)
	b = strings.TrimPrefix(b, weakETagPrefix)

	if len(a) < 2 || a[0] != '"' || a[len(a)-1] != '"' {
		return false
	}

	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return false
	}

	return a == b
}

func (w *etagWriter) WriteHeader(code int) {
	if w.passThrough {
		w.ResponseWriter.WriteHeader(code)
		return
	}

	if code > 0 && !w.started {
		w.status = code
	}
}

func (w *etagWriter) WriteHeaderNow() {
	if w.passThrough {
		w.ResponseWriter.WriteHeaderNow()
		return
	}

	w.started = true
}

func (w *etagWriter) Write(data []byte) (int, error) {
	if w.passThrough {
		return w.ResponseWriter.Write(data)
	}

	w.started = true

	return w.body.Write(data)
}

func (w *etagWriter) WriteString(s string) (int, error) {
	if w.passThrough {
		return w.ResponseWriter.WriteString(s)
	}

	w.started = true

	return w.body.WriteString(s)
}

func (w *etagWriter) Status() int {
	if w.passThrough {
		return w.ResponseWriter.Status()
	}

	return w.status
}

func (w *etagWriter) Size() int {
	if w.passThrough {
		return w.ResponseWriter.Size()
	}

	if !w.started {
		return unwrittenSize
	}

	return w.body.Len()
}

func (w *etagWriter) Written() bool {
	if w.passThrough {
		return w.ResponseWriter.Written()
	}

	return w.started
}

func (w *etagWriter) Flush() {
	if !w.passThrough {
		w.passThrough = true
		w.ResponseWriter.WriteHeader(w.status)

		if w.body.Len() > 0 {
			if _, err := w.ResponseWriter.Write(w.body.Bytes()); err != nil {
				w.flushErr = fmt.Errorf("flush buffered response: %w", err)
			}

			w.body.Reset()
		}
	}

	w.ResponseWriter.Flush()
}

func (w *etagWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
