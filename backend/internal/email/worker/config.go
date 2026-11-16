package worker

import (
	"time"

	"legnaris/backend/internal/platform/breaker"
)

const (
	DefaultPrefetch     = 10
	DefaultMaxAttempts  = 5
	DefaultDrainTimeout = 10 * time.Second

	RegisterRetryDelay = 5 * time.Second
)

type Config struct {
	Prefetch             int
	MaxAttempts          int
	DrainTimeout         time.Duration
	Now                  func() time.Time
	RegisterRetryDelay   time.Duration
	BreakerThreshold     int
	BreakerProbeInterval time.Duration
}

func (c Config) withDefaults() Config {
	if c.Prefetch <= 0 {
		c.Prefetch = DefaultPrefetch
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}
	if c.DrainTimeout <= 0 {
		c.DrainTimeout = DefaultDrainTimeout
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.RegisterRetryDelay <= 0 {
		c.RegisterRetryDelay = RegisterRetryDelay
	}
	if c.BreakerThreshold <= 0 {
		c.BreakerThreshold = breaker.DefaultBreakerThreshold
	}
	if c.BreakerProbeInterval <= 0 {
		c.BreakerProbeInterval = breaker.DefaultBreakerProbeInterval
	}
	return c
}
