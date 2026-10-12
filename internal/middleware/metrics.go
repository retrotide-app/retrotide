package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds by method and route.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status"},
	)
	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served.",
		},
	)

	unmeasuredRoutes = map[string]struct{}{
		metricsRoute: {},
		assetsRoute:  {},
		socketRoute:  {},
	}
)

func init() {
	prometheus.MustRegister(httpRequestDuration, httpRequestsInFlight)
}

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		route := routeLabel(c)
		if _, unmeasured := unmeasuredRoutes[route]; unmeasured || strings.HasPrefix(route, pprofRoutePrefix) {
			return
		}

		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		c.Next()

		httpRequestDuration.
			WithLabelValues(methodLabel(c), route, strconv.Itoa(c.Writer.Status())).
			Observe(time.Since(start).Seconds())
	}
}
