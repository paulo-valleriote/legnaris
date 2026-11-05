package resend

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNextUTCBoundaries proves the quota reset instants land on UTC boundaries,
// including year and leap-day rollovers (LGN-156).
func TestNextUTCBoundaries(t *testing.T) {
	sp := time.FixedZone("UTC-3", -3*60*60)

	tests := []struct {
		name         string
		now          time.Time
		wantMidnight time.Time
		wantMonth    time.Time
	}{
		{
			name:         "Happy path: mid-day resets at the next UTC midnight and month start",
			now:          time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC),
			wantMidnight: time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC),
			wantMonth:    time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "Edge: the last second of the day still resets at the next midnight",
			now:          time.Date(2026, time.March, 10, 23, 59, 59, 0, time.UTC),
			wantMidnight: time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC),
			wantMonth:    time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "Edge: a non-UTC clock is converted before the boundary is computed",
			now:          time.Date(2026, time.March, 31, 22, 0, 0, 0, sp),
			wantMidnight: time.Date(2026, time.April, 2, 0, 0, 0, 0, time.UTC),
			wantMonth:    time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "Edge: December rolls over to January of the next year",
			now:          time.Date(2026, time.December, 31, 10, 0, 0, 0, time.UTC),
			wantMidnight: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantMonth:    time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "Edge: a leap day resets at the next midnight and the next month",
			now:          time.Date(2028, time.February, 29, 8, 0, 0, 0, time.UTC),
			wantMidnight: time.Date(2028, time.March, 1, 0, 0, 0, 0, time.UTC),
			wantMonth:    time.Date(2028, time.March, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantMidnight, nextUTCMidnight(tc.now))
			assert.Equal(t, tc.wantMonth, nextUTCMonth(tc.now))
		})
	}
}

// TestQuotaGate proves the local ceiling opens and closes on the right instants
// and that a later block is never shortened by an earlier one (LGN-156).
func TestQuotaGate(t *testing.T) {
	base := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	t.Run("Edge: a fresh gate is open", func(t *testing.T) {
		gate := newQuotaGate(func() time.Time { return base })
		_, blocked := gate.blocked()
		assert.False(t, blocked)
	})

	t.Run("Happy path: a closed gate stays closed until its instant", func(t *testing.T) {
		now := base
		gate := newQuotaGate(func() time.Time { return now })
		until := nextUTCMidnight(base)
		gate.exhaust(until)

		got, blocked := gate.blocked()
		require.True(t, blocked)
		assert.Equal(t, until, got)

		now = until.Add(-time.Nanosecond)
		_, blocked = gate.blocked()
		assert.True(t, blocked, "the gate must hold right before the reset")

		now = until
		_, blocked = gate.blocked()
		assert.False(t, blocked, "the gate must reopen exactly at the reset")
	})

	t.Run("Edge: a daily block never shortens a monthly block", func(t *testing.T) {
		gate := newQuotaGate(func() time.Time { return base })
		monthly := nextUTCMonth(base)
		gate.exhaust(monthly)
		gate.exhaust(nextUTCMidnight(base))

		got, blocked := gate.blocked()
		require.True(t, blocked)
		assert.Equal(t, monthly, got)
	})

	t.Run("Edge: a daily block extends a shorter earlier block", func(t *testing.T) {
		gate := newQuotaGate(func() time.Time { return base })
		gate.exhaust(base.Add(time.Minute))
		monthly := nextUTCMonth(base)
		gate.exhaust(monthly)

		got, _ := gate.blocked()
		assert.Equal(t, monthly, got)
	})

	headerTests := []struct {
		name        string
		header      string
		wantBlocked bool
	}{
		{name: "Edge: an absent header never closes the gate", header: "", wantBlocked: false},
		{name: "Edge: a malformed header never closes the gate", header: "abc", wantBlocked: false},
		{name: "Edge: a negative-looking fractional header is ignored", header: "1.5", wantBlocked: false},
		{name: "Edge: a positive remaining count keeps the gate open", header: "7", wantBlocked: false},
		{name: "Edge: a zero remaining count closes the gate until midnight", header: "0", wantBlocked: true},
		{name: "Edge: a negative remaining count closes the gate until midnight", header: "-3", wantBlocked: true},
	}

	for _, tc := range headerTests {
		t.Run(tc.name, func(t *testing.T) {
			gate := newQuotaGate(func() time.Time { return base })
			gate.observeDailyHeader(tc.header)

			until, blocked := gate.blocked()
			assert.Equal(t, tc.wantBlocked, blocked)
			if tc.wantBlocked {
				assert.Equal(t, nextUTCMidnight(base), until)
			}
		})
	}

	t.Run("Edge: concurrent exhaust and blocked calls stay race-free", func(t *testing.T) {
		gate := newQuotaGate(func() time.Time { return base })

		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				gate.exhaust(nextUTCMidnight(base))
			}()
			go func() {
				defer wg.Done()
				_, _ = gate.blocked()
			}()
		}
		wg.Wait()

		_, blocked := gate.blocked()
		assert.True(t, blocked)
	})
}

