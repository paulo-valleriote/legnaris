package app_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postLogin drives the real router's login route with an explicit direct peer
// and forwarding chain. The unreachable store answers 500 when the request
// passes the guard, so 429 alone identifies a refused request.
func postLogin(t *testing.T, h http.Handler, email, remoteAddr string, forwarded ...string) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"password":"wrong-password"}`, email)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remoteAddr
	for _, value := range forwarded {
		req.Header.Add("X-Forwarded-For", value)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func requireGuarded(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	require.NotEqual(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusInternalServerError, rec.Code,
		"a request that passes the guard reaches the handler, which fails on the unreachable store")
}

func requireLimited(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderRetryAfter))
}

// TestClientIP_RateLimitKeyedByResolvedAddress proves the router installs the
// address policy: two clients behind one trusted proxy are limited
// independently, and an untrusted peer cannot choose its own key (PRQ-01,
// PRQ-02, PRQ-04, SC-001, SC-002).
func TestClientIP_RateLimitKeyedByResolvedAddress(t *testing.T) {
	const trustedPeer = "10.0.0.1:1234"

	t.Run("Happy path: one client exhausts its own limit only", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "1"})

		requireGuarded(t, postLogin(t, h, "client-a@example.com", trustedPeer, "203.0.113.10"))
		requireLimited(t, postLogin(t, h, "client-a@example.com", trustedPeer, "203.0.113.10"))
	})

	t.Run("Edge: a second client behind the same proxy stays admitted", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "1"})

		requireGuarded(t, postLogin(t, h, "client-a@example.com", trustedPeer, "203.0.113.10"))
		requireLimited(t, postLogin(t, h, "client-a@example.com", trustedPeer, "203.0.113.10"))
		requireGuarded(t, postLogin(t, h, "client-b@example.com", trustedPeer, "203.0.113.11"))
	})

	t.Run("Edge: an untrusted peer cannot choose its key with a forged chain", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "1"})

		const untrustedPeer = "198.51.100.9:1234"
		requireGuarded(t, postLogin(t, h, "forged-one@example.com", untrustedPeer, "203.0.113.10"))
		requireLimited(t, postLogin(t, h, "forged-two@example.com", untrustedPeer, "203.0.113.99"))
	})

	t.Run("Edge: an absent header uses the direct peer", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "1"})

		requireGuarded(t, postLogin(t, h, "no-header@example.com", "10.0.0.7:1234"))
		requireLimited(t, postLogin(t, h, "no-header@example.com", "10.0.0.7:1234"))
	})

	t.Run("Edge: a host:port chain resolves instead of collapsing to the proxy bucket", func(t *testing.T) {
		h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "1"})

		requireGuarded(t, postLogin(t, h, "port-a@example.com", trustedPeer, "203.0.113.10:443"))
		requireLimited(t, postLogin(t, h, "port-a@example.com", trustedPeer, "203.0.113.10:443"))
		requireGuarded(t, postLogin(t, h, "port-b@example.com", trustedPeer, "203.0.113.11:443"))
	})
}
