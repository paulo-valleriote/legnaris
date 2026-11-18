package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rateLimitOrigin = "http://localhost:5173"

func authSurfaceRequest(t *testing.T, h http.Handler, method, path, body, origin, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRouterRateLimit_SessionRoutesUseTheSessionBucket(t *testing.T) {
	h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "1"})

	sessionRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/auth/csrf"},
		{method: http.MethodPost, path: "/auth/refresh"},
		{method: http.MethodPost, path: "/auth/logout"},
	}

	for _, route := range sessionRoutes {
		for i := 1; i <= 6; i++ {
			rec := authSurfaceRequest(t, h, route.method, route.path, route.body, rateLimitOrigin, "198.51.100.60:1")
			require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
				"%s %s request %d must not spend the auth budget", route.method, route.path, i)
		}
	}

	t.Run("Happy path: a login after many session requests is still admitted", func(t *testing.T) {
		rec := routeRequest(t, h, http.MethodPost, "/auth/login", `{}`, "", "198.51.100.60:1")
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
			"the auth budget must remain untouched by the session routes")
	})

	t.Run("Edge: the auth budget still applies to login itself", func(t *testing.T) {
		requireRateLimited(t, routeRequest(t, h, http.MethodPost, "/auth/login", `{}`, "", "198.51.100.60:1"))
	})
}

func TestRouterRateLimit_SessionBucketExhaustion(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/auth/csrf"},
		{method: http.MethodPost, path: "/auth/refresh"},
		{method: http.MethodPost, path: "/auth/logout"},
	}

	for _, route := range routes {
		t.Run("Happy path: "+route.method+" "+route.path+" is refused past the session cap", func(t *testing.T) {
			h, _ := newWiringRouterWithEnv(t, map[string]string{"SESSION_RATE_LIMIT_ATTEMPTS": "1"})

			first := authSurfaceRequest(t, h, route.method, route.path, "", rateLimitOrigin, "198.51.100.61:1")
			require.NotEqual(t, http.StatusTooManyRequests, first.Code, first.Body.String())

			requireRateLimited(t, authSurfaceRequest(t, h, route.method, route.path, "", rateLimitOrigin, "198.51.100.61:1"))
		})
	}

	t.Run("Edge: exhausting the session cap leaves login admitted", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"SESSION_RATE_LIMIT_ATTEMPTS": "1"})

		authSurfaceRequest(t, h, http.MethodPost, "/auth/refresh", "", rateLimitOrigin, "198.51.100.62:1")
		requireRateLimited(t, authSurfaceRequest(t, h, http.MethodPost, "/auth/refresh", "", rateLimitOrigin, "198.51.100.62:1"))

		rec := routeRequest(t, h, http.MethodPost, "/auth/login", `{}`, "", "198.51.100.62:1")
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code, "login keeps its own auth budget")
	})
}

func TestRouterRateLimit_LoginAccountBucketIsIndependent(t *testing.T) {
	h, _ := newWiringRouterWithEnv(t, map[string]string{
		"AUTH_RATE_LIMIT_ATTEMPTS":         "100",
		"AUTH_ACCOUNT_RATE_LIMIT_ATTEMPTS": "1",
	})

	loginBody := func(email string) string {
		return `{"email":"` + email + `","password":"wrong-password"}`
	}

	require.NotEqual(t, http.StatusTooManyRequests,
		routeRequest(t, h, http.MethodPost, "/auth/login", loginBody("victim@example.com"), "", "203.0.113.70:1").Code)

	t.Run("Happy path: the same account from another address is refused", func(t *testing.T) {
		requireRateLimited(t, routeRequest(t, h, http.MethodPost, "/auth/login", loginBody("Victim@Example.com"), "", "203.0.113.71:1"))
	})

	t.Run("Edge: another account from another address stays admitted", func(t *testing.T) {
		rec := routeRequest(t, h, http.MethodPost, "/auth/login", loginBody("other@example.com"), "", "203.0.113.72:1")
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	})
}

func TestRouterRateLimit_ChallengeVerifyUsesTheAuthBucket(t *testing.T) {
	h, _ := newWiringRouterWithEnv(t, map[string]string{
		"AUTH_RATE_LIMIT_ATTEMPTS":    "1",
		"SESSION_RATE_LIMIT_ATTEMPTS": "100",
	})

	first := routeRequest(t, h, http.MethodPost, "/auth/challenge/verify", `{"challenge_id":`, "", "198.51.100.63:1")
	require.Equal(t, http.StatusBadRequest, first.Code, first.Body.String())

	requireRateLimited(t, routeRequest(t, h, http.MethodPost, "/auth/challenge/verify", `{"challenge_id":`, "", "198.51.100.63:1"))
}
