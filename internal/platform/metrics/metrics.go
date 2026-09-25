// Package metrics exposes Prometheus metrics for the HTTP API.
package metrics

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "legnaris",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "legnaris",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)

func init() {
	prometheus.MustRegister(requestDuration, requestsTotal)
}

// Middleware records request count and latency per route and status code.
func Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		start := time.Now()
		err := next(c)

		method := c.Request().Method
		path := c.Path()
		_, statusCode := echo.ResolveResponseStatus(c.Response(), err)
		status := strconv.Itoa(statusCode)

		requestsTotal.WithLabelValues(method, path, status).Inc()
		requestDuration.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())

		return err
	}
}

// Register exposes the /metrics endpoint backed by the default registry.
func Register(e *echo.Echo) {
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
}
