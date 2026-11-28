package metrics

import (
	"net/http"
	"strconv"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/funnel"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llm"
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
	genAIOperationDuration   *prometheus.HistogramVec
	genAITokenUsage          *prometheus.HistogramVec
	genAICost                *prometheus.CounterVec
	genAICostUnknown         *prometheus.CounterVec
	genAIInvalidOutput       *prometheus.CounterVec
	genAISchemaRetry         *prometheus.CounterVec
	genAIBreakerOpen         prometheus.Gauge
	genAIBreakerOpened       prometheus.Counter
	billingCorrections       *prometheus.CounterVec
	funnelHomeVisits         prometheus.Counter
	funnelHomeCTAClicks      prometheus.Counter
	funnelAccounts           prometheus.Gauge
	funnelFirstSheet24h      prometheus.Gauge
	funnelFirstSheet24hRatio prometheus.Gauge
}

var _ funnel.Observer = (*Metrics)(nil)

const metricsNamespace = "legnaris"

const sweeperSubsystem = "sweeper"

const queueLabel = "queue"

const billingSubsystem = "billing"

const funnelSubsystem = "funnel"

const genAISubsystem = "gen_ai"

const (
	genAIModelLabel         = "gen_ai_response_model"
	genAITokenTypeLabel     = "gen_ai_token_type"
	genAIPromptVersionLabel = "prompt_version"
	genAIModelUnknown       = "unknown"
	genAITokenInput         = "input"
	genAITokenOutput        = "output"
)

var genAIDurationBuckets = []float64{
	0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60, 120,
}

var genAITokenBuckets = []float64{
	1, 10, 50, 100, 250, 500, 1000, 2000, 4000, 8000, 16000, 32000,
}

const kindLabel = "kind"

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

	genAIOperationDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "client_operation_duration_seconds",
		Help:      "LLM provider call latency in seconds, by model, prompt version and result.",
		Buckets:   genAIDurationBuckets,
	}, []string{genAIModelLabel, genAIPromptVersionLabel, resultLabel})

	genAITokenUsage := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "client_token_usage",
		Help:      "Tokens consumed per LLM call, by model, prompt version and token type (input or output).",
		Buckets:   genAITokenBuckets,
	}, []string{genAIModelLabel, genAIPromptVersionLabel, genAITokenTypeLabel})

	genAICost := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "cost_usd_total",
		Help:      "Provider-reported cost in US dollars, by model and prompt version.",
	}, []string{genAIModelLabel, genAIPromptVersionLabel})

	genAICostUnknown := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "cost_unknown_total",
		Help:      "LLM calls whose provider response carried no cost, by model and prompt version; never counted as zero.",
	}, []string{genAIModelLabel, genAIPromptVersionLabel})

	genAIInvalidOutput := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "invalid_output_total",
		Help:      "LLM outputs rejected by schema or domain validation, by model and prompt version.",
	}, []string{genAIModelLabel, genAIPromptVersionLabel})

	genAISchemaRetry := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "schema_retry_total",
		Help:      "Schema retries scheduled after an invalid output, by model and prompt version.",
	}, []string{genAIModelLabel, genAIPromptVersionLabel})

	genAIBreakerOpen := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "breaker_open",
		Help:      "1 while the generation provider breaker is open, else 0.",
	})

	genAIBreakerOpened := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: genAISubsystem,
		Name:      "breaker_opened_total",
		Help:      "Times the generation provider breaker opened after consecutive outages.",
	})

	billingCorrections := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: billingSubsystem,
		Name:      "reconcile_corrections_total",
		Help:      "Drift corrected by the daily billing reconciliation, by kind (subscription, plan, event).",
	}, []string{kindLabel})

	funnelHomeVisits := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: funnelSubsystem,
		Name:      "home_visits_total",
		Help:      "Aggregated visits to the public landing page, counted without cookies or identifiers.",
	})

	funnelHomeCTAClicks := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: funnelSubsystem,
		Name:      "home_cta_clicks_total",
		Help:      "Aggregated clicks on the landing hero call to action, counted without cookies or identifiers.",
	})

	funnelAccounts := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: funnelSubsystem,
		Name:      "accounts",
		Help:      "Accounts present in the database, the conversion baseline of the home funnel.",
	})

	funnelFirstSheetWithin24h := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: funnelSubsystem,
		Name:      "first_sheet_within_24h",
		Help:      "Accounts whose earliest Sheet was saved within 24 hours of account creation.",
	})

	funnelFirstSheetRatio := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: funnelSubsystem,
		Name:      "first_sheet_within_24h_ratio",
		Help:      "Share of accounts whose earliest Sheet was saved within 24 hours, from 0 to 1.",
	})

	registry.MustRegister(requestDuration, requestsTotal, requestsInFlight, authLoginFailures, authRefreshReuseDetected, authRateLimited, cspReports, eventsDropped,
		emailOutboxBacklog, emailPublishTotal, emailDeadTotal,
		emailSendTotal, emailSendDuration, emailExpiredTotal,
		emailBreakerOpen, emailBreakerOpened,
		authOTPIssued, authOTPVerify, authOTPResend, authOTPCapBlocked,
		sweeperOldestPending, sweeperFailed,
		generationCapUsed, generationCapLimit, generationCapUsedRatio,
		genAIOperationDuration, genAITokenUsage, genAICost, genAICostUnknown,
		genAIInvalidOutput, genAISchemaRetry, genAIBreakerOpen, genAIBreakerOpened,
		billingCorrections,
		funnelHomeVisits, funnelHomeCTAClicks, funnelAccounts, funnelFirstSheetWithin24h, funnelFirstSheetRatio)

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
		genAIOperationDuration:   genAIOperationDuration,
		genAITokenUsage:          genAITokenUsage,
		genAICost:                genAICost,
		genAICostUnknown:         genAICostUnknown,
		genAIInvalidOutput:       genAIInvalidOutput,
		genAISchemaRetry:         genAISchemaRetry,
		genAIBreakerOpen:         genAIBreakerOpen,
		genAIBreakerOpened:       genAIBreakerOpened,
		billingCorrections:       billingCorrections,
		funnelHomeVisits:         funnelHomeVisits,
		funnelHomeCTAClicks:      funnelHomeCTAClicks,
		funnelAccounts:           funnelAccounts,
		funnelFirstSheet24h:      funnelFirstSheetWithin24h,
		funnelFirstSheet24hRatio: funnelFirstSheetRatio,
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

