package controllers

import (
	"log"
	"net/http"

	"retrotide/internal/logger"

	"github.com/gin-gonic/gin"
	healthgo "github.com/hellofresh/health-go/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

func (s *Service) getAllHealthRoutes() []FRoute {
	metrics := promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
		ErrorLog:      log.New(logger.NewLevelWriter(zerolog.ErrorLevel), "", 0),
	}))

	return []FRoute{
		{Method: http.MethodGet, Path: "/livez", Handler: func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		}},
		{Method: http.MethodGet, Path: "/health", Before: gin.HandlersChain{s.Guards.MetricsToken}, Handler: s.health},
		{Method: http.MethodGet, Path: "/metrics", Before: gin.HandlersChain{s.Guards.MetricsToken}, Handler: gin.WrapH(metrics)},
	}
}

func (s *Service) health(c *gin.Context) {
	check := s.HealthService.Measure(c.Request.Context())

	status := http.StatusOK
	if check.Status == healthgo.StatusUnavailable {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, check)
}
