package ws

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	unknownMessageType = "unknown"

	killSlowConsumer   = "slow_consumer"
	killPongTimeout    = "pong_timeout"
	killRevalidation   = "revalidation_failed"
	killDisconnected   = "disconnected"
	killShutdownExpiry = "shutdown_timeout"
)

var (
	wsInboundMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_inbound_messages_total",
			Help: "Number of WebSocket frames received from clients by message type.",
		},
		[]string{"type"},
	)
	wsInboundDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_inbound_dropped_total",
			Help: "Number of inbound WebSocket frames dropped by the per-socket rate limiter, by principal kind.",
		},
		[]string{"kind"},
	)
	wsConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ws_connections",
			Help: "Number of WebSocket connections currently open, by principal kind.",
		},
		[]string{"kind"},
	)
	wsConnectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_connections_total",
			Help: "Number of WebSocket connections opened, by principal kind.",
		},
		[]string{"kind"},
	)
	wsInboundTokens = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ws_inbound_limiter_tokens",
			Help:    "Rate limiter tokens remaining when an inbound frame was received.",
			Buckets: []float64{0, 1, 2, 5, 10, 20, 30, 40},
		},
	)
	wsOutboundFramesDropped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ws_outbound_frames_dropped_total",
			Help: "Number of lossy outbound frames, such as presence, skipped because a client send buffer was past its watermark.",
		},
	)
	wsClientsKilled = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_clients_killed_total",
			Help: "Number of WebSocket clients the server dropped, by reason.",
		},
		[]string{"reason"},
	)
	wsPingRTT = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ws_ping_rtt_seconds",
			Help:    "Round trip of the server's control-frame pings, measured around Ping.",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
	)
	wsClientRTT = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ws_client_reported_rtt_seconds",
			Help:    "Round trip clients report in their app-level ping, capped by the server's own measurement.",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
	)

	inboundTypes = map[string]struct{}{
		TypePing:             {},
		TypeBoardJoin:        {},
		TypeBoardLeave:       {},
		TypeBoardViewerState: {},
	}
)

func init() {
	prometheus.MustRegister(wsInboundMessages, wsInboundDropped, wsConnections, wsConnectionsTotal, wsInboundTokens, wsOutboundFramesDropped, wsClientsKilled, wsPingRTT, wsClientRTT)
}

func inboundLabel(msgType string) string {
	if _, known := inboundTypes[msgType]; known {
		return msgType
	}

	return unknownMessageType
}

func recordInbound(label string, tokens float64) {
	wsInboundMessages.WithLabelValues(label).Inc()
	wsInboundTokens.Observe(tokens)
}

func recordConnectionOpened(kind string) {
	wsConnections.WithLabelValues(kind).Inc()
	wsConnectionsTotal.WithLabelValues(kind).Inc()
}

func recordClientRTT(rttMS int) {
	if rttMS <= 0 {
		return
	}

	wsClientRTT.Observe((time.Duration(rttMS) * time.Millisecond).Seconds())
}
