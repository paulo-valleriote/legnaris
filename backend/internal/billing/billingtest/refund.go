package billingtest

import (
	"context"
	"slices"
	"time"

	"legnaris/backend/internal/billing"
)

func (l *Ledger) RequestRefund(_ context.Context, id string, now time.Time) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	sub, ok := l.st.subs[id]
	if !ok || sub.Status != billing.StatusActive || (sub.RefundState != "" && sub.RefundState != billing.RefundFailed) {
		return false, nil
	}
	for _, other := range l.st.subs {
		if other.UserID == sub.UserID && other.RefundUsedAt != nil {
			return false, billing.ErrRefundAlreadyUsed
		}
	}
	sub.RefundState = billing.RefundRequested
	sub.RefundRequestedAt = &now
	sub.RefundUsedAt = &now
	sub.RefundNextAttempt = now
	sub.RefundAttempts = 0
	sub.RefundDead = false
	sub.RefundLastError = ""
	sub.UpdatedAt = now
	l.st.subs[id] = sub
	return true, nil
}

func (l *Ledger) ClaimRefunds(_ context.Context, now time.Time, lease time.Duration, limit int) ([]billing.Subscription, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var due []billing.Subscription
	for _, sub := range l.st.subs {
		if (sub.RefundState == billing.RefundRequested || sub.RefundState == billing.RefundRefunded) &&
			!sub.RefundDead && !sub.RefundNextAttempt.After(now) {
			due = append(due, sub)
		}
	}
	slices.SortFunc(due, func(a, b billing.Subscription) int {
		return a.RefundRequestedAt.Compare(*b.RefundRequestedAt)
	})
	if len(due) > limit {
		due = due[:limit]
	}
	for i := range due {
		due[i].RefundNextAttempt = now.Add(lease)
		l.st.subs[due[i].ID] = due[i]
	}
	return due, nil
}

func (l *Ledger) RescheduleRefund(_ context.Context, id string, nextAt time.Time, reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	sub, ok := l.st.subs[id]
	if !ok {
		return billing.ErrRefundStateChanged
	}
	sub.RefundNextAttempt = nextAt
	sub.RefundAttempts++
	sub.RefundLastError = reason
	l.st.subs[id] = sub
	return nil
}

func (l *Ledger) SetRefundState(_ context.Context, id string, from, to billing.RefundState) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	sub, ok := l.st.subs[id]
	if !ok || sub.RefundState != from {
		return billing.ErrRefundStateChanged
	}
	sub.RefundState = to
	l.st.subs[id] = sub
	return nil
}

func (l *Ledger) FailRefund(_ context.Context, id string, reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	sub, ok := l.st.subs[id]
	if !ok {
		return nil
	}
	sub.RefundDead = true
	sub.RefundLastError = reason
	if sub.RefundState == billing.RefundRequested {
		sub.RefundState = billing.RefundFailed
		sub.RefundUsedAt = nil
	}
	l.st.subs[id] = sub
	return nil
}

func (l *Ledger) OldestRefundRequested(_ context.Context) (time.Time, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var oldest time.Time
	found := false
	for _, sub := range l.st.subs {
		if sub.RefundState != billing.RefundRequested || sub.RefundRequestedAt == nil {
			continue
		}
		if !found || sub.RefundRequestedAt.Before(oldest) {
			oldest = *sub.RefundRequestedAt
			found = true
		}
	}
	return oldest, found, nil
}
