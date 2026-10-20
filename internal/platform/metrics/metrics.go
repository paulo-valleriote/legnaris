package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics bundles the HTTP request collectors with a dedicated registry. Each
// instance owns its own *prometheus.Registry so packages never write into (or
// collide on) the process-global default registry.
type Metrics struct {
	registry                 *prometheus.Registry
	requestDuration          *prometheus.HistogramVec
	requestsTotal            *prometheus.CounterVec
	authLoginFailures        prometheus.Counter
	authRefreshReuseDetected prometheus.Counter
	authRateLimited          prometheus.Counter
	cspReports               *prometheus.CounterVec
	emailOutboxBacklog       prometheus.Gauge
	emailPublishTotal        *prometheus.CounterVec
	emailDeadTotal           prometheus.Counter
}

const metricsNamespace = "legnaris"

// authSubsystem is the Prometheus subsystem shared by the auth observers.
const authSubsystem = "auth"

// New builds a Metrics instance with its own registry and collectors.
func New() *Metrics {
	registry := prometheus.NewRegistry()

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	authLoginFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: authSubsystem,
		Name:      "login_failures_total",
		Help:      "Total number of failed login attempts.",
	})

	authRefreshReuseDetected := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: authSubsystem,
		Name:      "refresh_reuse_detected_total",
		Help:      "Total number of refresh token reuse detections (possible theft).",
	})

	authRateLimited := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: authSubsystem,
		Name:      "rate_limited_total",
		Help:      "Total number of auth requests refused by the rate limiter.",
	})

	cspReports := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "csp_reports_total",
		Help:      "Total number of collected CSP violation reports.",
	}, []string{"effective_directive", "disposition"})

	// Email-delivery series consumed by the relay: the pending backlog gauge,
	// the publication outcomes by result, and a dead-letter counter reserved
	// for the worker (EML-23).
	emailOutboxBacklog := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "email_outbox_backlog",
		Help:      "Pending email send intentions awaiting publication.",
	})

	emailPublishTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "email_publish_total",
		Help:      "Email publication outcomes by result (published, retried, failed).",
	}, []string{"result"})

	emailDeadTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "email_dead_total",
		Help:      "Email messages dead-lettered for operator attention.",
	})

	registry.MustRegister(requestDuration, requestsTotal, authLoginFailures, authRefreshReuseDetected, authRateLimited, cspReports,
		emailOutboxBacklog, emailPublishTotal, emailDeadTotal)

	// The private registry must be self-sufficient: register the standard Go
	// and process collectors so the deployment's own resource metrics (goroutines,
	// heap, CPU, RSS) are exposed alongside the HTTP series.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return &Metrics{
		registry:                 registry,
		requestDuration:          requestDuration,
		requestsTotal:            requestsTotal,
		authLoginFailures:        authLoginFailures,
		authRefreshReuseDetected: authRefreshReuseDetected,
		authRateLimited:          authRateLimited,
		cspReports:               cspReports,
		emailOutboxBacklog:       emailOutboxBacklog,
		emailPublishTotal:        emailPublishTotal,
		emailDeadTotal:           emailDeadTotal,
	}
}

// LoginFailure implements auth.Observer: it counts a rejected credential check.
func (m *Metrics) LoginFailure() {
	m.authLoginFailures.Inc()
}

// RefreshReuseDetected implements auth.Observer: it counts a rotated refresh
// token being presented again, which revokes the whole family.
func (m *Metrics) RefreshReuseDetected() {
	m.authRefreshReuseDetected.Inc()
}

// RateLimitBlocked implements app.Observer: it counts a request refused by the
// auth rate limiter (AUTH-20).
func (m *Metrics) RateLimitBlocked() {
	m.authRateLimited.Inc()
}

// CSPReportReported implements app.CSPReportObserver: it counts a collected CSP
// violation report, labelled by the violated directive and the disposition
// (POL-05, A6).
func (m *Metrics) CSPReportReported(effectiveDirective, disposition string) {
	m.cspReports.WithLabelValues(effectiveDirective, disposition).Inc()
}

// Middleware records request count and latency per route and status code.
//
// The status label is resolved from the committed response after next(c)
// returns. The app registers the echo RequestLogger (HandleError=true) *inner*
// to this middleware, so the global HTTPErrorHandler commits the final 4xx/5xx
// before this middleware resumes and records it — keep this middleware outer
// to any error-handling middleware, otherwise a status is read before the
// error handler maps it (a bare `errors.New` would fall back to 500).
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

// Handler exposes this instance's registry as a plain http.Handler, so the
// relay — a separate process with no Echo server — can serve its own metrics
// listener from the same namespace (EML-23).
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// PublishResult implements the relay Observer: it counts one publication
// outcome by result ("published", "retried", "failed") (EML-23).
func (m *Metrics) PublishResult(result string) {
	m.emailPublishTotal.WithLabelValues(result).Inc()
}

// BacklogPending implements the relay Observer: it sets the pending-backlog
// gauge so a stalled pipeline can be alerted on (EML-23).
func (m *Metrics) BacklogPending(pending int) {
	m.emailOutboxBacklog.Set(float64(pending))
}

// MessageDead counts a message dead-lettered for operator attention; it is
// reserved for the worker that will dead-letter a poison message (EML-23).
func (m *Metrics) MessageDead() {
	m.emailDeadTotal.Inc()
}
