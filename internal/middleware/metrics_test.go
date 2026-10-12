package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	clientmodel "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func observedRequests(t *testing.T, method string, route string, status string) uint64 {
	t.Helper()

	observer, err := httpRequestDuration.GetMetricWithLabelValues(method, route, status)
	require.NoError(t, err)

	metric, ok := observer.(prometheus.Metric)
	require.True(t, ok)

	var sample clientmodel.Metric
	require.NoError(t, metric.Write(&sample))

	return sample.GetHistogram().GetSampleCount()
}

func metricsEngine(t *testing.T) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(Metrics())
	engine.GET("/metrics-test/items/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "item")
	})
	engine.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "go_goroutines 12")
	})
	engine.GET("/metrics-test/panic", func(c *gin.Context) {
		panic("boom")
	})

	return engine
}

func TestMetrics_ObservesAMatchedRouteByItsTemplate(t *testing.T) {
	// given
	engine := metricsEngine(t)
	before := observedRequests(t, http.MethodGet, "/metrics-test/items/:id", "200")

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics-test/items/42", nil))

	// then
	assert.Equal(t, before+1, observedRequests(t, http.MethodGet, "/metrics-test/items/:id", "200"))
}

func TestMetrics_LabelsAnUnmatchedPathOther(t *testing.T) {
	// given
	engine := metricsEngine(t)
	before := observedRequests(t, http.MethodGet, otherRoute, "404")

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/b/secret-slug", nil))

	// then
	assert.Equal(t, before+1, observedRequests(t, http.MethodGet, otherRoute, "404"))
}

func TestMetrics_FoldsUnknownMethodsIntoOneSeries(t *testing.T) {
	// given
	engine := metricsEngine(t)
	serve(engine, httptest.NewRequestWithContext(t.Context(), "ZZ0", "/metrics-test/nowhere", nil))
	seriesBefore := testutil.CollectAndCount(httpRequestDuration)
	observedBefore := observedRequests(t, otherMethod, otherRoute, "404")

	// when
	for _, method := range []string{"ZZ1", "ZZ2", "ZZ3", "ZZ4"} {
		serve(engine, httptest.NewRequestWithContext(t.Context(), method, "/metrics-test/nowhere", nil))
	}

	// then an invented method never mints a series of its own
	assert.Equal(t, seriesBefore, testutil.CollectAndCount(httpRequestDuration))
	assert.Equal(t, observedBefore+4, observedRequests(t, otherMethod, otherRoute, "404"))
}

func TestMetrics_LeavesTheScrapeOut(t *testing.T) {
	// given
	engine := metricsEngine(t)
	before := observedRequests(t, http.MethodGet, "/metrics", "200")

	// when
	serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil))

	// then
	assert.Equal(t, before, observedRequests(t, http.MethodGet, "/metrics", "200"))
}

func TestMetrics_ReleasesTheInFlightGaugeAfterAPanic(t *testing.T) {
	// given
	engine := metricsEngine(t)
	before := testutil.ToFloat64(httpRequestsInFlight)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics-test/panic", nil))

	// then
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.InDelta(t, before, testutil.ToFloat64(httpRequestsInFlight), 0)
}
