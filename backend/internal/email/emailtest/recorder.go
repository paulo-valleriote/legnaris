// Package emailtest provides recording test doubles and shared fixtures for
// the email domain, following backend/internal/sheet/sheettest: a recording Sender with
// injectable failures and plain builders over email.Message and email.Outbound
// (never an outbox.Intent, so this package stays inside the sending side)
// (EML-10).
package emailtest

import (
	"context"
	"sync"

	"legnaris/backend/internal/email"
)

// Recorder is a Sender that records every outbound message and can be told to
// return a chosen sentinel (EML-10).
type Recorder struct {
	mu      sync.Mutex
	sent    []email.Outbound
	failure error
}

// NewRecorder builds an empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Send records the outbound message and returns the configured failure, if any.
func (r *Recorder) Send(_ context.Context, m email.Outbound) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sent = append(r.sent, m)
	return r.failure
}

// SetFailure makes every subsequent Send return err; pass nil to clear it.
func (r *Recorder) SetFailure(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failure = err
}

// Sent returns a copy of the recorded outbound messages.
func (r *Recorder) Sent() []email.Outbound {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]email.Outbound(nil), r.sent...)
}

// Count returns how many messages were recorded.
func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sent)
}
