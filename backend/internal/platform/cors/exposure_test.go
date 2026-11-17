package cors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestMiddlewareAllowsConditionalRequestsAndExposesETag(t *testing.T) {
	const origin = "https://legnaris.com.br"
	e := echo.New()
	e.Use(Middleware([]string{origin}))
	e.GET("/sheet", func(c *echo.Context) error {
		c.Response().Header().Set("ETag", `"1.0.gm"`)
		return c.String(http.StatusOK, "ok")
	})

	t.Run("Happy path: preflight allows If-None-Match", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/sheet", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		req.Header.Set("Access-Control-Request-Headers", "If-None-Match")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		allowed := rec.Header().Get("Access-Control-Allow-Headers")
		if !strings.Contains(strings.ToLower(allowed), "if-none-match") {
			t.Fatalf("Access-Control-Allow-Headers = %q, want If-None-Match", allowed)
		}
	})

	t.Run("Happy path: simple GET exposes ETag to the browser", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sheet", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		exposed := rec.Header().Get("Access-Control-Expose-Headers")
		if !strings.Contains(strings.ToLower(exposed), "etag") {
			t.Fatalf("Access-Control-Expose-Headers = %q, want ETag", exposed)
		}
	})

	t.Run("Edge: unknown origin gets no CORS allowance", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/sheet", nil)
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		req.Header.Set("Access-Control-Request-Headers", "If-None-Match")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("Access-Control-Allow-Origin = %q for an unknown origin, want empty", got)
		}
	})
}
