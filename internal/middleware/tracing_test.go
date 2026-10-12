package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		assert.NoError(t, provider.Shutdown(context.Background()))
	})

	return recorder
}

func tracingEngine(t *testing.T) *gin.Engine {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil).Maybe()

	engine := newTestEngine(t)
	engine.Use(Tracing(settingsSvc))
	engine.GET("/items/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})
	engine.GET("/broken", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	})

	return engine
}

func onlySpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()

	spans := recorder.Ended()
	require.Len(t, spans, 1)

	return spans[0]
}

func spanAttributes(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	values := make(map[attribute.Key]attribute.Value, len(span.Attributes()))
	for _, kv := range span.Attributes() {
		values[kv.Key] = kv.Value
	}

	return values
}

func TestTracing_NamesTheSpanByTheRouteTemplate(t *testing.T) {
	// given
	recorder := recordSpans(t)
	engine := tracingEngine(t)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/secret-42", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	span := onlySpan(t, recorder)
	attrs := spanAttributes(span)
	assert.Equal(t, "GET /items/:id", span.Name())
	assert.Equal(t, "/items/:id", attrs["http.route"].AsString())
	assert.Equal(t, "https", attrs["url.scheme"].AsString())
	assert.Equal(t, int64(http.StatusOK), attrs["http.response.status_code"].AsInt64())
	assert.NotContains(t, attrs, attribute.Key("url.path"))
	assert.NotContains(t, attrs, attribute.Key("client.address"))
	assert.Equal(t, span.SpanContext().TraceID().String(), rec.Header().Get("X-Trace-ID"))
}

func TestTracing_NamesAnUnmatchedPathOther(t *testing.T) {
	// given
	recorder := recordSpans(t)
	engine := tracingEngine(t)

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/b/secret-slug", nil))

	// then a share slug never names a span
	assert.Equal(t, "GET other", onlySpan(t, recorder).Name())
}

func TestTracing_NamesAnUnknownMethodByTheSemconvPlaceholder(t *testing.T) {
	// given
	recorder := recordSpans(t)
	engine := tracingEngine(t)

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), "FOO1", "/items/42", nil))

	// then a client cannot mint a new span name per method it invents
	span := onlySpan(t, recorder)
	attrs := spanAttributes(span)
	assert.Equal(t, "HTTP other", span.Name())
	assert.Equal(t, "_OTHER", attrs["http.request.method"].AsString())
	assert.Equal(t, "FOO1", attrs["http.request.method_original"].AsString())
}

func TestTracing_MarksAServerErrorOnTheSpan(t *testing.T) {
	// given
	recorder := recordSpans(t)
	engine := tracingEngine(t)

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/broken", nil))

	// then
	assert.Equal(t, codes.Error, onlySpan(t, recorder).Status().Code)
}

func TestTracing_LeavesTheSocketToItsOwnSpan(t *testing.T) {
	// given
	recorder := recordSpans(t)
	engine := tracingEngine(t)
	engine.GET(socketRoute, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, socketRoute, nil))

	// then
	assert.Empty(t, recorder.Ended())
	assert.Empty(t, rec.Header().Get("X-Trace-ID"))
}
