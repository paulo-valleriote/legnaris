package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestMiddlewareRecordsMetrics(t *testing.T) {
	e := echo.New()
	Register(e)
	e.Use(Middleware)
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// Exercise the middleware so the metrics gain at least one observation.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Scrape /metrics and assert our series are present.
	mreq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, mreq)
	if mrec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from /metrics, got %d", mrec.Code)
	}

	body := mrec.Body.String()
	for _, want := range []string{
		"legnaris_http_request_duration_seconds_bucket",
		"legnaris_http_requests_total",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in /metrics output", want)
		}
	}
}
