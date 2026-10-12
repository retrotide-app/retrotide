package observability

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "retrotide/internal/cache"

	statsTimeout = 2 * time.Second

	pipelineCommand = "pipeline"
)

type (
	ClientFunc func() valkey.Client

	Hook struct {
		tracer trace.Tracer
	}

	statsCollector struct {
		client ClientFunc

		up          *prometheus.Desc
		serverKeys  *prometheus.Desc
		memoryUsed  *prometheus.Desc
		memoryMax   *prometheus.Desc
		evictedKeys *prometheus.Desc

		connectedClients  *prometheus.Desc
		blockedClients    *prometheus.Desc
		connectionsTotal  *prometheus.Desc
		rejectedConns     *prometheus.Desc
		commandsProcessed *prometheus.Desc
	}
)

var (
	commandDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cache_command_duration_seconds",
			Help:    "Valkey command duration in seconds by command.",
			Buckets: []float64{0.000025, 0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"command"},
	)
	commandErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_command_errors_total",
			Help: "Number of Valkey commands that failed by command.",
		},
		[]string{"command"},
	)
)

func init() {
	prometheus.MustRegister(commandDuration, commandErrors)
}

func NewHook() *Hook {
	return &Hook{tracer: otel.Tracer(tracerName)}
}

func CallerGone(ctx context.Context, err error) bool {
	return ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
}

func RegisterStats(reg prometheus.Registerer, client ClientFunc) error {
	if err := reg.Register(newStatsCollector(client)); err != nil {
		return fmt.Errorf("register valkey stats collector: %w", err)
	}

	return nil
}

func (h *Hook) Do(client valkey.Client, ctx context.Context, cmd valkey.Completed) valkey.ValkeyResult {
	name := commandName(cmd.Commands())

	ctx, span := h.tracer.Start(ctx, "valkey "+name, trace.WithSpanKind(trace.SpanKindClient))
	start := time.Now()

	resp := client.Do(ctx, cmd)

	observe(ctx, span, name, start, resp.Error())

	return resp
}

func (h *Hook) DoMulti(client valkey.Client, ctx context.Context, multi ...valkey.Completed) []valkey.ValkeyResult {
	ctx, span := h.tracer.Start(ctx, "valkey "+pipelineCommand, trace.WithSpanKind(trace.SpanKindClient))
	start := time.Now()

	resps := client.DoMulti(ctx, multi...)

	observe(ctx, span, pipelineCommand, start, firstError(resps))

	return resps
}

func (h *Hook) DoCache(client valkey.Client, ctx context.Context, cmd valkey.Cacheable, ttl time.Duration) valkey.ValkeyResult {
	name := commandName(cmd.Commands())

	ctx, span := h.tracer.Start(ctx, "valkey "+name, trace.WithSpanKind(trace.SpanKindClient))
	start := time.Now()

	resp := client.DoCache(ctx, cmd, ttl)

	observe(ctx, span, name, start, resp.Error())

	return resp
}

func (h *Hook) DoMultiCache(client valkey.Client, ctx context.Context, multi ...valkey.CacheableTTL) []valkey.ValkeyResult {
	ctx, span := h.tracer.Start(ctx, "valkey "+pipelineCommand, trace.WithSpanKind(trace.SpanKindClient))
	start := time.Now()

	resps := client.DoMultiCache(ctx, multi...)

	observe(ctx, span, pipelineCommand, start, firstError(resps))

	return resps
}

func (h *Hook) Receive(client valkey.Client, ctx context.Context, subscribe valkey.Completed, fn func(msg valkey.PubSubMessage)) error {
	return client.Receive(ctx, subscribe, fn)
}

func (h *Hook) DoStream(client valkey.Client, ctx context.Context, cmd valkey.Completed) valkey.ValkeyResultStream {
	return client.DoStream(ctx, cmd)
}

func (h *Hook) DoMultiStream(client valkey.Client, ctx context.Context, multi ...valkey.Completed) valkey.MultiValkeyResultStream {
	return client.DoMultiStream(ctx, multi...)
}

func commandName(parts []string) string {
	if len(parts) == 0 {
		return "unknown"
	}

	return parts[0]
}

func firstError(resps []valkey.ValkeyResult) error {
	for _, resp := range resps {
		err := resp.Error()
		if err != nil && !valkey.IsValkeyNil(err) {
			return err
		}
	}

	return nil
}

