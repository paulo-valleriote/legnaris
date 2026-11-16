package metrics

import (
	"net/http"
	"strconv"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/sweeper"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry                 *prometheus.Registry
	requestDuration          *prometheus.HistogramVec
	requestsTotal            *prometheus.CounterVec
	requestsInFlight         prometheus.Gauge
	authLoginFailures        prometheus.Counter
	authRefreshReuseDetected prometheus.Counter
	authRateLimited          prometheus.Counter
	cspReports               *prometheus.CounterVec
	eventsDropped            *prometheus.CounterVec
	emailOutboxBacklog       prometheus.Gauge
	emailPublishTotal        *prometheus.CounterVec
	emailDeadTotal           prometheus.Counter
	emailSendTotal           *prometheus.CounterVec
	emailSendDuration        prometheus.Histogram
	emailExpiredTotal        prometheus.Counter
	emailBreakerOpen         prometheus.Gauge
	emailBreakerOpened       prometheus.Counter
	authOTPIssued            *prometheus.CounterVec
	authOTPVerify            *prometheus.CounterVec
	authOTPResend            *prometheus.CounterVec
	authOTPCapBlocked        prometheus.Counter
	sweeperOldestPending     *prometheus.GaugeVec
	sweeperFailed            *prometheus.CounterVec
	generationCapUsed        prometheus.Gauge
	generationCapLimit       prometheus.Gauge
	generationCapUsedRatio   prometheus.Gauge
}

const metricsNamespace = "legnaris"

const sweeperSubsystem = "sweeper"

const queueLabel = "queue"

var requestDurationBuckets = []float64{
	0.0005, 0.001, 0.002, 0.003, 0.005, 0.0075, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5,
}

const httpSubsystem = "http"

const authSubsystem = "auth"

const resultLabel = "result"

func New() *Metrics {
	registry := prometheus.NewRegistry()

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: httpSubsystem,
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   requestDurationBuckets,
		},
		[]string{"method", "path", "status"},
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: httpSubsystem,
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

	eventsDropped := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "events_dropped_total",
		Help:      "Total number of domain events dropped by the best-effort publisher.",
	}, []string{"type", "reason"})

	emailOutboxBacklog := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "email_outbox_backlog",
		Help:      "Pending email send intentions awaiting publication.",
	})

	emailPublishTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "email_publish_total",
		Help:      "Email publication outcomes by result (published, retried, failed).",
	}, []string{resultLabel})

	emailDeadTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "email_dead_total",
		Help:      "Email messages dead-lettered for operator attention.",
	})

	emailSendTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "email_send_total",
		Help:      "Email delivery outcomes by result (sent, retried, dead_lettered, expired).",
	}, []string{resultLabel})

	emailSendDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Name:      "email_send_duration_seconds",
		Help:      "Provider send latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	})

	emailExpiredTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "email_expired_total",
		Help:      "Email messages dropped because their code had expired.",
	})

	emailBreakerOpen := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "email_breaker_open",
		Help:      "1 while the email provider breaker is open and consumption is paused, else 0.",
	})

	emailBreakerOpened := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "email_breaker_opened_total",
		Help:      "Times the email provider breaker opened after consecutive outages.",
	})

	authOTPIssued := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: authSubsystem,
		Name:      "otp_issued_total",
		Help:      "Challenges issued by purpose (each recorded one send intention).",
	}, []string{"purpose"})

	authOTPVerify := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: authSubsystem,
		Name:      "otp_verify_total",
		Help:      "Verification outcomes by result (the internal classification of the uniform failure).",
	}, []string{resultLabel})

	authOTPResend := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: authSubsystem,
		Name:      "otp_resend_total",
		Help:      "Resend outcomes by result (sent, suppressed, capped).",
	}, []string{resultLabel})

	authOTPCapBlocked := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: authSubsystem,
		Name:      "otp_cap_blocked_total",
		Help:      "Requests refused because the daily email send cap was reached.",
	})

	requestsInFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: httpSubsystem,
		Name:      "in_flight_requests",
		Help:      "Number of HTTP requests currently being served.",
	})

	sweeperOldestPending := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: sweeperSubsystem,
		Name:      "oldest_pending_seconds",
		Help:      "Age in seconds of the oldest pending item of a sweeper queue; 0 when none is pending.",
	}, []string{queueLabel})

	sweeperFailed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: sweeperSubsystem,
		Name:      "failed_total",
		Help:      "Total number of failed handler attempts of a sweeper queue.",
	}, []string{queueLabel})

	generationCapUsed := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "generation_cap_used",
		Help:      "Generation jobs accepted today (UTC) against the global daily cap, as last observed.",
	})
	generationCapLimit := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "generation_cap_limit",
		Help:      "Global daily cap of generation jobs (GENERATION_DAILY_CAP).",
	})
	generationCapUsedRatio := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "generation_cap_used_ratio",
		Help:      "Share of the global daily generation cap in use, from 0 to 1; the 80% alert fires above 0.8.",
	})

	registry.MustRegister(requestDuration, requestsTotal, requestsInFlight, authLoginFailures, authRefreshReuseDetected, authRateLimited, cspReports, eventsDropped,
		emailOutboxBacklog, emailPublishTotal, emailDeadTotal,
		emailSendTotal, emailSendDuration, emailExpiredTotal,
		emailBreakerOpen, emailBreakerOpened,
		authOTPIssued, authOTPVerify, authOTPResend, authOTPCapBlocked,
		sweeperOldestPending, sweeperFailed,
		generationCapUsed, generationCapLimit, generationCapUsedRatio)

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return &Metrics{
		registry:                 registry,
		requestDuration:          requestDuration,
		requestsTotal:            requestsTotal,
		requestsInFlight:         requestsInFlight,
		authLoginFailures:        authLoginFailures,
		authRefreshReuseDetected: authRefreshReuseDetected,
		authRateLimited:          authRateLimited,
		cspReports:               cspReports,
		eventsDropped:            eventsDropped,
		emailOutboxBacklog:       emailOutboxBacklog,
		emailPublishTotal:        emailPublishTotal,
		emailDeadTotal:           emailDeadTotal,
		emailSendTotal:           emailSendTotal,
		emailSendDuration:        emailSendDuration,
		emailExpiredTotal:        emailExpiredTotal,
		emailBreakerOpen:         emailBreakerOpen,
		emailBreakerOpened:       emailBreakerOpened,
		authOTPIssued:            authOTPIssued,
		authOTPVerify:            authOTPVerify,
		authOTPResend:            authOTPResend,
		authOTPCapBlocked:        authOTPCapBlocked,
		sweeperOldestPending:     sweeperOldestPending,
		sweeperFailed:            sweeperFailed,
		generationCapUsed:        generationCapUsed,
		generationCapLimit:       generationCapLimit,
		generationCapUsedRatio:   generationCapUsedRatio,
	}
}

