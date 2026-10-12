package middleware

import (
	"net/http"

	"retrotide/internal/logger"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName            = "retrotide/http"
	traceIDHeader         = "X-Trace-ID"
	unknownMethodSpanName = "HTTP"
)

func Tracing(settingsSvc settings.Service) gin.HandlerFunc {
	tracer := otel.Tracer(tracerName)

	return func(c *gin.Context) {
		route := routeLabel(c)
		if route == socketRoute {
			return
		}

		method := methodLabel(c)
		spanMethod := method
		attributes := []attribute.KeyValue{
			semconv.HTTPRequestMethodKey.String(method),
			semconv.HTTPRoute(route),
			semconv.UserAgentOriginal(c.Request.UserAgent()),
		}

		if method == otherMethod {
			spanMethod = unknownMethodSpanName
			attributes = append(attributes, semconv.HTTPRequestMethodOriginal(c.Request.Method))
		}

		parent := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := tracer.Start(parent, spanMethod+" "+route,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attributes...),
		)
		defer span.End()

		c.Request = c.Request.WithContext(ctx)

		if sc := span.SpanContext(); sc.IsValid() {
			c.Header(traceIDHeader, sc.TraceID().String())
		}

		base, err := baseURL(ctx, settingsSvc)
		if err != nil {
			logger.Ctx(ctx).Error().Err(err).Str("route", route).Msg("request span recorded without url.scheme")
		} else {
			span.SetAttributes(semconv.URLScheme(base.Scheme))
		}

		c.Next()

		status := c.Writer.Status()
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))

		switch {
		case len(c.Errors) > 0:
			span.SetStatus(codes.Error, c.Errors.String())
		case status >= http.StatusInternalServerError:
			span.SetStatus(codes.Error, http.StatusText(status))
		}
	}
}
