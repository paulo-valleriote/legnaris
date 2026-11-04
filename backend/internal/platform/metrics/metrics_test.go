package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/auth"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRouter builds an Echo instance with a Metrics instance registered
// (exposing /metrics) and its middleware applied, plus a single /test route —
// mirroring how backend/internal/app/router.go wires the middleware.
func newTestRouter(t *testing.T) *echo.Echo {
	t.Helper()
	e := echo.New()
	m := New()
	m.Register(e)
	e.Use(m.Middleware)
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	return e
}

// scrapeMetrics GETs /metrics, asserts a 200, and returns the response body.
func scrapeMetrics(t *testing.T, e *echo.Echo) string {
	t.Helper()
	mreq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, mreq)
	if mrec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from /metrics, got %d", mrec.Code)
	}
	return mrec.Body.String()
}

func TestMiddlewareRecordsMetrics(t *testing.T) {
	e := newTestRouter(t)

	// Exercise the middleware so the metrics gain at least one observation.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := scrapeMetrics(t, e)
	for _, want := range []string{
		"legnaris_http_request_duration_seconds_bucket",
		"legnaris_http_requests_total",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in /metrics output", want)
		}
	}
}

func TestMiddlewareNormalizesUnmatchedPath(t *testing.T) {
	e := newTestRouter(t)

	// Request an unmatched route so the path label falls back to "unmatched".
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	if body := scrapeMetrics(t, e); !strings.Contains(body, `path="unmatched"`) {
		t.Errorf("expected an unmatched-path series in /metrics output, got:\n%s", body)
	}
}

// TestNewRegistersRuntimeCollectors asserts the private registry also exposes
// the standard Go/process collectors, so the deployment's own resource metrics
// are present without depending on the process-global default registry.
func TestNewRegistersRuntimeCollectors(t *testing.T) {
	e := newTestRouter(t)

	body := scrapeMetrics(t, e)
	for _, want := range []string{
		"go_goroutines",
		"process_cpu_seconds_total",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in /metrics output", want)
		}
	}
}

// TestMiddlewareRecordsStatusAfterErrorHandling reproduces the app's real
// middleware order (backend/internal/app/router.go): an error-handling middleware runs
// *inner* to the metrics middleware and invokes the HTTPErrorHandler, which
// commits the mapped status before the metrics middleware resumes. The status
// label must reflect the handler's real status (404 here), not the
// pre-handler 200 or a bare 500 — this guards the ordering the label depends on.
func TestMiddlewareRecordsStatusAfterErrorHandling(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		_ = c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	m := New()
	m.Register(e)
	e.Use(m.Middleware)
	// Mirror the RequestLogger (HandleError=true) that commits the error
	// response before the metrics middleware records the status.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			err := next(c)
			if err != nil {
				c.Echo().HTTPErrorHandler(c, err)
			}
			return err
		}
	})
	e.GET("/missing", func(c *echo.Context) error {
		return errors.New("sheet not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 from handler, got %d", rec.Code)
	}

	body := scrapeMetrics(t, e)
	want := `legnaris_http_requests_total{method="GET",path="/missing",status="404"}`
	if !strings.Contains(body, want) {
		t.Errorf("expected %q in /metrics output, got:\n%s", want, body)
	}
}

// TestRateLimitBlockedIncrementsCounter pins the counter the rate-limit guard
// increments on every refused request (AUTH-20).
func TestRateLimitBlockedIncrementsCounter(t *testing.T) {
	e := echo.New()
	m := New()
	m.Register(e)

	m.RateLimitBlocked()
	m.RateLimitBlocked()

	body := scrapeMetrics(t, e)
	if !strings.Contains(body, "legnaris_auth_rate_limited_total 2") {
		t.Errorf("expected the rate-limited counter at 2 in /metrics output, got:\n%s", body)
	}
}

