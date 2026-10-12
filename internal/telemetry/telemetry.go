package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

type (
	SettingsListener struct{}
)

var (
	mu               sync.Mutex
	tp               *sdktrace.TracerProvider
	currentProcessor sdktrace.SpanProcessor
	currentDSN       string
)

func Init(ctx context.Context, serviceName, version, endpoint string) error {
	mu.Lock()
	defer mu.Unlock()

	if tp != nil {
		return nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
		resource.WithHost(),
		resource.WithProcessPID(),
	)
	if err != nil {
		return fmt.Errorf("otel resource: %w", err)
	}

	tp = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return applyLocked(ctx, endpoint)
}

func Apply(endpoint string) error {
	mu.Lock()
	defer mu.Unlock()

	if tp == nil {
		return nil
	}

	return applyLocked(context.Background(), endpoint)
}

func applyLocked(ctx context.Context, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == currentDSN {
		return nil
	}

	var shutdownErr error
	if currentProcessor != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := currentProcessor.Shutdown(shutdownCtx); err != nil {
			shutdownErr = fmt.Errorf("shut down span processor: %w", err)
		}
		cancel()

		tp.UnregisterSpanProcessor(currentProcessor)
		currentProcessor = nil
	}

	currentDSN = ""

	if endpoint == "" {
		logger.Ctx(ctx).Info().Msg("otel tracing disabled")
		return shutdownErr
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(endpoint + "/v1/traces")}
	if strings.HasPrefix(endpoint, "http://") {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return errors.Join(shutdownErr, fmt.Errorf("otlp exporter: %w", err))
	}

	processor := sdktrace.NewBatchSpanProcessor(exporter, sdktrace.WithBatchTimeout(5*time.Second))
	tp.RegisterSpanProcessor(processor)
	currentProcessor = processor
	currentDSN = endpoint

	logger.Ctx(ctx).Info().Str("endpoint", endpoint).Msg("otel tracing enabled")

	return shutdownErr
}

func Shutdown() error {
	mu.Lock()
	defer mu.Unlock()

	if tp == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider := tp
	tp = nil
	currentProcessor = nil
	currentDSN = ""

	if err := provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut down tracer provider: %w", err)
	}

	return nil
}

func NewSettingsListener() *SettingsListener {
	return &SettingsListener{}
}

func (l *SettingsListener) OnSettingChanged(key config.SiteSettingKey, value string) {
	if key != config.SettingOTLPEndpoint.Key {
		return
	}

	if err := Apply(value); err != nil {
		logger.Log.Warn().Err(err).Msg("failed to apply otel endpoint change")
	}
}