var _ auth.ChallengeObserver = (*Metrics)(nil)

func (m *Metrics) LoginFailure() {
	m.authLoginFailures.Inc()
}

func (m *Metrics) RefreshReuseDetected() {
	m.authRefreshReuseDetected.Inc()
}

func (m *Metrics) RateLimitBlocked() {
	m.authRateLimited.Inc()
}

func (m *Metrics) CSPReportReported(effectiveDirective, disposition string) {
	m.cspReports.WithLabelValues(effectiveDirective, disposition).Inc()
}

func (m *Metrics) EventDropped(eventType, reason string) {
	m.eventsDropped.WithLabelValues(eventType, reason).Inc()
}

func (m *Metrics) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		start := time.Now()
		m.requestsInFlight.Inc()
		defer m.requestsInFlight.Dec()
		err := next(c)

		method := c.Request().Method
		path := c.Path()
		if path == "" {
			path = "unmatched"
		}
		_, statusCode := echo.ResolveResponseStatus(c.Response(), err)
		status := strconv.Itoa(statusCode)

		m.requestsTotal.WithLabelValues(method, path, status).Inc()
		m.requestDuration.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())

		return err
	}
}

func (m *Metrics) Register(e *echo.Echo, routeMiddleware ...echo.MiddlewareFunc) {
	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})), routeMiddleware...)
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) PublishResult(result string) {
	m.emailPublishTotal.WithLabelValues(result).Inc()
}

func (m *Metrics) BacklogPending(pending int) {
	m.emailOutboxBacklog.Set(float64(pending))
}

func (m *Metrics) MessageDead() {
	m.emailDeadTotal.Inc()
}

func (m *Metrics) SendResult(result string) {
	m.emailSendTotal.WithLabelValues(result).Inc()
}

func (m *Metrics) SendDuration(d time.Duration) {
	m.emailSendDuration.Observe(d.Seconds())
}

func (m *Metrics) MessageExpired() {
	m.emailExpiredTotal.Inc()
}

func (m *Metrics) BreakerOpened() {
	m.emailBreakerOpen.Set(1)
	m.emailBreakerOpened.Inc()
}

func (m *Metrics) BreakerClosed() {
	m.emailBreakerOpen.Set(0)
}

func (m *Metrics) ChallengeIssued(purpose auth.Purpose) {
	m.authOTPIssued.WithLabelValues(string(purpose)).Inc()
}

func (m *Metrics) ChallengeVerified(result string) {
	m.authOTPVerify.WithLabelValues(result).Inc()
}

func (m *Metrics) ResendRequested(result string) {
	m.authOTPResend.WithLabelValues(result).Inc()
}

func (m *Metrics) SendCapBlocked() {
	m.authOTPCapBlocked.Inc()
}

var _ sweeper.Observer = (*Metrics)(nil)

func (m *Metrics) OldestPending(queue string, age time.Duration) {
	m.sweeperOldestPending.WithLabelValues(queue).Set(age.Seconds())
}

func (m *Metrics) Failed(queue string) {
	m.sweeperFailed.WithLabelValues(queue).Inc()
}

func (m *Metrics) GenerationCapUsage(used, limit int) {
	m.generationCapUsed.Set(float64(used))
	m.generationCapLimit.Set(float64(limit))
	ratio := 0.0
	if limit > 0 {
		ratio = float64(used) / float64(limit)
	}
	m.generationCapUsedRatio.Set(ratio)
}