// TestCSPReportReportedIncrementsCounter pins the CSP violation counter and its
// labels (POL-05, A6).
func TestCSPReportReportedIncrementsCounter(t *testing.T) {
	e := echo.New()
	m := New()
	m.Register(e)

	m.CSPReportReported("script-src", "enforce")
	m.CSPReportReported("script-src", "enforce")
	m.CSPReportReported("style-src", "report")

	body := scrapeMetrics(t, e)
	for _, want := range []string{
		`legnaris_csp_reports_total{disposition="enforce",effective_directive="script-src"} 2`,
		`legnaris_csp_reports_total{disposition="report",effective_directive="style-src"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in /metrics output, got:\n%s", want, body)
		}
	}
}

// TestEmailMetrics pins the relay-facing series: the pending-backlog gauge, the
// publication outcomes by result, and the dead-letter counter, all served by the
// standalone Handler for a process with no Echo server (EML-23).
func TestEmailMetrics(t *testing.T) {
	m := New()

	m.BacklogPending(3)
	m.PublishResult("published")
	m.PublishResult("published")
	m.PublishResult("failed")
	m.MessageDead()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from the metrics handler, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"legnaris_email_outbox_backlog 3",
		`legnaris_email_publish_total{result="published"} 2`,
		`legnaris_email_publish_total{result="failed"} 1`,
		"legnaris_email_dead_total 1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in the metrics output, got:\n%s", want, body)
		}
	}
}

// TestOTPMetrics pins the code-step series and their exact label sets, and
// proves the type satisfies the challenge observer without any series carrying
// a code, a challenge identity or an unmasked address (OTP-15, OTP-16).
func TestOTPMetrics(t *testing.T) {
	observer := New()

	observer.ChallengeIssued(auth.PurposeVerifyEmail)
	observer.ChallengeIssued(auth.Purpose("other_purpose"))
	observer.ChallengeIssued(auth.Purpose("other_purpose"))
	observer.ChallengeVerified(auth.ResultVerified)
	for _, result := range []string{
		auth.ResultInvalid, auth.ResultExpired, auth.ResultConsumed,
		auth.ResultExhausted, auth.ResultUnknown,
	} {
		observer.ChallengeVerified(result)
	}
	observer.ResendRequested(auth.ResendSent)
	observer.ResendRequested(auth.ResendSuppressed)
	observer.ResendRequested(auth.ResendCapped)
	observer.SendCapBlocked()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	observer.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	t.Run("Happy path: issuance is counted per purpose", func(t *testing.T) {
		for _, want := range []string{
			`legnaris_auth_otp_issued_total{purpose="verify_email"} 1`,
			`legnaris_auth_otp_issued_total{purpose="other_purpose"} 2`,
		} {
			assert.Contains(t, body, want)
		}
	})

	t.Run("Happy path: every verification result has its own series", func(t *testing.T) {
		for _, result := range []string{
			auth.ResultVerified, auth.ResultInvalid, auth.ResultExpired,
			auth.ResultConsumed, auth.ResultExhausted, auth.ResultUnknown,
		} {
			assert.Contains(t, body, fmt.Sprintf(`legnaris_auth_otp_verify_total{result="%s"} 1`, result))
		}
	})

	t.Run("Happy path: every resend result has its own series", func(t *testing.T) {
		for _, result := range []string{auth.ResendSent, auth.ResendSuppressed, auth.ResendCapped} {
			assert.Contains(t, body, fmt.Sprintf(`legnaris_auth_otp_resend_total{result="%s"} 1`, result))
		}
	})

	t.Run("Happy path: the cap refusal is counted", func(t *testing.T) {
		assert.Contains(t, body, "legnaris_auth_otp_cap_blocked_total 1")
	})

	t.Run("Edge: no OTP series carries a code or an unmasked address", func(t *testing.T) {
		// Scan only the code-step series; the Go/process collectors legitimately
		// expose runtime numbers.
		var otpLines []string
		for _, line := range strings.Split(body, "\n") {
			if strings.Contains(line, "legnaris_auth_otp") {
				otpLines = append(otpLines, line)
			}
		}
		joined := strings.Join(otpLines, "\n")
		assert.NotEmpty(t, otpLines, "the code-step series must be exposed")
		assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), joined)
		assert.NotContains(t, joined, "@")
	})
}

func scrapeHandler(t *testing.T, m *Metrics) string {
	t.Helper()

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

// TestWorkerMetrics pins the delivery series: every outcome is counted under
// its own label, the duration histogram observes, and the dead-letter and
// expired counters move on their own (WKR-21).
func TestWorkerMetrics(t *testing.T) {
	m := New()

	for _, result := range []string{"sent", "sent", "retried", "dead_lettered", "expired"} {
		m.SendResult(result)
	}
	m.SendDuration(250 * time.Millisecond)
	m.MessageDead()
	m.MessageExpired()
	m.MessageExpired()

	body := scrapeHandler(t, m)

	for _, want := range []string{
		`legnaris_email_send_total{result="sent"} 2`,
		`legnaris_email_send_total{result="retried"} 1`,
		`legnaris_email_send_total{result="dead_lettered"} 1`,
		`legnaris_email_send_total{result="expired"} 1`,
		"legnaris_email_send_duration_seconds_count 1",
		"legnaris_email_dead_total 1",
		"legnaris_email_expired_total 2",
	} {
		assert.Contains(t, body, want)
	}
}

// TestWorkerMetrics_NoSensitiveLabels proves no worker series carries a code or
// an address: the only label is the outcome (WKR-21, WKR-22).
func TestWorkerMetrics_NoSensitiveLabels(t *testing.T) {
	m := New()
	m.SendResult("sent")
	m.SendDuration(time.Second)
	m.MessageDead()
	m.MessageExpired()

	body := scrapeHandler(t, m)

	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "legnaris_email_send") &&
			!strings.HasPrefix(line, "legnaris_email_dead") &&
			!strings.HasPrefix(line, "legnaris_email_expired") {
			continue
		}
		assert.NotContains(t, line, "@", line)
		assert.NotRegexp(t, `\b\d{6}\b`, line)
		if strings.Contains(line, "{") && !strings.Contains(line, "le=") {
			assert.Regexp(t, `\{result="(sent|retried|dead_lettered|expired)"\}`, line)
		}
	}
}

// TestWorkerMetrics_UnexpectedResult proves an unknown outcome still lands in
// its own series rather than panicking: the caller owns the vocabulary.
func TestWorkerMetrics_UnexpectedResult(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() { m.SendResult("") })
}

func TestMiddlewareLatencyBucketsResolveSubFiveMillisecond(t *testing.T) {
	e := newTestRouter(t)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := scrapeMetrics(t, e)

	tests := []struct {
		name string
		le   string
	}{
		{name: "Happy path: finest bucket is 0.5ms", le: `le="0.0005"`},
		{name: "Edge: 1ms bucket exists", le: `le="0.001"`},
		{name: "Edge: 3ms bucket exists", le: `le="0.003"`},
		{name: "Edge: 2.5s upper bucket exists", le: `le="2.5"`},
		{name: "Edge: +Inf bucket exists", le: `le="+Inf"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, body, "legnaris_http_request_duration_seconds_bucket{method=\"GET\",path=\"/test\",status=\"200\","+tc.le)
		})
	}
}

