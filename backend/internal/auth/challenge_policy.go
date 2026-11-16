package auth

import (
	"math"
	"time"
)

type ChallengePolicy struct {
	Lifetime         time.Duration
	Ceiling          time.Duration
	ResendCooldown   time.Duration
	MaxAttempts      int
	DailySendCap     int
	ResendCapRatio   float64
	UserWindow       time.Duration
	MaxPerUserWindow int
}

func (p ChallengePolicy) Deadline(createdAt, now time.Time) time.Time {
	deadline := now.Add(p.Lifetime)
	if ceiling := createdAt.Add(p.Ceiling); deadline.After(ceiling) {
		return ceiling
	}
	return deadline
}

func (p ChallengePolicy) DeadlineWithin(ceilingAt, now time.Time) time.Time {
	deadline := now.Add(p.Lifetime)
	if deadline.After(ceilingAt) {
		return ceilingAt
	}
	return deadline
}

func (p ChallengePolicy) CooldownElapsed(lastSentAt, now time.Time) bool {
	return !now.Before(lastSentAt.Add(p.ResendCooldown))
}

func (p ChallengePolicy) DayStart(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func (p ChallengePolicy) ResendDayCap() int {
	return int(math.Floor(float64(p.DailySendCap) * p.ResendCapRatio))
}
