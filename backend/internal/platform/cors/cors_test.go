package cors

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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

func TestMiddlewarePreflightAdvertisesDelete(t *testing.T) {
	e := echo.New()
	e.Use(Middleware([]string{"https://legnaris.com.br"}))
	e.DELETE("/test", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set(echo.HeaderOrigin, "https://legnaris.com.br")
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodDelete)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for preflight, got %d", rec.Code)
	}
	// The browser only lets through methods listed here, so a DELETE missing
	// from Access-Control-Allow-Methods would never reach the API.
	allowed := strings.Split(rec.Header().Get(echo.HeaderAccessControlAllowMethods), ",")
	if !slices.Contains(allowed, http.MethodDelete) {
		t.Errorf("expected Access-Control-Allow-Methods to include DELETE, got %q", rec.Header().Get(echo.HeaderAccessControlAllowMethods))
	}
}

func TestMiddlewarePreflightAdvertisesPut(t *testing.T) {
	e := echo.New()
	e.Use(Middleware([]string{"https://legnaris.com.br"}))
	e.PUT("/test", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set(echo.HeaderOrigin, "https://legnaris.com.br")
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPut)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for preflight, got %d", rec.Code)
	}
	allowed := strings.Split(rec.Header().Get(echo.HeaderAccessControlAllowMethods), ",")
	if !slices.Contains(allowed, http.MethodPut) {
		t.Errorf("expected Access-Control-Allow-Methods to include PUT, got %q", rec.Header().Get(echo.HeaderAccessControlAllowMethods))
	}
}

// advertisedAllowHeaders splits the preflight's Access-Control-Allow-Headers.
func advertisedAllowHeaders(rec *httptest.ResponseRecorder) []string {
	raw := rec.Header().Get(echo.HeaderAccessControlAllowHeaders)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func TestMiddlewareCredentialedPreflight(t *testing.T) {
	const origin = "https://legnaris.com.br"

	e := echo.New()
	e.Use(Middleware([]string{origin}))
	e.POST("/auth/refresh", func(c *echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/auth/refresh", nil)
	req.Header.Set(echo.HeaderOrigin, origin)
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
	req.Header.Set(echo.HeaderAccessControlRequestHeaders, csrfHeader)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for preflight, got %d", rec.Code)
	}
	if got := rec.Header().Get(echo.HeaderAccessControlAllowOrigin); got != origin {
		t.Errorf("expected the exact allowlisted origin, got %q", got)
	}
	if got := rec.Header().Get(echo.HeaderAccessControlAllowCredentials); got != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true, got %q", got)
	}
	if got := rec.Header().Get(echo.HeaderVary); !strings.Contains(got, echo.HeaderOrigin) {
		t.Errorf("expected Vary to include Origin, got %q", got)
	}

	advertised := advertisedAllowHeaders(rec)
	if !slices.Contains(advertised, csrfHeader) {
		t.Errorf("expected the preflight to advertise %s, got %v", csrfHeader, advertised)
	}
	if slices.Contains(advertised, "X-Refresh-Token") {
		t.Errorf("the retired native refresh header must not be advertised, got %v", advertised)
	}
}

func TestMiddlewareUnlistedPreflightAdvertisesNothing(t *testing.T) {
	e := echo.New()
	e.Use(Middleware([]string{"https://legnaris.com.br"}))
	e.POST("/auth/refresh", func(c *echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/auth/refresh", nil)
	req.Header.Set(echo.HeaderOrigin, "https://evil.example.com")
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
	req.Header.Set(echo.HeaderAccessControlRequestHeaders, csrfHeader)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if got := rec.Header().Get(echo.HeaderAccessControlAllowOrigin); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for an unlisted origin, got %q", got)
	}
	if got := rec.Header().Get(echo.HeaderAccessControlAllowHeaders); got != "" {
		t.Errorf("expected no Access-Control-Allow-Headers for an unlisted origin, got %q", got)
	}
}
