package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func routeRequest(t *testing.T, h http.Handler, method, path, body, token, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func requireRateLimited(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "RATE_LIMITED", body.Error.Code)

	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err, "a refusal must advertise Retry-After")
	assert.GreaterOrEqual(t, retryAfter, 1)
}

func TestRouterRateLimit_LoginEleventhRequestIsRefused(t *testing.T) {
	h, _ := newWiringRouter(t)

	for i := 1; i <= 10; i++ {
		rec := routeRequest(t, h, http.MethodPost, "/auth/login", `{}`, "", "198.51.100.7:4000")
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "request %d must be admitted", i)
	}

	requireRateLimited(t, routeRequest(t, h, http.MethodPost, "/auth/login", `{}`, "", "198.51.100.7:4000"))

	t.Run("Edge: another address keeps its own budget", func(t *testing.T) {
		rec := routeRequest(t, h, http.MethodPost, "/auth/login", `{}`, "", "198.51.100.8:4000")
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	})
}

func TestRouterRateLimit_AuthenticatedRoutesArePerUser(t *testing.T) {
	h, modules := newWiringRouterWithEnv(t, map[string]string{"USER_RATE_LIMIT_ATTEMPTS": "3"})

	tokenFor := func(userID string) string {
		access, err := modules.Auth.TokenManager.Issue(userID)
		require.NoError(t, err)
		return access.Value
	}
	alice, bob := tokenFor("alice"), tokenFor("bob")

	t.Run("Happy path: the request over the cap is 429 across protected routes", func(t *testing.T) {
		for i := 1; i <= 3; i++ {
			rec := routeRequest(t, h, http.MethodGet, "/templates", "", alice, "203.0.113.1:1")
			require.Equal(t, http.StatusOK, rec.Code, "request %d must be admitted", i)
		}

		requireRateLimited(t, routeRequest(t, h, http.MethodGet, "/templates", "", alice, "203.0.113.1:1"))
		requireRateLimited(t, routeRequest(t, h, http.MethodGet, "/sheets", "", alice, "203.0.113.1:1"))
	})

	t.Run("Edge: another user behind the same address is unaffected", func(t *testing.T) {
		rec := routeRequest(t, h, http.MethodGet, "/templates", "", bob, "203.0.113.1:1")
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Edge: the same user from another address shares the budget", func(t *testing.T) {
		requireRateLimited(t, routeRequest(t, h, http.MethodGet, "/templates", "", alice, "203.0.113.99:1"))
	})

	t.Run("Edge: unauthenticated requests are 401 and never spend a budget", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			rec := routeRequest(t, h, http.MethodGet, "/templates", "", "", "203.0.113.50:1")
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		}
	})
}

func TestRouterRateLimit_PublicProbesArePerAddress(t *testing.T) {
	for _, path := range []string{"/health", "/metrics"} {
		t.Run(path, func(t *testing.T) {
			h, _ := newWiringRouterWithEnv(t, map[string]string{"PUBLIC_RATE_LIMIT_ATTEMPTS": "2"})

			for i := 1; i <= 2; i++ {
				rec := routeRequest(t, h, http.MethodGet, path, "", "", "198.51.100.20:1")
				require.Equal(t, http.StatusOK, rec.Code, "request %d must be admitted", i)
			}

			requireRateLimited(t, routeRequest(t, h, http.MethodGet, path, "", "", "198.51.100.20:1"))

			t.Run("Edge: another address keeps its own budget", func(t *testing.T) {
				rec := routeRequest(t, h, http.MethodGet, path, "", "", "198.51.100.21:1")
				assert.Equal(t, http.StatusOK, rec.Code)
			})
		})
	}

}

func TestRouterRateLimit_AuthSurfaceIsCovered(t *testing.T) {
	t.Run("Happy path: the eleventh register from one address is 429", func(t *testing.T) {
		h, _ := newWiringRouter(t)

		for i := 1; i <= 10; i++ {
			rec := routeRequest(t, h, http.MethodPost, "/auth/register", `{}`, "", "198.51.100.40:1")
			require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "request %d must be admitted", i)
		}

		requireRateLimited(t, routeRequest(t, h, http.MethodPost, "/auth/register", `{}`, "", "198.51.100.40:1"))
	})

	t.Run("Edge: logout is capped per address", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "2"})

		for i := 1; i <= 2; i++ {
			rec := routeRequest(t, h, http.MethodPost, "/auth/logout", "", "", "198.51.100.41:1")
			require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "request %d must be admitted", i)
		}

		requireRateLimited(t, routeRequest(t, h, http.MethodPost, "/auth/logout", "", "", "198.51.100.41:1"))
	})

	t.Run("Edge: the identity read is capped per user", func(t *testing.T) {
		h, modules := newWiringRouterWithEnv(t, map[string]string{"USER_RATE_LIMIT_ATTEMPTS": "2"})
		access, err := modules.Auth.TokenManager.Issue("me-user")
		require.NoError(t, err)

		for i := 1; i <= 2; i++ {
			rec := routeRequest(t, h, http.MethodGet, "/auth/me", "", access.Value, "198.51.100.42:1")
			require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "request %d must be admitted", i)
		}

		requireRateLimited(t, routeRequest(t, h, http.MethodGet, "/auth/me", "", access.Value, "198.51.100.42:1"))
	})

	t.Run("Edge: an anonymous identity read is 401, never 429", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"USER_RATE_LIMIT_ATTEMPTS": "1"})

		for i := 0; i < 5; i++ {
			rec := routeRequest(t, h, http.MethodGet, "/auth/me", "", "", "198.51.100.43:1")
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		}
	})
}

func TestRouterRateLimit_PublicProbesHaveIndependentBudgets(t *testing.T) {
	h, _ := newWiringRouterWithEnv(t, map[string]string{"PUBLIC_RATE_LIMIT_ATTEMPTS": "1"})
	addr := "198.51.100.50:1"

	require.Equal(t, http.StatusOK, routeRequest(t, h, http.MethodGet, "/health", "", "", addr).Code)
	requireRateLimited(t, routeRequest(t, h, http.MethodGet, "/health", "", "", addr))

	assert.Equal(t, http.StatusOK, routeRequest(t, h, http.MethodGet, "/metrics", "", "", addr).Code,
		"spending the health budget must not consume the metrics scrape")
}
