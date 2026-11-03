package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/platform/clientip"
	"legnaris/backend/internal/platform/ratelimit"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replica is one API pod: its own router and limiters over a shared Redis.
type replica struct {
	echo *echo.Echo
}

func newReplica(t *testing.T, client redis.Scripter, policy *clientip.Policy, attempts int) *replica {
	t.Helper()

	ok := func(c *echo.Context) error { return c.NoContent(http.StatusOK) }
	withUser := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if id := c.Request().Header.Get("X-Test-User"); id != "" {
				c.Set(auth.UserIDContextKey, id)
			}
			return next(c)
		}
	}

	authLimiter := newLimiter(client, "auth", attempts, time.Minute, nil)
	userLimiter := newLimiter(client, "user", attempts, time.Minute, nil)
	publicLimiter := newLimiter(client, "public", attempts, time.Minute, nil)

	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.IPExtractor = policy.Resolve
	e.POST("/login", ok, rateLimit(authLimiter, nil, true))
	e.GET("/me", ok, withUser, userRateLimit(userLimiter))
	e.POST("/csp-report", ok, cspRateLimit(publicLimiter))

	return &replica{echo: e}
}

func (r *replica) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.echo.ServeHTTP(rec, req)
	return rec
}

func loginRequest(remoteAddr, forwarded, email string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginBody(email)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remoteAddr
	if forwarded != "" {
		req.Header.Set("X-Forwarded-For", forwarded)
	}
	return req
}

func TestNewLimiter(t *testing.T) {
	t.Run("Happy path: no Redis client yields the in-process limiter", func(t *testing.T) {
		limiter := newLimiter(nil, "auth", 1, time.Minute, nil)

		assert.IsType(t, &ratelimit.Limiter{}, limiter)
	})

	t.Run("Happy path: a Redis client yields the shared limiter", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
		t.Cleanup(func() { _ = client.Close() })

		limiter := newLimiter(client, "auth", 1, time.Minute, nil)

		assert.IsType(t, &ratelimit.Shared{}, limiter)
	})
}

func TestRateLimit_SharedAcrossReplicas(t *testing.T) {
	policy, err := clientip.New([]string{"10.0.0.0/8"})
	require.NoError(t, err)

	newPair := func(t *testing.T, attempts int) (*replica, *replica) {
		client := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
		t.Cleanup(func() { _ = client.Close() })
		return newReplica(t, client, policy, attempts), newReplica(t, client, policy, attempts)
	}

	t.Run("Happy path: auth cap holds across two replicas for one real client behind the proxy", func(t *testing.T) {
		podA, podB := newPair(t, 2)

		first := podA.do(loginRequest("10.0.0.1:1111", "203.0.113.7", "a@example.com"))
		second := podB.do(loginRequest("10.0.0.2:2222", "203.0.113.7", "b@example.com"))
		third := podA.do(loginRequest("10.0.0.1:1111", "203.0.113.7", "c@example.com"))

		assert.Equal(t, http.StatusOK, first.Code)
		assert.Equal(t, http.StatusOK, second.Code)
		assertRateLimited(t, third)
	})

	t.Run("Edge: a different real client behind the same proxy keeps its own budget", func(t *testing.T) {
		podA, podB := newPair(t, 1)

		require.Equal(t, http.StatusOK, podA.do(loginRequest("10.0.0.1:1", "203.0.113.7", "a@example.com")).Code)

		other := podB.do(loginRequest("10.0.0.2:1", "203.0.113.8", "b@example.com"))

		assert.Equal(t, http.StatusOK, other.Code)
	})

	t.Run("Edge: the per-account budget is shared across replicas and addresses", func(t *testing.T) {
		podA, podB := newPair(t, 1)

		require.Equal(t, http.StatusOK, podA.do(loginRequest("10.0.0.1:1", "203.0.113.7", "victim@example.com")).Code)

		attack := podB.do(loginRequest("10.0.0.2:1", "198.51.100.9", "victim@example.com"))

		assertRateLimited(t, attack)
	})

	t.Run("Happy path: the user cap holds across two replicas", func(t *testing.T) {
		podA, podB := newPair(t, 1)
		req := func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/me", nil)
			r.Header.Set("X-Test-User", "u1")
			return r
		}

		assert.Equal(t, http.StatusOK, podA.do(req()).Code)
		assertRateLimited(t, podB.do(req()))
	})

	t.Run("Edge: an anonymous request never spends a user's shared budget", func(t *testing.T) {
		podA, _ := newPair(t, 1)

		for i := 0; i < 3; i++ {
			rec := podA.do(httptest.NewRequest(http.MethodGet, "/me", nil))
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("Happy path: the public CSP cap holds across two replicas", func(t *testing.T) {
		podA, podB := newPair(t, 1)
		req := func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/csp-report", strings.NewReader("{}"))
			r.RemoteAddr = "203.0.113.50:1"
			return r
		}

		assert.Equal(t, http.StatusOK, podA.do(req()).Code)
		assertRateLimited(t, podB.do(req()))
	})

	t.Run("Edge: auth, user and public budgets do not drain each other", func(t *testing.T) {
		podA, _ := newPair(t, 1)

		require.Equal(t, http.StatusOK, podA.do(loginRequest("203.0.113.50:1", "", "a@example.com")).Code)

		csp := httptest.NewRequest(http.MethodPost, "/csp-report", strings.NewReader("{}"))
		csp.RemoteAddr = "203.0.113.50:1"
		me := httptest.NewRequest(http.MethodGet, "/me", nil)
		me.Header.Set("X-Test-User", "203.0.113.50")

		assert.Equal(t, http.StatusOK, podA.do(csp).Code)
		assert.Equal(t, http.StatusOK, podA.do(me).Code)
	})

	t.Run("Edge: Redis going down fails open to the per-pod cap", func(t *testing.T) {
		server := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: server.Addr(), DialTimeout: 100 * time.Millisecond, MaxRetries: -1})
		t.Cleanup(func() { _ = client.Close() })
		pod := newReplica(t, client, policy, 2)
		server.Close()

		first := pod.do(loginRequest("10.0.0.1:1", "203.0.113.7", "a@example.com"))
		second := pod.do(loginRequest("10.0.0.1:1", "203.0.113.7", "b@example.com"))
		third := pod.do(loginRequest("10.0.0.1:1", "203.0.113.7", "c@example.com"))

		assert.Equal(t, http.StatusOK, first.Code, "an outage must not lock every login out")
		assert.Equal(t, http.StatusOK, second.Code)
		assertRateLimited(t, third)
	})
}

