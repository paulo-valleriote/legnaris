package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestMiddlewareAllowsConfiguredOrigin(t *testing.T) {
	e := echo.New()
	e.Use(Middleware([]string{"https://legnaris.com.br"}))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set(echo.HeaderOrigin, "https://legnaris.com.br")
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodGet)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for preflight, got %d", rec.Code)
	}
	if got := rec.Header().Get(echo.HeaderAccessControlAllowOrigin); got != "https://legnaris.com.br" {
		t.Errorf("expected Access-Control-Allow-Origin to be set, got %q", got)
	}
}

func TestMiddlewareBlocksDisallowedOrigin(t *testing.T) {
	e := echo.New()
	e.Use(Middleware([]string{"https://legnaris.com.br"}))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(echo.HeaderOrigin, "https://evil.example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if got := rec.Header().Get(echo.HeaderAccessControlAllowOrigin); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", got)
	}
}
