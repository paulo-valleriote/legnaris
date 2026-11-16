package billing

import (
	"context"
	"time"
)

type Summary struct {
	Status            Status
	CurrentPeriodEnd  *time.Time
	CancelAtPeriodEnd bool
	PaymentIssue      bool
}

type StatusReader struct {
	ledger Ledger
}

func NewStatusReader(ledger Ledger) *StatusReader {
	return &StatusReader{ledger: ledger}
}

func (r *StatusReader) Summary(ctx context.Context, userID string) (Summary, error) {
	subs, err := r.ledger.SubscriptionsOf(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	if len(subs) == 0 {
		return Summary{}, nil
	}
	chosen := subs[0]
	for _, sub := range subs[1:] {
		if relevance(sub) > relevance(chosen) || (relevance(sub) == relevance(chosen) && sub.UpdatedAt.After(chosen.UpdatedAt)) {
			chosen = sub
		}
	}
	return Summary{
		Status:            chosen.Status,
		CurrentPeriodEnd:  chosen.CurrentPeriodEnd,
		CancelAtPeriodEnd: chosen.CancelAtPeriodEnd,
		PaymentIssue:      chosen.Status == StatusPastDue,
	}, nil
}

func relevance(sub Subscription) int {
	switch sub.Status {
	case StatusActive:
		return 3
	case StatusPastDue:
		return 2
	case StatusPending:
		return 1
	}
	return 0
}
