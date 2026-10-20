package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/logger"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logEntry is one captured structured log line, flattened so a test can assert
// a field by name.
type logEntry struct {
	level string
	msg   string
	attrs map[string]string
}

// recordingLogger captures every structured entry so a test can prove the
// report was logged with sanitised URLs (A4, A5).
type recordingLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

func (l *recordingLogger) record(level, msg string, args []logger.Attribute) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attrs := make(map[string]string, len(args))
	for _, arg := range args {
		attrs[arg.Key] = arg.Value
	}
	l.entries = append(l.entries, logEntry{level: level, msg: msg, attrs: attrs})
}

func (l *recordingLogger) Info(_ context.Context, msg string, args ...logger.Attribute) {
	l.record("info", msg, args)
}

func (l *recordingLogger) Debug(_ context.Context, msg string, args ...logger.Attribute) {
	l.record("debug", msg, args)
}

func (l *recordingLogger) Warn(_ context.Context, msg string, args ...logger.Attribute) {
	l.record("warn", msg, args)
}

func (l *recordingLogger) Error(_ context.Context, msg string, args ...logger.Attribute) {
	l.record("error", msg, args)
}

func (l *recordingLogger) Fatal(_ context.Context, msg string, args ...logger.Attribute) {
	l.record("fatal", msg, args)
}

func (l *recordingLogger) With(string, string) logger.StructuredLogger { return l }

// byLevel returns the captured entries at one level, in order.
func (l *recordingLogger) byLevel(level string) []logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var entries []logEntry
	for _, entry := range l.entries {
		if entry.level == level {
			entries = append(entries, entry)
		}
	}
	return entries
}

// containsAny reports whether any captured entry (at any level) carries a value
// matching the predicate, so a test can prove a raw secret never reached a log.
func (l *recordingLogger) containsAny(match func(logEntry) bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if match(entry) {
			return true
		}
	}
	return false
}

// cspHarness mounts the report collector with a real metrics registry and a
// frozen clock, mirroring the production wiring in NewRouter.
type cspHarness struct {
	echo *echo.Echo
	log  *recordingLogger
}

func newCSPHarness(attempts int) *cspHarness {
	log := &recordingLogger{}
	registry := metrics.New()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	controller := NewCSPReportController(log, registry)
	controller.now = func() time.Time { return now }
	limiter := ratelimit.New(attempts, time.Minute, func() time.Time { return now })

	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	registry.Register(e)
	e.POST("/csp-report", controller.Report, cspRateLimit(limiter))

	return &cspHarness{echo: e, log: log}
}

func (h *cspHarness) post(t *testing.T, contentType, body, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/csp-report", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set(echo.HeaderContentType, contentType)
	}
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)
	return rec
}

