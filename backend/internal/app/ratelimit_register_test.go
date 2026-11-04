package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/platform/ratelimit"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const registerTestWindow = time.Hour

// registerHarness routes POST /register through registerRateLimit and counts
// the handler calls, so a test can prove a refused request did no account work.
type registerHarness struct {
	echo     *echo.Echo
	calls    *int
	observer *countingObserver
}

func newRegisterHarness(ip, email ratelimit.Allower) *registerHarness {
	observer := &countingObserver{}
	calls := 0
	handler := func(c *echo.Context) error {
		calls++
		return c.NoContent(http.StatusCreated)
	}

	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.POST("/register", handler, registerRateLimit(ip, email, observer))

	return &registerHarness{echo: e, calls: &calls, observer: observer}
}

func (h *registerHarness) post(t *testing.T, body, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remoteAddr

	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)
	return rec
}

// registerBody builds a registration payload; only the e-mail matters to the guard.
func registerBody(email string) string {
	return `{"email":"` + email + `","username":"player","password":"sup3r-secret"}`
}

// clockedLimiter builds an in-process limiter driven by a shared test clock.
func clockedLimiter(max int, now *time.Time) ratelimit.Allower {
	return ratelimit.New(max, registerTestWindow, func() time.Time { return *now })
}

// failingStore stands in for an unreachable Redis: every decision errors out.
type failingStore struct{}

func (failingStore) Hit(context.Context, string, int, time.Duration) (bool, time.Duration, error) {
	return false, 0, errors.New("redis: connection refused")
}

func TestRegisterRateLimit_IPBudget(t *testing.T) {
	tests := []struct {
		name      string
		ipMax     int
		attempts  int
		wantCalls int
		wantLast  int
	}{
		{name: "Happy path: five registrations from one address are admitted", ipMax: 5, attempts: 5, wantCalls: 5, wantLast: http.StatusCreated},
		{name: "Edge: exactly the limit is admitted and one above is refused", ipMax: 5, attempts: 6, wantCalls: 5, wantLast: http.StatusTooManyRequests},
		{name: "Edge: a single allowed attempt leaves the next one admitted", ipMax: 1, attempts: 1, wantCalls: 1, wantLast: http.StatusCreated},
		{name: "Edge: a cap of one refuses the second attempt", ipMax: 1, attempts: 2, wantCalls: 1, wantLast: http.StatusTooManyRequests},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_000, 0)
			h := newRegisterHarness(clockedLimiter(tc.ipMax, &now), clockedLimiter(100, &now))

			var last *httptest.ResponseRecorder
			for i := 0; i < tc.attempts; i++ {
				last = h.post(t, registerBody("player"+string(rune('a'+i))+"@example.com"), "198.51.100.7:4000")
			}

			assert.Equal(t, tc.wantLast, last.Code, last.Body.String())
			assert.Equal(t, tc.wantCalls, *h.calls, "only admitted registrations reach the handler")
			if tc.wantLast == http.StatusTooManyRequests {
				assertRateLimited(t, last)
				assert.Equal(t, 1, h.observer.blocks, "the refusal is counted exactly once")
			}
		})
	}

	t.Run("Happy path: a refusal answers 429 with Retry-After from the IP window", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(1, &now), clockedLimiter(100, &now))

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("a@example.com"), "198.51.100.7:4000").Code)

		rec := h.post(t, registerBody("b@example.com"), "198.51.100.7:4000")
		assertRateLimited(t, rec)
		assert.Equal(t, "3600", rec.Header().Get(echo.HeaderRetryAfter))
	})

	t.Run("Edge: another address keeps its own budget", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(1, &now), clockedLimiter(100, &now))

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("a@example.com"), "198.51.100.7:4000").Code)
		require.Equal(t, http.StatusTooManyRequests, h.post(t, registerBody("b@example.com"), "198.51.100.7:4000").Code)

		other := h.post(t, registerBody("c@example.com"), "198.51.100.8:4000")
		assert.Equal(t, http.StatusCreated, other.Code)
	})

	t.Run("Edge: the IP window frees once its oldest attempt ages out", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(1, &now), clockedLimiter(100, &now))

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("a@example.com"), "198.51.100.7:4000").Code)
		require.Equal(t, http.StatusTooManyRequests, h.post(t, registerBody("b@example.com"), "198.51.100.7:4000").Code)

		now = now.Add(registerTestWindow)
		assert.Equal(t, http.StatusCreated, h.post(t, registerBody("c@example.com"), "198.51.100.7:4000").Code)
	})
}

