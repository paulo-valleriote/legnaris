package resend

import (
	"strconv"
	"sync"
	"time"

	"legnaris/backend/shared/json"
)

const (
	quotaNameDaily   = "daily_quota_exceeded"
	quotaNameMonthly = "monthly_quota_exceeded"

	dailyQuotaHeader = "x-resend-daily-quota"
)

type quotaGate struct {
	mu           sync.Mutex
	blockedUntil time.Time
	now          func() time.Time
}

func newQuotaGate(now func() time.Time) *quotaGate {
	return &quotaGate{now: now}
}

func (g *quotaGate) blocked() (time.Time, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.blockedUntil.After(g.now()) {
		return g.blockedUntil, true
	}
	return time.Time{}, false
}

func (g *quotaGate) exhaust(until time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if until.After(g.blockedUntil) {
		g.blockedUntil = until
	}
}

func (g *quotaGate) observeDailyHeader(header string) {
	remaining, err := strconv.Atoi(header)
	if err != nil || remaining > 0 {
		return
	}
	g.exhaust(nextUTCMidnight(g.now()))
}

func providerQuotaName(body string) string {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.DecodeBytes([]byte(body), &payload); err != nil {
		return ""
	}
	return payload.Name
}

func nextUTCMidnight(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
}

func nextUTCMonth(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}
