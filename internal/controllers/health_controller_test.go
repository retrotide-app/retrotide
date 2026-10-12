package controllers

import (
	"errors"
	"net/http"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/health"

	healthgo "github.com/hellofresh/health-go/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	failingCollector struct {
		desc *prometheus.Desc
	}
)

const (
	testMetricsToken = "s3cret"
)

func (f failingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- f.desc
}

func (f failingCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.NewInvalidMetric(f.desc, errors.New("source unreachable"))
}

func newHealthHarness(t *testing.T) (*testutil.Harness, *health.MockService) {
	t.Helper()

	h := testutil.NewHarness(t)
	svc := health.NewMockService(t)
	s := &Service{
		HealthService: svc,
		Guards:        h.Guards,
	}
	RegisterRoutes(h.Engine, s.getAllHealthRoutes())

	return h, svc
}

func TestHealthController_AllChecksPass_Returns200(t *testing.T) {
	// given
	h, svc := newHealthHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return("", nil)
	svc.EXPECT().Measure(mock.Anything).Return(healthgo.Check{Status: healthgo.StatusOK})

	// when
	status, body := h.NewRequest(http.MethodGet, "/health").Do()

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), string(healthgo.StatusOK))
}

func TestHealthController_CriticalCheckDown_Returns503(t *testing.T) {
	// given
	h, svc := newHealthHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return("", nil)
	svc.EXPECT().Measure(mock.Anything).Return(healthgo.Check{
		Status:   healthgo.StatusUnavailable,
		Failures: map[string]string{"postgres": "connection refused"},
	})

	// when
	status, body := h.NewRequest(http.MethodGet, "/health").Do()

	// then
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Contains(t, string(body), "postgres")
}

func TestHealthController_NonCriticalDegraded_Returns200(t *testing.T) {
	// given
	h, svc := newHealthHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return("", nil)
	svc.EXPECT().Measure(mock.Anything).Return(healthgo.Check{
		Status:   healthgo.StatusPartiallyAvailable,
		Failures: map[string]string{"valkey": "timeout"},
	})

	// when
	status, _ := h.NewRequest(http.MethodGet, "/health").Do()

	// then
	assert.Equal(t, http.StatusOK, status)
}

func TestHealthController_Livez_IsShallowAndAlways200(t *testing.T) {
	// given
	h, _ := newHealthHarness(t)

	// when
	status, body := h.NewRequest(http.MethodGet, "/livez").Do()

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"status":"ok"}`, string(body))
}

func TestHealthController_Livez_AnswersHead(t *testing.T) {
	// given
	h, _ := newHealthHarness(t)

	// when
	status, _ := h.NewRequest(http.MethodHead, "/livez").Do()

	// then a probe that only asks for the headers still sees the process is up
	assert.Equal(t, http.StatusOK, status)
}

func TestHealthController_HealthWithoutTheConfiguredToken_Returns404(t *testing.T) {
	// given
	h, _ := newHealthHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return(testMetricsToken, nil)

	// when
	status, body := h.NewRequest(http.MethodGet, "/health").Do()

	// then the checks never run, and the endpoint does not advertise that it exists
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

func TestHealthController_MetricsWithTheToken_ServesTheDefaultRegistry(t *testing.T) {
	// given
	h, _ := newHealthHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return(testMetricsToken, nil)

	// when
	status, body := h.NewRequest(http.MethodGet, "/metrics").WithHeader("Authorization", "Bearer "+testMetricsToken).Do()

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), "go_goroutines")
}

func TestHealthController_MetricsServesTheRestWhenOneCollectorFails(t *testing.T) {
	// given a collector whose source is down, such as valkey timing out on INFO
	failing := failingCollector{desc: prometheus.NewDesc("retrotide_test_failing", "A collector that cannot read its source.", nil, nil)}
	require.NoError(t, prometheus.Register(failing))
	t.Cleanup(func() {
		assert.True(t, prometheus.Unregister(failing))
	})

	h, _ := newHealthHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return(testMetricsToken, nil)

	// when
	status, body := h.NewRequest(http.MethodGet, "/metrics").WithHeader("Authorization", "Bearer "+testMetricsToken).Do()

	// then one failing collector costs only its own series, never the whole scrape, and its error text stays in the log
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), "go_goroutines")
	assert.NotContains(t, string(body), "source unreachable")
}

func TestHealthController_HealthRefusesWhenTheTokenCannotBeRead(t *testing.T) {
	// given
	h, _ := newHealthHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingMetricsToken).Return("", errors.New("db down"))

	// when
	status, body := h.NewRequest(http.MethodGet, "/health").Do()

	// then the checks never run for a caller the guard could not vet
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.JSONEq(t, `{"error":"service unavailable"}`, string(body))
}