// scrape returns the exposed metrics so a test can assert the counter series.
func (h *cspHarness) scrape(t *testing.T) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestCSPReport(t *testing.T) {
	t.Run("Happy path: the legacy csp-report format is collected with sanitised URLs", func(t *testing.T) {
		h := newCSPHarness(30)

		body := `{"csp-report":{` +
			`"document-uri":"https://app.legnaris.com.br/sheets/1?token=secret#frag",` +
			`"blocked-uri":"https://evil.example.com/track?u=1#x",` +
			`"effective-directive":"script-src","disposition":"enforce"}}`

		rec := h.post(t, "application/csp-report", body, "10.0.0.1:1")
		require.Equal(t, http.StatusNoContent, rec.Code)

		infos := h.log.byLevel("info")
		require.Len(t, infos, 1)
		assert.Equal(t, cspReportLogMessage, infos[0].msg)
		assert.Equal(t, "script-src", infos[0].attrs["effective_directive"])
		assert.Equal(t, "enforce", infos[0].attrs["disposition"])
		assert.Equal(t, "2026-10-18T12:00:00Z", infos[0].attrs["timestamp"])
		assert.Equal(t, "https://evil.example.com/track", infos[0].attrs["blocked_uri"])
		assert.Equal(t, "https://app.legnaris.com.br/sheets/1", infos[0].attrs["document_uri"])

		assert.False(t, h.log.containsAny(func(entry logEntry) bool {
			return strings.Contains(entry.attrs["blocked_uri"], "u=1") ||
				strings.Contains(entry.attrs["document_uri"], "token=secret")
		}), "the query and fragment must never reach the log")

		assert.Contains(t, h.scrape(t),
			`legnaris_csp_reports_total{disposition="enforce",effective_directive="script-src"} 1`)
	})

	t.Run("Happy path: the reports+json array is normalised and non-violations are skipped", func(t *testing.T) {
		h := newCSPHarness(30)

		body := `[` +
			`{"type":"csp-violation","body":{"documentURL":"https://app.legnaris.com.br/","blockedURL":"inline","effectiveDirective":"style-src","disposition":"report"}},` +
			`{"type":"deprecation","body":{"documentURL":"https://app.legnaris.com.br/"}}` +
			`]`

		rec := h.post(t, "application/reports+json", body, "10.0.0.1:1")
		require.Equal(t, http.StatusNoContent, rec.Code)

		infos := h.log.byLevel("info")
		require.Len(t, infos, 1, "only the CSP violation is collected")
		assert.Equal(t, "style-src", infos[0].attrs["effective_directive"])
		assert.Equal(t, "inline", infos[0].attrs["blocked_uri"])

		assert.Contains(t, h.scrape(t), `legnaris_csp_reports_total{disposition="report",effective_directive="style-src"} 1`)
	})

	edgeCases := []struct {
		name        string
		contentType string
		body        string
		wantInfo    int
	}{
		{
			name:        "Edge: an empty body is a no-op",
			contentType: "application/csp-report",
			body:        "",
		},
		{
			name:        "Edge: a whitespace-only body is a no-op",
			contentType: "application/csp-report",
			body:        "   \n\t ",
		},
		{
			name:        "Edge: an empty reports+json array is a no-op",
			contentType: "application/reports+json",
			body:        "[]",
		},
		{
			name:        "Edge: an empty legacy envelope is a no-op",
			contentType: "application/csp-report",
			body:        `{"csp-report":{}}`,
		},
		{
			name:        "Edge: an unrelated JSON object is a no-op",
			contentType: "application/json",
			body:        `{"foo":"bar"}`,
		},
		{
			name:        "Edge: application/json is tolerated and normalised",
			contentType: "application/json",
			body:        `{"csp-report":{"effective-directive":"img-src","disposition":"enforce"}}`,
			wantInfo:    1,
		},
		{
			name:        "Edge: an unexpected content type is still normalised",
			contentType: "text/plain",
			body:        `{"csp-report":{"effective-directive":"font-src","disposition":"enforce"}}`,
			wantInfo:    1,
		},
		{
			name:        "Edge: a missing content type is still normalised",
			contentType: "",
			body:        `{"csp-report":{"effective-directive":"connect-src","disposition":"enforce"}}`,
			wantInfo:    1,
		},
		{
			name:        "Edge: the legacy violated-directive fills in for a missing effective-directive",
			contentType: "application/csp-report",
			body:        `{"csp-report":{"violated-directive":"object-src","disposition":"enforce"}}`,
			wantInfo:    1,
		},
		{
			name:        "Edge: a single report-to object is normalised",
			contentType: "application/reports+json",
			body:        `{"type":"csp-violation","body":{"effectiveDirective":"media-src","disposition":"enforce"}}`,
			wantInfo:    1,
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newCSPHarness(30)

			rec := h.post(t, tc.contentType, tc.body, "10.0.0.1:1")
			require.Equal(t, http.StatusNoContent, rec.Code, "a report always answers 204")
			assert.Len(t, h.log.byLevel("info"), tc.wantInfo)
			assert.Empty(t, h.log.byLevel("error"))
		})
	}

	t.Run("Error: a malformed body is logged and still answers 204", func(t *testing.T) {
		h := newCSPHarness(30)

		rec := h.post(t, "application/csp-report", `{"csp-report":`, "10.0.0.1:1")
		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, h.log.byLevel("info"))

		errors := h.log.byLevel("error")
		require.Len(t, errors, 1)
		assert.NotContains(t, h.scrape(t), "legnaris_csp_reports_total{",
			"a body that decoded nothing must not create a series")
	})

	t.Run("Edge: a body past the cap answers 204 without panicking", func(t *testing.T) {
		h := newCSPHarness(30)

		oversized := `{"csp-report":{"effective-directive":"script-src","disposition":"enforce","blocked-uri":"` +
			strings.Repeat("a", cspReportMaxBody) + `"}}`

		rec := h.post(t, "application/csp-report", oversized, "10.0.0.1:1")
		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, h.log.byLevel("info"))
		require.Len(t, h.log.byLevel("error"), 1)
	})

	t.Run("Edge: an empty directive and disposition get a stable metric label", func(t *testing.T) {
		h := newCSPHarness(30)

		rec := h.post(t, "application/csp-report",
			`{"csp-report":{"document-uri":"https://app.legnaris.com.br/"}}`, "10.0.0.1:1")
		require.Equal(t, http.StatusNoContent, rec.Code)

		assert.Contains(t, h.scrape(t),
			`legnaris_csp_reports_total{disposition="unknown",effective_directive="unknown"} 1`)
	})

	t.Run("Edge: a relative blocked-uri is sanitised", func(t *testing.T) {
		h := newCSPHarness(30)

		rec := h.post(t, "application/csp-report",
			`{"csp-report":{"blocked-uri":"/static/app.js?v=2#top","effective-directive":"script-src","disposition":"enforce"}}`,
			"10.0.0.1:1")
		require.Equal(t, http.StatusNoContent, rec.Code)

		infos := h.log.byLevel("info")
		require.Len(t, infos, 1)
		assert.Equal(t, "/static/app.js", infos[0].attrs["blocked_uri"])
	})

	t.Run("Error: the report past the address cap is 429 with Retry-After", func(t *testing.T) {
		h := newCSPHarness(1)
		body := `{"csp-report":{"effective-directive":"script-src","disposition":"enforce"}}`

		require.Equal(t, http.StatusNoContent, h.post(t, "application/csp-report", body, "10.0.0.1:1").Code)

		rec := h.post(t, "application/csp-report", body, "10.0.0.1:1")
		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Equal(t, "60", rec.Header().Get(echo.HeaderRetryAfter))
		assert.Len(t, h.log.byLevel("info"), 1, "a refused report never reaches the collector")

		// The block is not counted against the auth rate-limit metric, which stays
		// at its zero value (AUTH-20).
		assert.Contains(t, h.scrape(t), "legnaris_auth_rate_limited_total 0")
	})

	t.Run("Edge: the address cap is independent per address and namespaced from auth", func(t *testing.T) {
		h := newCSPHarness(1)
		body := `{"csp-report":{"effective-directive":"script-src","disposition":"enforce"}}`

		require.Equal(t, http.StatusNoContent, h.post(t, "application/csp-report", body, "10.0.0.1:1").Code)
		require.Equal(t, http.StatusTooManyRequests, h.post(t, "application/csp-report", body, "10.0.0.1:1").Code)
		assert.Equal(t, http.StatusNoContent, h.post(t, "application/csp-report", body, "10.0.0.2:1").Code)
	})
}
