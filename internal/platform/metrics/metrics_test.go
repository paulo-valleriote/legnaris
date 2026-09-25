package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestMiddlewareRecordsMetrics(t *testing.T) {
	e := echo.New()
	m := New()
	m.Register(e)
	e.Use(m.Middleware)
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

func TestMiddlewareNormalizesUnmatchedPath(t *testing.T) {
	e := echo.New()
	m := New()
	m.Register(e)
	e.Use(m.Middleware)
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// Request an unmatched route so the path label falls back to "unmatched".
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	mreq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, mreq)
	if mrec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from /metrics, got %d", mrec.Code)
	}

	if body := mrec.Body.String(); !strings.Contains(body, `path="unmatched"`) {
		t.Errorf("expected an unmatched-path series in /metrics output, got:\n%s", body)
	}
}

// TestNewRegistersRuntimeCollectors asserts the private registry also exposes
// the standard Go/process collectors, so the deployment's own resource metrics
// are present without depending on the process-global default registry.
func TestNewRegistersRuntimeCollectors(t *testing.T) {
	e := echo.New()
	m := New()
	m.Register(e)

	mreq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, mreq)
	if mrec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from /metrics, got %d", mrec.Code)
	}

	body := mrec.Body.String()
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
// middleware order (internal/app/router.go): an error-handling middleware runs
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

	mreq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, mreq)
	if mrec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from /metrics, got %d", mrec.Code)
	}

	want := `legnaris_http_requests_total{method="GET",path="/missing",status="404"}`
	if body := mrec.Body.String(); !strings.Contains(body, want) {
		t.Errorf("expected %q in /metrics output, got:\n%s", want, body)
	}
}
