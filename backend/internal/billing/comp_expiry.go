package billing

import (
	"context"
	"time"

	"legnaris/backend/internal/sweeper"
)

const (
	CompExpiryQueueName = "billing_comp_expiry"
	CompExpiryInterval  = time.Hour
	compExpiryJobID     = "comp_expiry"
)

type CompExpiryLedger interface {
	CancelExpiredComps(ctx context.Context, now time.Time) (int, error)
}

type compExpiryStore struct {
	ledger CompExpiryLedger
}

func (s compExpiryStore) Claim(context.Context, string, time.Time, time.Duration, int) ([]sweeper.Job[struct{}], error) {
	return []sweeper.Job[struct{}]{{ID: compExpiryJobID}}, nil
}

func (s compExpiryStore) Reschedule(context.Context, string, time.Time, time.Time, string) error {
	return nil
}

func (s compExpiryStore) MarkDead(context.Context, string, time.Time, string) error {
	return nil
}

func (s compExpiryStore) OldestPending(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func NewCompExpiryQueue(ledger CompExpiryLedger) (sweeper.Queue, error) {
	return sweeper.NewQueue(sweeper.QueueSpec[struct{}]{
		Name:       CompExpiryQueueName,
		Interval:   CompExpiryInterval,
		AttemptCap: 1,
		Batch:      1,
		Store:      compExpiryStore{ledger: ledger},
		Handle: func(ctx context.Context, _ sweeper.Job[struct{}], now time.Time) error {
			_, err := ledger.CancelExpiredComps(ctx, now)
			return err
		},
	})
}
