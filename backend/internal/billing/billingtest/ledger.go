package billingtest

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/idgen"
)

type state struct {
	events   map[string]billing.Event
	locks    map[string]lockState
	subs     map[string]billing.Subscription
	accounts map[string]billing.AccountPlan
	outbox   []outbox.Entry
}

type lockState struct {
	claimer    string
	lockedThru time.Time
}

type Ledger struct {
	mu sync.Mutex
	st state
}

func NewLedger() *Ledger {
	return &Ledger{st: emptyState()}
}

func emptyState() state {
	return state{
		events:   map[string]billing.Event{},
		locks:    map[string]lockState{},
		subs:     map[string]billing.Subscription{},
		accounts: map[string]billing.AccountPlan{},
	}
}

func cloneState(s state) state {
	return state{
		events:   maps.Clone(s.events),
		locks:    maps.Clone(s.locks),
		subs:     maps.Clone(s.subs),
		accounts: maps.Clone(s.accounts),
		outbox:   slices.Clone(s.outbox),
	}
}

func (l *Ledger) snapshot() state {
	l.mu.Lock()
	defer l.mu.Unlock()
	return cloneState(l.st)
}

func (l *Ledger) restore(s state) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.st = s
}

func (l *Ledger) AddAccount(userID string, acc billing.AccountPlan) {
	l.mu.Lock()
	defer l.mu.Unlock()
	acc.Exists = true
	l.st.accounts[userID] = acc
}

func (l *Ledger) RemoveAccount(userID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.st.accounts, userID)
}

func (l *Ledger) Account(userID string) (billing.AccountPlan, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	acc, ok := l.st.accounts[userID]
	return acc, ok
}

func (l *Ledger) Event(id string) (billing.Event, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ev, ok := l.st.events[id]
	return ev, ok
}

func (l *Ledger) Events() []billing.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]billing.Event, 0, len(l.st.events))
	for _, ev := range l.st.events {
		out = append(out, ev)
	}
	return out
}

func (l *Ledger) Subscriptions(userID string) []billing.Subscription {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []billing.Subscription
	for _, sub := range l.st.subs {
		if sub.UserID == userID {
			out = append(out, sub)
		}
	}
	return out
}

func (l *Ledger) SeedSubscription(sub billing.Subscription) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.st.subs[sub.ID] = sub
}

func (l *Ledger) OutboxEntries() []outbox.Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.st.outbox)
}

func (l *Ledger) InsertEvent(_ context.Context, ev billing.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, existing := range l.st.events {
		if existing.Provider == ev.Provider && existing.ProviderEventID == ev.ProviderEventID {
			return billing.ErrDuplicateEvent
		}
	}
	l.st.events[ev.ID] = ev
	return nil
}

func (l *Ledger) ResolveUser(_ context.Context, providerSubscriptionID, providerCustomerID string) (string, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, sub := range l.st.subs {
		if providerSubscriptionID != "" && sub.ProviderSubscriptionID == providerSubscriptionID {
			return sub.UserID, true, nil
		}
		if providerCustomerID != "" && sub.ProviderCustomerID == providerCustomerID {
			return sub.UserID, true, nil
		}
	}
	return "", false, nil
}

