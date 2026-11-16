package emailtest

import (
	"context"
	"sync"

	"legnaris/backend/internal/email"
)

type Recorder struct {
	mu      sync.Mutex
	sent    []email.Outbound
	failure error
}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Send(_ context.Context, m email.Outbound) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sent = append(r.sent, m)
	return r.failure
}

func (r *Recorder) SetFailure(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failure = err
}

func (r *Recorder) Sent() []email.Outbound {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]email.Outbound(nil), r.sent...)
}

func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sent)
}