var _ billing.ReconcileObserver = (*Metrics)(nil)

func (m *Metrics) Corrected(kind string, count int) {
	if count <= 0 {
		return
	}
	m.billingCorrections.WithLabelValues(kind).Add(float64(count))
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

var _ generation.WorkerObserver = (*Metrics)(nil)

func (m *Metrics) GenerationAttempt(model, promptVersion string, duration time.Duration, usage llm.Usage, result string) {
	model = genAIModel(model)
	if result != generation.AttemptResultSuccess {
		result = generation.AttemptResultError
	}
	m.genAIOperationDuration.WithLabelValues(model, promptVersion, result).Observe(duration.Seconds())
	if result != generation.AttemptResultSuccess {
		return
	}
	m.genAITokenUsage.WithLabelValues(model, promptVersion, genAITokenInput).Observe(float64(max(usage.PromptTokens, 0)))
	m.genAITokenUsage.WithLabelValues(model, promptVersion, genAITokenOutput).Observe(float64(max(usage.CompletionTokens, 0)))
	if usage.CostUSD == nil || *usage.CostUSD < 0 {
		m.genAICostUnknown.WithLabelValues(model, promptVersion).Inc()
		return
	}
	m.genAICost.WithLabelValues(model, promptVersion).Add(*usage.CostUSD)
}

func (m *Metrics) GenerationInvalidOutput(model, promptVersion string) {
	m.genAIInvalidOutput.WithLabelValues(genAIModel(model), promptVersion).Inc()
}

func (m *Metrics) GenerationSchemaRetry(model, promptVersion string) {
	m.genAISchemaRetry.WithLabelValues(genAIModel(model), promptVersion).Inc()
}

func (m *Metrics) GenerationBreakerState(open bool) {
	if open {
		m.genAIBreakerOpen.Set(1)
		return
	}
	m.genAIBreakerOpen.Set(0)
}

func (m *Metrics) GenerationBreakerOpened() {
	m.genAIBreakerOpened.Inc()
}

func genAIModel(model string) string {
	if model == "" {
		return genAIModelUnknown
	}
	return model
}

func (m *Metrics) HomeVisited() {
	m.funnelHomeVisits.Inc()
}

func (m *Metrics) HomeCTAClicked() {
	m.funnelHomeCTAClicks.Inc()
}

func (m *Metrics) FunnelStats(stats funnel.Stats) {
	accounts := max(stats.Accounts, 0)
	within := max(stats.FirstSheetWithin24h, 0)
	ratio := 0.0
	if accounts > 0 {
		ratio = min(float64(within)/float64(accounts), 1)
	}

	m.funnelAccounts.Set(float64(accounts))
	m.funnelFirstSheet24h.Set(float64(within))
	m.funnelFirstSheet24hRatio.Set(ratio)
}
