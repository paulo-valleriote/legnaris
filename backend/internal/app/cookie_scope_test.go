package app_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_CookieScope pins AUTH-26 / SC-008: the bearer routes keep
// working with an Authorization header, never require a cookie, and are not
// touched by the origin or the rate-limit guard that protect the auth routes.
// The auth budget is 10 so the exhausted-limiter edge trips in a few requests.
func TestIntegration_CookieScope(t *testing.T) {
	h, mdb, _ := newIntegrationStack(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "10"})
	token, _ := newAuthUser(t, h, mdb, "cookie-scope@example.com", "cookie-scope")

	t.Run("Happy path: bearer routes work without a cookie and without an Origin", func(t *testing.T) {
		for _, path := range []string{"/templates", "/templates/fantasy_v1"} {
			rec := doAuthRequest(t, h, token, http.MethodGet, path, "")
			require.Equal(t, http.StatusOK, rec.Code, "%s must accept the bearer token", path)
			assert.Empty(t, rec.Header().Get("Set-Cookie"), "a bearer route must not set a session cookie")
		}

		createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Scope"}`)
		require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	})

	t.Run("Edge: an exhausted auth limiter does not block the bearer routes", func(t *testing.T) {
		exhausted := false
		for i := 0; i < 30; i++ {
			body := fmt.Sprintf(`{"email":"nobody%d@example.com","password":"wrong-password"}`, i)
			rec := doRequest(t, h, http.MethodPost, "/auth/login", body)
			if rec.Code == http.StatusTooManyRequests {
				exhausted = true
				break
			}
		}
		require.True(t, exhausted, "repeated logins must eventually be rate limited")

		// The bearer routes are not limited by the auth guard, so the access
		// token still works with no Origin and no cookie.
		rec := doAuthRequest(t, h, token, http.MethodGet, "/templates", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}
