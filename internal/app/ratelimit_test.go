package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/internal/platform/ratelimit"
	"legnaris/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingObserver records how many requests the guard refused (AUTH-20).
type countingObserver struct {
	blocks int
}

func (o *countingObserver) RateLimitBlocked() { o.blocks++ }

// rateLimitHarness is a router whose auth routes sit behind the guard and whose
// handler counts invocations, so a test can prove no credential work happened.
type rateLimitHarness struct {
	echo    *echo.Echo
	calls   *int
	session *countingObserver
}

func newRateLimitHarness(limiter *ratelimit.Limiter) *rateLimitHarness {
	observer := &countingObserver{}
	calls := 0
	handler := func(c *echo.Context) error {
		calls++
		return c.NoContent(http.StatusOK)
	}

	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.POST("/login", handler, rateLimit(limiter, observer, true))
	e.POST("/register", handler, rateLimit(limiter, observer, true))
	e.POST("/refresh", handler, rateLimit(limiter, observer, false))

	return &rateLimitHarness{echo: e, calls: &calls, session: observer}
}

func (h *rateLimitHarness) post(t *testing.T, path, body, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)
	return rec
}

func loginBody(email string) string {
	return `{"email":"` + email + `","password":"sup3r-secret"}`
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

		// A different address is admitted even though the body says nothing about
		// an account; the IP key frees with the address (AUTH-17).
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
