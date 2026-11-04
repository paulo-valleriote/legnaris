package resend

import (
	"strconv"
	"sync"
	"time"

	"legnaris/backend/shared/json"
)

// Provider quota names carried in the body of a 429 response (LGN-156).
const (
	quotaNameDaily   = "daily_quota_exceeded"
	quotaNameMonthly = "monthly_quota_exceeded"

	// dailyQuotaHeader carries the sends the free plan still allows today.
	dailyQuotaHeader = "x-resend-daily-quota"
)

// quotaGate is the local ceiling: once the provider reports a spent quota, sends
// stop reaching the provider until the quota resets, so a dead day does not burn
// one request per retry tier.
type quotaGate struct {
	mu           sync.Mutex
	blockedUntil time.Time
	now          func() time.Time
}

func newQuotaGate(now func() time.Time) *quotaGate {
	return &quotaGate{now: now}
}

// blocked reports the instant sends resume when the gate is closed.
func (g *quotaGate) blocked() (time.Time, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.blockedUntil.After(g.now()) {
		return g.blockedUntil, true
	}
	return time.Time{}, false
}

// exhaust closes the gate until the given instant. A later instant wins, so a
// monthly block is never shortened by a daily one.
func (g *quotaGate) exhaust(until time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if until.After(g.blockedUntil) {
		g.blockedUntil = until
	}
}

// observeDailyHeader closes the gate for the rest of the day when the provider
// reports no sends left. A zero or malformed header is never a reason to close.
func (g *quotaGate) observeDailyHeader(header string) {
	remaining, err := strconv.Atoi(header)
	if err != nil || remaining > 0 {
		return
	}
	g.exhaust(nextUTCMidnight(g.now()))
}

// providerQuotaName returns the quota name of a 429 body, or "" when the body is
// not a quota response, which means the request rate was exceeded.
func providerQuotaName(body string) string {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.DecodeBytes([]byte(body), &payload); err != nil {
		return ""
	}
	return payload.Name
}

// nextUTCMidnight is the instant the daily quota resets.
func nextUTCMidnight(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
}

// nextUTCMonth is the instant the monthly quota resets.
func nextUTCMonth(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}
