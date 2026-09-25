package metrics

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics bundles the HTTP request collectors with a dedicated registry. Each
// instance owns its own *prometheus.Registry so packages never write into (or
// collide on) the process-global default registry.
type Metrics struct {
	registry        *prometheus.Registry
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
}

// New builds a Metrics instance with its own registry and collectors.
func New() *Metrics {
	registry := prometheus.NewRegistry()

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "legnaris",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "legnaris",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	registry.MustRegister(requestDuration, requestsTotal)

	return &Metrics{
		registry:        registry,
		requestDuration: requestDuration,
		requestsTotal:   requestsTotal,
	}
}

// Middleware records request count and latency per route and status code.
func (m *Metrics) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		start := time.Now()
		err := next(c)

		method := c.Request().Method
		path := c.Path()
		if path == "" {
			// Unmatched routes report an empty template; collapse them into a
			// single label instead of an opaque empty series.
			path = "unmatched"
		}
		_, statusCode := echo.ResolveResponseStatus(c.Response(), err)
		status := strconv.Itoa(statusCode)

		m.requestsTotal.WithLabelValues(method, path, status).Inc()
		m.requestDuration.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())

		return err
	}
}

// Register exposes the /metrics endpoint backed by this instance's registry.
func (m *Metrics) Register(e *echo.Echo) {
	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})))
}
