package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingObserver struct {
	blocks int
}

func (o *countingObserver) RateLimitBlocked() { o.blocks++ }

type rateLimitHarness struct {
	echo    *echo.Echo
	calls   *int
	session *countingObserver
}

func newRateLimitHarness(limiter *ratelimit.Limiter) *rateLimitHarness {
	return newScopedRateLimitHarness(limiter, limiter, limiter)
}

func newScopedRateLimitHarness(authLimiter, accountLimiter, sessionLimiter ratelimit.Allower) *rateLimitHarness {
	observer := &countingObserver{}
	calls := 0
	handler := func(c *echo.Context) error {
		calls++
		return c.NoContent(http.StatusOK)
	}

	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.POST("/login", handler, rateLimit(authLimiter, accountLimiter, observer))
	e.POST("/register", handler, rateLimit(authLimiter, accountLimiter, observer))
	e.POST("/refresh", handler, rateLimit(sessionLimiter, nil, observer))
	e.GET("/csrf", handler, rateLimit(sessionLimiter, nil, observer))

	return &rateLimitHarness{echo: e, calls: &calls, session: observer}
}

func (h *rateLimitHarness) get(t *testing.T, path, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)
	return rec
}

func (h *rateLimitHarness) post(t *testing.T, path, body, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)
	return rec
}

func loginBody(email string) string {
	payload, err := json.Marshal(map[string]string{"email": email, "password": "sup3r-secret"})
	if err != nil {
		panic(err)
	}
	return string(payload)
}

func TestAccountKey(t *testing.T) {
	t.Run("Happy path: equivalent spellings share one key", func(t *testing.T) {
		for _, email := range []string{
			"Name <Player@Example.com>",
			"  player@example.com ",
			"PLAYER@EXAMPLE.COM",
		} {
			assert.Equal(t, "account:player@example.com", accountKey(email), email)
		}
	})

	t.Run("Edge: +tag and dotted spellings keep distinct keys", func(t *testing.T) {
		assert.Equal(t, "account:player+news@example.com", accountKey("Player+News@Example.com"))
		assert.Equal(t, "account:p.layer@example.com", accountKey("P.Layer@Example.com"))
		assert.NotEqual(t, accountKey("player@example.com"), accountKey("player+news@example.com"))
	})

	t.Run("Edge: an unparseable value falls back to a stable trimmed lowercase key", func(t *testing.T) {
		assert.Equal(t, "account:not-an-email", accountKey("  Not-An-Email  "))
		assert.NotEqual(t, "account:", accountKey("not-an-email"))
	})
}

func assertRateLimited(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, string(apperr.CodeRateLimited), body.Error.Code)
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderRetryAfter))
}

func TestRateLimit(t *testing.T) {
	t.Run("Happy path: requests under the cap reach the handler", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(2, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)
		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assert.Equal(t, 2, *harness.calls)
		assert.Zero(t, harness.session.blocks)
	})

	t.Run("Error: the request past the limit is 429 with Retry-After and no credential work", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)

		rec := harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1")
		assertRateLimited(t, rec)
		assert.Equal(t, "60", rec.Header().Get(echo.HeaderRetryAfter))
		assert.Equal(t, 1, *harness.calls, "a blocked request must not reach the handler")
		assert.Equal(t, 1, harness.session.blocks, "the block must increment the counter exactly once")
	})

	t.Run("Edge: a different address is unaffected", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assertRateLimited(t, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1"))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("b@example.com"), "10.0.0.2:1").Code)
		assert.Equal(t, 2, *harness.calls)
	})

	t.Run("Edge: the same account from another address is blocked by the account key", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)

		assertRateLimited(t, harness.post(t, "/login", loginBody("A@Example.com"), "10.0.0.2:1"))
		assert.Equal(t, 1, *harness.calls, "the account key must block the credential work")
	})

	t.Run("Edge: a display-name spelling shares the account bucket", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)

		assertRateLimited(t, harness.post(t, "/login", loginBody("Name <A@Example.COM>"), "10.0.0.2:1"))
		assert.Equal(t, 1, *harness.calls, "the canonical account key must block the credential work")
	})

	t.Run("Edge: the window frees the same key again", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assertRateLimited(t, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1"))

		now = now.Add(time.Minute + time.Second)
		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assert.Equal(t, 2, *harness.calls)
		assert.Equal(t, 1, harness.session.blocks, "only the refused request counts")
	})

	t.Run("Edge: registration is limited like login", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/register", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assertRateLimited(t, harness.post(t, "/register", loginBody("a@example.com"), "10.0.0.1:1"))
	})

	t.Run("Edge: refresh is limited per address only", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/refresh", "", "10.0.0.1:1").Code)
		assertRateLimited(t, harness.post(t, "/refresh", "", "10.0.0.1:1"))

		require.Equal(t, http.StatusOK, harness.post(t, "/refresh", "", "10.0.0.9:1").Code)
		assert.Equal(t, 2, *harness.calls)
	})

	t.Run("Edge: every refused request increments the counter once", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		harness := newRateLimitHarness(ratelimit.New(1, time.Minute, func() time.Time { return now }))

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assertRateLimited(t, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1"))
		assertRateLimited(t, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1"))
		assert.Equal(t, 2, harness.session.blocks)
	})
}

func TestRateLimit_SeparateBuckets(t *testing.T) {
	now := time.Unix(1_000, 0)
	clock := func() time.Time { return now }

	t.Run("Happy path: session requests do not spend the auth or account budget", func(t *testing.T) {
		harness := newScopedRateLimitHarness(
			ratelimit.New(1, time.Minute, clock),
			ratelimit.New(1, time.Minute, clock),
			ratelimit.New(5, time.Minute, clock),
		)

		for i := 1; i <= 4; i++ {
			require.Equal(t, http.StatusOK, harness.post(t, "/refresh", "", "10.0.0.1:1").Code, "refresh %d", i)
		}
		require.Equal(t, http.StatusOK, harness.get(t, "/csrf", "10.0.0.1:1").Code)

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code,
			"the auth and account budgets must still be untouched")
	})

	t.Run("Edge: a nil account limiter leaves the session endpoint address-only", func(t *testing.T) {
		harness := newScopedRateLimitHarness(
			ratelimit.New(10, time.Minute, clock),
			nil,
			ratelimit.New(1, time.Minute, clock),
		)

		require.Equal(t, http.StatusOK, harness.post(t, "/refresh", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assert.Equal(t, http.StatusOK, harness.post(t, "/refresh", loginBody("a@example.com"), "10.0.0.2:1").Code,
			"a nil account limiter must not key on the body")
	})

	t.Run("Edge: the account limiter is independent of the address limiter", func(t *testing.T) {
		harness := newScopedRateLimitHarness(
			ratelimit.New(10, time.Minute, clock),
			ratelimit.New(1, time.Minute, clock),
			ratelimit.New(10, time.Minute, clock),
		)

		require.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("a@example.com"), "10.0.0.1:1").Code)
		assertRateLimited(t, harness.post(t, "/login", loginBody("A@Example.com"), "10.0.0.2:1"))
		assert.Equal(t, http.StatusOK, harness.post(t, "/login", loginBody("b@example.com"), "10.0.0.3:1").Code,
			"another account keeps its own budget")
	})
}