// gaugeValue reads the current value of a gauge without scraping /metrics.
func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	var out dto.Metric
	require.NoError(t, g.Write(&out))
	return out.GetGauge().GetValue()
}

func TestMiddlewareInFlightGauge(t *testing.T) {
	newRouter := func(handler echo.HandlerFunc) (*echo.Echo, *Metrics) {
		e := echo.New()
		m := New()
		e.Use(m.Middleware)
		e.GET("/h", handler)
		return e, m
	}

	t.Run("Happy path: counts a request while it is being served and releases it after", func(t *testing.T) {
		var during float64
		var m *Metrics
		e, built := newRouter(func(c *echo.Context) error {
			during = gaugeValue(t, m.requestsInFlight)
			return c.String(http.StatusOK, "ok")
		})
		m = built

		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/h", nil))

		assert.InDelta(t, 1, during, 0)
		assert.InDelta(t, 0, gaugeValue(t, m.requestsInFlight), 0)
	})

	t.Run("Edge: releases the slot when the handler returns an error", func(t *testing.T) {
		e, m := newRouter(func(c *echo.Context) error {
			return errors.New("boom")
		})

		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/h", nil))

		assert.InDelta(t, 0, gaugeValue(t, m.requestsInFlight), 0)
	})

	t.Run("Edge: releases the slot when the handler panics", func(t *testing.T) {
		e, m := newRouter(func(c *echo.Context) error {
			panic("boom")
		})

		assert.Panics(t, func() {
			e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/h", nil))
		})

		assert.InDelta(t, 0, gaugeValue(t, m.requestsInFlight), 0)
	})

	t.Run("Edge: concurrent requests are all counted, then all released", func(t *testing.T) {
		const concurrent = 8
		arrived := make(chan struct{}, concurrent)
		release := make(chan struct{})
		e, m := newRouter(func(c *echo.Context) error {
			arrived <- struct{}{}
			<-release
			return c.String(http.StatusOK, "ok")
		})

		var wg sync.WaitGroup
		for i := 0; i < concurrent; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/h", nil))
			}()
		}
		for i := 0; i < concurrent; i++ {
			<-arrived
		}
		assert.InDelta(t, concurrent, gaugeValue(t, m.requestsInFlight), 0)

		close(release)
		wg.Wait()
		assert.InDelta(t, 0, gaugeValue(t, m.requestsInFlight), 0)
	})

	t.Run("Edge: gauge is exposed on /metrics at zero before any traffic", func(t *testing.T) {
		assert.Contains(t, scrapeMetrics(t, newTestRouter(t)), "legnaris_http_in_flight_requests")
	})
}