func TestStoreFailureReporter(t *testing.T) {
	t0 := time.Unix(1_000, 0)

	t.Run("Happy path: the first failure is logged as a warning with the namespace", func(t *testing.T) {
		log := &recordingLogger{}
		report := storeFailureReporter(log, "auth", time.Minute, func() time.Time { return t0 })

		report(assert.AnError)

		warns := log.byLevel("warn")
		require.Len(t, warns, 1)
		assert.Equal(t, "auth", warns[0].attrs["limiter"])
		assert.Equal(t, assert.AnError.Error(), warns[0].attrs["error"])
	})

	t.Run("Edge: failures inside the interval are suppressed", func(t *testing.T) {
		log := &recordingLogger{}
		now := t0
		report := storeFailureReporter(log, "auth", time.Minute, func() time.Time { return now })

		report(assert.AnError)
		now = t0.Add(59 * time.Second)
		report(assert.AnError)

		assert.Len(t, log.byLevel("warn"), 1)
	})

	t.Run("Edge: a failure after the interval is logged again", func(t *testing.T) {
		log := &recordingLogger{}
		now := t0
		report := storeFailureReporter(log, "auth", time.Minute, func() time.Time { return now })

		report(assert.AnError)
		now = t0.Add(time.Minute)
		report(assert.AnError)

		assert.Len(t, log.byLevel("warn"), 2)
	})
}

func TestNewLimiterClient(t *testing.T) {
	t.Run("Edge: an empty URL selects the in-process limiters", func(t *testing.T) {
		client, closers, err := newLimiterClient("")

		require.NoError(t, err)
		assert.Nil(t, client)
		assert.Empty(t, closers)
	})

	t.Run("Error: a malformed URL fails instead of silently disabling the shared store", func(t *testing.T) {
		_, _, err := newLimiterClient("not a url")

		assert.Error(t, err)
	})

	t.Run("Happy path: a valid URL yields a client and a closer; an unreachable server does not block", func(t *testing.T) {
		client, closers, err := newLimiterClient("redis://127.0.0.1:1/0")

		require.NoError(t, err)
		require.NotNil(t, client)
		require.Len(t, closers, 1)
		assert.NoError(t, (&Modules{closers: closers}).Close())
	})
}

func TestModules_Close(t *testing.T) {
	t.Run("Edge: closing modules without Redis is a no-op", func(t *testing.T) {
		assert.NoError(t, (&Modules{}).Close())
	})
}