func TestRegisterRateLimit_EmailBudget(t *testing.T) {
	t.Run("Happy path: three registrations for one mailbox are admitted and the fourth is refused", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(100, &now), clockedLimiter(3, &now))

		for i := 1; i <= 3; i++ {
			require.Equal(t, http.StatusCreated, h.post(t, registerBody("player@example.com"), "198.51.100.7:1").Code, "attempt %d", i)
		}

		assertRateLimited(t, h.post(t, registerBody("player@example.com"), "198.51.100.7:1"))
		assert.Equal(t, 3, *h.calls)
	})

	t.Run("Edge: equivalent spellings of one mailbox share the budget across addresses", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(100, &now), clockedLimiter(3, &now))

		spellings := []string{
			"Player@Example.com",
			"  player@example.com ",
			"PLAYER@EXAMPLE.COM",
		}
		for i, email := range spellings {
			require.Equal(t, http.StatusCreated, h.post(t, registerBody(email), "198.51.100."+string(rune('1'+i))+":1").Code, email)
		}

		assertRateLimited(t, h.post(t, registerBody("Name <player@example.com>"), "203.0.113.9:1"))
	})

	t.Run("Edge: a +tag address is a different mailbox with its own budget", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(100, &now), clockedLimiter(1, &now))

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("player@example.com"), "198.51.100.7:1").Code)
		require.Equal(t, http.StatusTooManyRequests, h.post(t, registerBody("player@example.com"), "198.51.100.7:1").Code)

		assert.Equal(t, http.StatusCreated, h.post(t, registerBody("player+news@example.com"), "198.51.100.7:1").Code)
	})

	t.Run("Edge: the e-mail refusal is independent of the address and never reaches the handler", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(100, &now), clockedLimiter(1, &now))

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("taken@example.com"), "198.51.100.7:1").Code)

		refusedSameAddress := h.post(t, registerBody("taken@example.com"), "198.51.100.7:1")
		refusedOtherAddress := h.post(t, registerBody("taken@example.com"), "198.51.100.200:1")

		assertRateLimited(t, refusedSameAddress)
		assertRateLimited(t, refusedOtherAddress)
		assert.Equal(t, refusedSameAddress.Code, refusedOtherAddress.Code)
		assert.Equal(t, 1, *h.calls, "a refused e-mail never reaches the account store")
	})

	t.Run("Edge: the e-mail window frees once its oldest attempt ages out", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(100, &now), clockedLimiter(1, &now))

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("player@example.com"), "198.51.100.7:1").Code)
		require.Equal(t, http.StatusTooManyRequests, h.post(t, registerBody("player@example.com"), "198.51.100.7:1").Code)

		now = now.Add(registerTestWindow + time.Second)
		assert.Equal(t, http.StatusCreated, h.post(t, registerBody("player@example.com"), "198.51.100.7:1").Code)
	})

	t.Run("Edge: a refused address does not spend the e-mail budget", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		h := newRegisterHarness(clockedLimiter(1, &now), clockedLimiter(2, &now))

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("a@example.com"), "198.51.100.7:1").Code)
		require.Equal(t, http.StatusTooManyRequests, h.post(t, registerBody("b@example.com"), "198.51.100.7:1").Code)

		// Only the IP refused the second attempt, so the mailbox still has one slot.
		assert.Equal(t, http.StatusCreated, h.post(t, registerBody("b@example.com"), "198.51.100.8:1").Code)
	})
}

func TestRegisterRateLimit_BodyWithoutEmail(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "Edge: an empty object skips the e-mail budget", body: `{}`},
		{name: "Edge: a blank e-mail skips the e-mail budget", body: `{"email":"   "}`},
		{name: "Edge: a body that is not JSON skips the e-mail budget", body: `not json`},
		{name: "Edge: an empty body skips the e-mail budget", body: ``},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_000, 0)
			h := newRegisterHarness(clockedLimiter(100, &now), clockedLimiter(1, &now))

			for i := 0; i < 3; i++ {
				require.Equal(t, http.StatusCreated, h.post(t, tc.body, "198.51.100.7:1").Code, "request %d", i+1)
			}
			assert.Equal(t, 3, *h.calls)
		})
	}
}

func TestRegisterRateLimit_StoreFailure(t *testing.T) {
	t.Run("Error: an unreachable store fails open to the per-process cap", func(t *testing.T) {
		var failures int
		ip := ratelimit.NewShared(failingStore{}, 2, registerTestWindow, time.Now, func(error) { failures++ })
		email := ratelimit.NewShared(failingStore{}, 100, registerTestWindow, time.Now, func(error) { failures++ })
		h := newRegisterHarness(ip, email)

		require.Equal(t, http.StatusCreated, h.post(t, registerBody("a@example.com"), "198.51.100.7:1").Code)
		require.Equal(t, http.StatusCreated, h.post(t, registerBody("b@example.com"), "198.51.100.7:1").Code)

		assertRateLimited(t, h.post(t, registerBody("c@example.com"), "198.51.100.7:1"))
		assert.Equal(t, 2, *h.calls, "the local fallback still caps registrations while Redis is down")
		assert.Positive(t, failures, "every store failure is reported to the caller")
	})
}