func observe(ctx context.Context, span trace.Span, command string, start time.Time, err error) {
	commandDuration.WithLabelValues(command).Observe(time.Since(start).Seconds())

	if err != nil && !valkey.IsValkeyNil(err) && !CallerGone(ctx, err) {
		commandErrors.WithLabelValues(command).Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	span.End()
}

func newStatsCollector(client ClientFunc) *statsCollector {
	return &statsCollector{
		client:      client,
		up:          prometheus.NewDesc("cache_up", "Whether the Valkey cache is configured and reachable (1) or not (0).", nil, nil),
		serverKeys:  prometheus.NewDesc("cache_server_keys", "Number of keys stored in the Valkey cache database.", nil, nil),
		memoryUsed:  prometheus.NewDesc("cache_server_memory_used_bytes", "Bytes of memory used by the Valkey server.", nil, nil),
		memoryMax:   prometheus.NewDesc("cache_server_memory_max_bytes", "Configured Valkey maxmemory limit in bytes (0 means unlimited).", nil, nil),
		evictedKeys: prometheus.NewDesc("cache_server_evicted_keys_total", "Number of keys evicted by the Valkey server due to maxmemory.", nil, nil),

		connectedClients:  prometheus.NewDesc("cache_server_connected_clients", "Number of client connections currently open on the Valkey server.", nil, nil),
		blockedClients:    prometheus.NewDesc("cache_server_blocked_clients", "Number of clients currently blocked on a blocking command.", nil, nil),
		connectionsTotal:  prometheus.NewDesc("cache_server_connections_received_total", "Number of connections the Valkey server has accepted since start.", nil, nil),
		rejectedConns:     prometheus.NewDesc("cache_server_rejected_connections_total", "Number of connections the Valkey server rejected because maxclients was reached.", nil, nil),
		commandsProcessed: prometheus.NewDesc("cache_server_commands_processed_total", "Number of commands the Valkey server has processed since start.", nil, nil),
	}
}

func (c *statsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.serverKeys
	ch <- c.memoryUsed
	ch <- c.memoryMax
	ch <- c.evictedKeys
	ch <- c.connectedClients
	ch <- c.blockedClients
	ch <- c.connectionsTotal
	ch <- c.rejectedConns
	ch <- c.commandsProcessed
}

func (c *statsCollector) Collect(ch chan<- prometheus.Metric) {
	client := c.client()
	if client == nil {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), statsTimeout)
	defer cancel()

	size, err := client.Do(ctx, client.B().Dbsize().Build()).AsInt64()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)

		return
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)
	ch <- prometheus.MustNewConstMetric(c.serverKeys, prometheus.GaugeValue, float64(size))

	info, err := client.Do(ctx, client.B().Info().Section("memory", "stats", "clients").Build()).ToString()
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.memoryUsed, fmt.Errorf("read valkey info: %w", err))

		return
	}

	sendInfoMetric(ch, info, c.memoryUsed, prometheus.GaugeValue, "used_memory")
	sendInfoMetric(ch, info, c.memoryMax, prometheus.GaugeValue, "maxmemory")
	sendInfoMetric(ch, info, c.evictedKeys, prometheus.CounterValue, "evicted_keys")
	sendInfoMetric(ch, info, c.connectedClients, prometheus.GaugeValue, "connected_clients")
	sendInfoMetric(ch, info, c.blockedClients, prometheus.GaugeValue, "blocked_clients")
	sendInfoMetric(ch, info, c.connectionsTotal, prometheus.CounterValue, "total_connections_received")
	sendInfoMetric(ch, info, c.rejectedConns, prometheus.CounterValue, "rejected_connections")
	sendInfoMetric(ch, info, c.commandsProcessed, prometheus.CounterValue, "total_commands_processed")
}

func sendInfoMetric(ch chan<- prometheus.Metric, info string, desc *prometheus.Desc, valueType prometheus.ValueType, field string) {
	value, found, err := infoInt(info, field)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(desc, err)

		return
	}

	if !found {
		return
	}

	ch <- prometheus.MustNewConstMetric(desc, valueType, value)
}

func infoInt(info, field string) (float64, bool, error) {
	prefix := field + ":"

	for line := range strings.SplitSeq(info, "\n") {
		line = strings.TrimSpace(line)

		raw, found := strings.CutPrefix(line, prefix)
		if !found {
			continue
		}

		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, false, fmt.Errorf("parse valkey info %s: %w", field, err)
		}

		return value, true, nil
	}

	return 0, false, nil
}