func (l *Ledger) ClaimEvents(_ context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]billing.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	due := make([]billing.Event, 0)
	for _, ev := range l.st.events {
		if ev.Status == billing.EventReceived && !ev.NextAttemptAt.After(now) {
			due = append(due, ev)
		}
	}
	slices.SortFunc(due, func(a, b billing.Event) int {
		if c := a.ReceivedAt.Compare(b.ReceivedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	var claimed []billing.Event
	seen := map[string]bool{}
	for _, ev := range due {
		if len(claimed) == limit {
			break
		}
		if seen[ev.UserID] {
			continue
		}
		seen[ev.UserID] = true
		if held, ok := l.st.locks[ev.UserID]; ok && held.lockedThru.After(now) {
			continue
		}
		l.st.locks[ev.UserID] = lockState{claimer: claimer, lockedThru: now.Add(lease)}
		ev.Claimer = claimer
		claimed = append(claimed, ev)
	}
	return claimed, nil
}

func (l *Ledger) ReleaseUser(_ context.Context, userID, claimer string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if held, ok := l.st.locks[userID]; ok && held.claimer == claimer {
		delete(l.st.locks, userID)
	}
	return nil
}

func (l *Ledger) RescheduleEvent(_ context.Context, id string, nextAt time.Time, reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	ev, ok := l.st.events[id]
	if !ok || ev.Status != billing.EventReceived {
		return billing.ErrDuplicateEvent
	}
	ev.Attempts++
	ev.NextAttemptAt = nextAt
	ev.LastError = reason
	l.st.events[id] = ev
	return nil
}

func (l *Ledger) FailEvent(_ context.Context, id string, reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	ev, ok := l.st.events[id]
	if !ok || ev.Status != billing.EventReceived {
		return billing.ErrDuplicateEvent
	}
	ev.Status = billing.EventFailed
	ev.LastError = reason
	l.st.events[id] = ev
	return nil
}

func (l *Ledger) MarkEventProcessed(_ context.Context, id, reason string, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	ev, ok := l.st.events[id]
	if !ok {
		return fmt.Errorf("event %s not found", id)
	}
	if ev.Status == billing.EventProcessed {
		return nil
	}
	ev.Status = billing.EventProcessed
	ev.Reason = reason
	ev.ProcessedAt = &at
	l.st.events[id] = ev
	return nil
}

func (l *Ledger) OldestReceived(context.Context) (time.Time, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var oldest time.Time
	found := false
	for _, ev := range l.st.events {
		if ev.Status == billing.EventReceived && (!found || ev.ReceivedAt.Before(oldest)) {
			oldest, found = ev.ReceivedAt, true
		}
	}
	return oldest, found, nil
}

func (l *Ledger) AccountPlan(_ context.Context, userID string) (billing.AccountPlan, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.st.accounts[userID], nil
}

func (l *Ledger) SetPlan(_ context.Context, userID, plan string) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	acc, ok := l.st.accounts[userID]
	if !ok {
		return 0, errors.New("account not found")
	}
	acc.Plan = plan
	acc.PlanVersion++
	l.st.accounts[userID] = acc
	return acc.PlanVersion, nil
}

func (l *Ledger) SubscriptionsOf(_ context.Context, userID string) ([]billing.Subscription, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []billing.Subscription
	for _, sub := range l.st.subs {
		if sub.UserID == userID {
			out = append(out, sub)
		}
	}
	slices.SortFunc(out, func(a, b billing.Subscription) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (l *Ledger) FindTarget(_ context.Context, userID, providerSubscriptionID string) (billing.Subscription, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if providerSubscriptionID != "" {
		for _, sub := range l.st.subs {
			if sub.UserID == userID && sub.ProviderSubscriptionID == providerSubscriptionID {
				return sub, true, nil
			}
		}
	}
	for _, sub := range l.st.subs {
		if sub.UserID == userID && sub.ProviderSubscriptionID == "" && liveStatus(sub.Status) {
			return sub, true, nil
		}
	}
	return billing.Subscription{}, false, nil
}

func liveStatus(s billing.Status) bool {
	return s == billing.StatusPending || s == billing.StatusActive || s == billing.StatusPastDue
}

func (l *Ledger) ApplySubscription(_ context.Context, sub billing.Subscription, readAt time.Time) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	current, ok := l.st.subs[sub.ID]
	if !ok {
		return false, fmt.Errorf("subscription %s not found", sub.ID)
	}
	if current.ProviderSyncedAt != nil && !current.ProviderSyncedAt.Before(readAt) {
		return false, nil
	}
	l.st.subs[sub.ID] = sub
	return true, nil
}

func (l *Ledger) InsertSubscription(_ context.Context, sub billing.Subscription) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.st.subs[sub.ID]; ok {
		return billing.ErrLiveExists
	}
	if liveStatus(sub.Status) {
		for _, other := range l.st.subs {
			if other.UserID == sub.UserID && liveStatus(other.Status) {
				return billing.ErrLiveExists
			}
		}
	}
	l.st.subs[sub.ID] = sub
	return nil
}

func (l *Ledger) AcquireUser(_ context.Context, userID, claimer string, now time.Time, lease time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if held, ok := l.st.locks[userID]; ok && held.lockedThru.After(now) {
		return false, nil
	}
	l.st.locks[userID] = lockState{claimer: claimer, lockedThru: now.Add(lease)}
	return true, nil
}

func (l *Ledger) LiveSubscriptions(context.Context) ([]billing.Subscription, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []billing.Subscription
	for _, sub := range l.st.subs {
		if sub.ProviderSubID != "" && liveStatus(sub.Status) {
			out = append(out, sub)
		}
	}
	slices.SortFunc(out, func(a, b billing.Subscription) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (l *Ledger) RequeueEvents(_ context.Context, now, stuckBefore time.Time) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	requeued := 0
	for id, ev := range l.st.events {
		switch {
		case ev.Status == billing.EventFailed:
			ev.Status = billing.EventReceived
			ev.Attempts = 0
			ev.NextAttemptAt = now
			requeued++
		case ev.Status == billing.EventReceived && ev.ReceivedAt.Before(stuckBefore) && ev.NextAttemptAt.After(now):
			ev.NextAttemptAt = now
			requeued++
		default:
			continue
		}
		l.st.events[id] = ev
	}
	return requeued, nil
}

type Outbox struct{ ledger *Ledger }

func (o Outbox) Insert(_ context.Context, entry outbox.Entry) error {
	o.ledger.mu.Lock()
	defer o.ledger.mu.Unlock()
	o.ledger.st.outbox = append(o.ledger.st.outbox, entry)
	return nil
}

func (l *Ledger) Outbox() Outbox { return Outbox{ledger: l} }

type Transactor struct {
	ledger *Ledger
	hook   db.StepHook
	txMu   sync.Mutex
}

func NewTransactor(ledger *Ledger, hook db.StepHook) *Transactor {
	return &Transactor{ledger: ledger, hook: hook}
}

func (t *Transactor) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if fn == nil {
		return db.ErrNilCallback
	}
	t.txMu.Lock()
	defer t.txMu.Unlock()
	before := t.ledger.snapshot()
	if err := fn(ctx); err != nil {
		t.ledger.restore(before)
		return fmt.Errorf("%w: %w", db.ErrTransactionAborted, err)
	}
	return nil
}

func (t *Transactor) Step(ctx context.Context, step db.Step) error {
	if t.hook == nil {
		return nil
	}
	if err := t.hook(ctx, step); err != nil {
		return fmt.Errorf("step %s: %w", step, err)
	}
	return nil
}

func NewEventID() string {
	id, err := idgen.New()
	if err != nil {
		panic(err)
	}
	return id
}
