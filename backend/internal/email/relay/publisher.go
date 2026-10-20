package relay

import (
	"context"
	"time"

	"legnaris/backend/internal/email"
)

// Publisher publishes one encoded message to the broker and reports success
// only once the broker confirmed it accepted the message for routing. The
// backend/internal/email/amqp adapter and a test fake both implement it (EML-02, EML-07).
type Publisher interface {
	Publish(ctx context.Context, msg email.Message) error
}

// Observer receives one publication outcome per intention and the pending
// backlog each pass, so a stalled pipeline can be alerted on (EML-23).
type Observer interface {
	PublishResult(result string)
	BacklogPending(n int)
}

// Publication outcomes reported to the Observer.
const (
	ResultPublished = "published"
	ResultRetried   = "retried"
	ResultFailed    = "failed"
)

// Default relay settings; the process seeds them from configuration and Config
// fills any zero value from these (EML-05).
const (
	DefaultPollInterval   = 1 * time.Second
	DefaultLease          = 30 * time.Second
	DefaultBatch          = 25
	DefaultConfirmTimeout = 5 * time.Second
	DefaultMaxAttempts    = 5
)

// DefaultBackoff is the bounded, increasing tier schedule applied to a failed
// publication; the last entry repeats once the schedule runs out (EML-04).
var DefaultBackoff = []time.Duration{
	5 * time.Second,
	15 * time.Second,
	time.Minute,
	5 * time.Minute,
}

// Config is the relay's tunable behaviour. Every field has a documented default
// that withDefaults applies when it is zero (EML-04, EML-05).
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

// withDefaults fills each zero field from the documented defaults so a
// partially configured relay still runs.
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