// quotaServer answers every request with the given status, body and headers
// and counts the provider calls, so the tests can prove the local gate spares
// the provider.
type quotaServer struct {
	mu        sync.Mutex
	hits      int
	responses []quotaResponse
}

type quotaResponse struct {
	status  int
	body    string
	headers map[string]string
}

func (q *quotaServer) handler(w http.ResponseWriter, _ *http.Request) {
	q.mu.Lock()
	idx := q.hits
	q.hits++
	resp := q.responses[len(q.responses)-1]
	if idx < len(q.responses) {
		resp = q.responses[idx]
	}
	q.mu.Unlock()

	for k, v := range resp.headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.status)
	_, _ = w.Write([]byte(resp.body))
}

func (q *quotaServer) callCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.hits
}

// TestSender_QuotaAndRateLimit proves each provider 429 and 5xx class maps to
// the right sentinel and that a spent quota closes the local gate (LGN-156).
func TestSender_QuotaAndRateLimit(t *testing.T) {
	const dailyBody = `{"statusCode":429,"name":"daily_quota_exceeded","message":"quota"}`
	const monthlyBody = `{"statusCode":429,"name":"monthly_quota_exceeded","message":"quota"}`
	const rateBody = `{"statusCode":429,"name":"rate_limit_exceeded","message":"slow down"}`

	tests := []struct {
		name          string
		responses     []quotaResponse
		wantErr       error
		wantNoErr     bool
		wantHits      int
		wantExhausted bool
		wantRateKind  bool
		permanent     bool
	}{
		{
			name:          "Error: a spent daily quota is quota-exhausted and transient",
			responses:     []quotaResponse{{status: http.StatusTooManyRequests, body: dailyBody}},
			wantErr:       email.ErrQuotaExhausted,
			wantHits:      1,
			wantExhausted: true,
		},
		{
			name:          "Error: a spent monthly quota is quota-exhausted and transient",
			responses:     []quotaResponse{{status: http.StatusTooManyRequests, body: monthlyBody}},
			wantErr:       email.ErrQuotaExhausted,
			wantHits:      1,
			wantExhausted: true,
		},
		{
			name:         "Error: a rate limit without Retry-After is rate-limited and transient",
			responses:    []quotaResponse{{status: http.StatusTooManyRequests, body: rateBody}},
			wantErr:      email.ErrRateLimited,
			wantHits:     1,
			wantRateKind: true,
		},
		{
			name:         "Error: a 429 with a non-JSON body is treated as a rate limit",
			responses:    []quotaResponse{{status: http.StatusTooManyRequests, body: "slow"}},
			wantErr:      email.ErrRateLimited,
			wantHits:     1,
			wantRateKind: true,
		},
		{
			name: "Happy path: a short Retry-After is retried once and succeeds",
			responses: []quotaResponse{
				{status: http.StatusTooManyRequests, body: rateBody, headers: map[string]string{"Retry-After": "0"}},
				{status: http.StatusOK, body: `{"id":"provider-id"}`},
			},
			wantNoErr: true,
			wantHits:  2,
		},
		{
			name: "Error: a short Retry-After that fails again is rate-limited after one retry",
			responses: []quotaResponse{
				{status: http.StatusTooManyRequests, body: rateBody, headers: map[string]string{"Retry-After": "0"}},
				{status: http.StatusTooManyRequests, body: rateBody, headers: map[string]string{"Retry-After": "0"}},
			},
			wantErr:      email.ErrRateLimited,
			wantHits:     2,
			wantRateKind: true,
		},
		{
			name:         "Edge: a Retry-After above the in-place cap is not retried",
			responses:    []quotaResponse{{status: http.StatusTooManyRequests, body: rateBody, headers: map[string]string{"Retry-After": "3600"}}},
			wantErr:      email.ErrRateLimited,
			wantHits:     1,
			wantRateKind: true,
		},
		{
			name:         "Edge: a malformed Retry-After is not retried",
			responses:    []quotaResponse{{status: http.StatusTooManyRequests, body: rateBody, headers: map[string]string{"Retry-After": "soon"}}},
			wantErr:      email.ErrRateLimited,
			wantHits:     1,
			wantRateKind: true,
		},
		{
			name:          "Edge: a daily quota is never retried in place even with a short Retry-After",
			responses:     []quotaResponse{{status: http.StatusTooManyRequests, body: dailyBody, headers: map[string]string{"Retry-After": "0"}}},
			wantErr:       email.ErrQuotaExhausted,
			wantHits:      1,
			wantExhausted: true,
		},
		{
			name:      "Error: a 500 is transient and not quota-exhausted",
			responses: []quotaResponse{{status: http.StatusInternalServerError, body: `{}`}},
			wantErr:   email.ErrTransient,
			wantHits:  1,
		},
		{
			name:      "Error: a 400 is permanent and never touches the gate",
			responses: []quotaResponse{{status: http.StatusBadRequest, body: `{"name":"daily_quota_exceeded"}`}},
			wantErr:   email.ErrPermanent,
			wantHits:  1,
			permanent: true,
		},
		{
			name:          "Edge: a zero daily header on a 500 closes the gate",
			responses:     []quotaResponse{{status: http.StatusInternalServerError, body: `{}`, headers: map[string]string{dailyQuotaHeader: "0"}}},
			wantErr:       email.ErrTransient,
			wantHits:      1,
			wantExhausted: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &quotaServer{responses: tc.responses}
			sender := newTestSender(t, provider.handler, time.Second)

			err := sender.Send(t.Context(), testOutbound())

			if tc.wantNoErr {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				if tc.permanent {
					assert.NotErrorIs(t, err, email.ErrTransient)
				} else {
					assert.ErrorIs(t, err, email.ErrTransient, "every quota and rate failure must stay retryable")
				}
			}
			assert.Equal(t, tc.wantHits, provider.callCount())

			if tc.wantRateKind {
				assert.NotErrorIs(t, err, email.ErrQuotaExhausted)
			}
			if tc.wantExhausted {
				_, blocked := sender.quota.blocked()
				assert.True(t, blocked)
			}
		})
	}
}

