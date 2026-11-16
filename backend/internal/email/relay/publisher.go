package relay

import (
	"context"
	"time"

	"legnaris/backend/internal/email"
)

type Publisher interface {
	Publish(ctx context.Context, msg email.Message) error
}

type Observer interface {
	PublishResult(result string)
	BacklogPending(n int)
}

const (
	ResultPublished = "published"
	ResultRetried   = "retried"
	ResultFailed    = "failed"
)

const (
	DefaultPollInterval   = 1 * time.Second
	DefaultLease          = 30 * time.Second
	DefaultBatch          = 25
	DefaultConfirmTimeout = 5 * time.Second
	DefaultMaxAttempts    = 5
)

var DefaultBackoff = []time.Duration{
	5 * time.Second,
	15 * time.Second,
	time.Minute,
	5 * time.Minute,
}

type Config struct {
	PollInterval   time.Duration
	Lease          time.Duration
	Batch          int
	ConfirmTimeout time.Duration
	MaxAttempts    int
	Backoff        []time.Duration
	Now            func() time.Time
	ClaimerID      string
}

func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultPollInterval
	}
	if c.Lease <= 0 {
		c.Lease = DefaultLease
	}
	if c.Batch <= 0 {
		c.Batch = DefaultBatch
	}
	if c.ConfirmTimeout <= 0 {
		c.ConfirmTimeout = DefaultConfirmTimeout
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}
	if len(c.Backoff) == 0 {
		c.Backoff = DefaultBackoff
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}
