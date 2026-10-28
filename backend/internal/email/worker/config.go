package worker

import "time"

// Default worker settings; Config fills any zero value from them (WKR-14,
// WKR-16).
const (
	DefaultPrefetch     = 10
	DefaultMaxAttempts  = 5
	DefaultDrainTimeout = 10 * time.Second

	// RegisterRetryDelay is the fixed pause after a failed consumer
	// registration. It is a constant on purpose: a configurable delay would let
	// an operator turn the retry into a hot loop against a refusing broker.
	RegisterRetryDelay = 5 * time.Second
)

// Config is the worker's tunable behaviour.
type Config struct {
	// Prefetch bounds the messages held in flight at once.
	Prefetch int
	// MaxAttempts is the send-attempt cap.
	MaxAttempts int
	// DrainTimeout bounds the wait for in-flight messages on stop; it follows
	// the provider send timeout.
	DrainTimeout time.Duration
	// Now is the clock the expiry decision reads.
	Now func() time.Time
	// RegisterRetryDelay overrides the registration pause; tests only.
	RegisterRetryDelay time.Duration
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
	return c
}