// TestSender_ExhaustedGateSkipsProvider proves that once a quota is spent, later
// sends fail without reaching the provider until the reset instant (LGN-156).
func TestSender_ExhaustedGateSkipsProvider(t *testing.T) {
	const dailyBody = `{"statusCode":429,"name":"daily_quota_exceeded"}`

	provider := &quotaServer{responses: []quotaResponse{
		{status: http.StatusTooManyRequests, body: dailyBody},
		{status: http.StatusOK, body: `{"id":"provider-id"}`},
	}}
	sender := newTestSender(t, provider.handler, time.Second)

	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	sender.quota = newQuotaGate(func() time.Time { return now })

	require.ErrorIs(t, sender.Send(t.Context(), testOutbound()), email.ErrQuotaExhausted)
	require.Equal(t, 1, provider.callCount())

	t.Run("Edge: a send while the quota is spent never reaches the provider", func(t *testing.T) {
		err := sender.Send(t.Context(), testOutbound())
		require.ErrorIs(t, err, email.ErrQuotaExhausted)
		require.ErrorIs(t, err, email.ErrTransient)
		assert.Equal(t, 1, provider.callCount())
	})

	t.Run("Happy path: the first send after the reset reaches the provider again", func(t *testing.T) {
		now = nextUTCMidnight(now)
		require.NoError(t, sender.Send(t.Context(), testOutbound()))
		assert.Equal(t, 2, provider.callCount())
	})
}

// TestSender_RetryAfterHonorsContext proves an in-place Retry-After wait stops
// when the caller's context ends, instead of holding the send (LGN-156).
func TestSender_RetryAfterHonorsContext(t *testing.T) {
	provider := &quotaServer{responses: []quotaResponse{
		{status: http.StatusTooManyRequests, body: `{"name":"rate_limit_exceeded"}`, headers: map[string]string{"Retry-After": "2"}},
		{status: http.StatusOK, body: `{"id":"provider-id"}`},
	}}
	sender := newTestSender(t, provider.handler, 5*time.Second)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err := sender.Send(ctx, testOutbound())
	require.Error(t, err)
	assert.ErrorIs(t, err, email.ErrRateLimited)
	assert.Equal(t, 1, provider.callCount(), "a cancelled wait must not retry")
}
