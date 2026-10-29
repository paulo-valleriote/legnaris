package worker_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/emailtest"
	"legnaris/backend/internal/email/worker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var errTransient = fmt.Errorf("%w: provider timeout", email.ErrTransient)

// TestWorker_RetryLadder pins the delay tier chosen from the message's own
// attempt count, and that the republished message changes nothing but the count
// (WKR-09, WKR-14, WKR-15, SC-003).
func TestWorker_RetryLadder(t *testing.T) {
	tests := []struct {
		name        string
		attempt     int
		wantTier    email.Destination
		wantAttempt int
	}{
		{name: "Happy path: the first failure waits half a minute", attempt: 0, wantTier: email.DestinationRetry30s, wantAttempt: 1},
		{name: "Happy path: the second failure waits two minutes", attempt: 1, wantTier: email.DestinationRetry2m, wantAttempt: 2},
		{name: "Happy path: the third failure waits five minutes", attempt: 2, wantTier: email.DestinationRetry5m, wantAttempt: 3},
		{name: "Happy path: the fourth failure waits five minutes", attempt: 3, wantTier: email.DestinationRetry5m, wantAttempt: 4},
		{name: "Edge: the last allowed attempt still waits five minutes", attempt: 4, wantTier: email.DestinationRetry5m, wantAttempt: 5},
		{name: "Edge: a negative count is treated as zero", attempt: -3, wantTier: email.DestinationRetry30s, wantAttempt: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			msg := emailtest.Message()
			msg.Attempt = tc.attempt
			handle := h.queue(body(t, msg))
			h.sender.SetFailure(errTransient)

			want := msg
			want.Attempt = tc.wantAttempt
			h.republisher.EXPECT().PublishTo(mock.Anything, tc.wantTier, want).
				Run(func(_ context.Context, _ email.Destination, _ email.Message) { h.events.add("publish") }).
				Return(nil).Once()
			h.observer.EXPECT().SendDuration(mock.Anything).Once()
			h.observer.EXPECT().SendResult(worker.ResultRetried).Once()
			h.expectAck(handle)

			h.run()

			assert.Equal(t, []string{"send", "publish", "ack"}, h.events.all(), "the original is acknowledged only after the confirmed republish")
		})
	}
}

// TestWorker_RetryClassification proves any failure that is not permanent is
// retried: an unclassified provider error must not lose the code.
func TestWorker_RetryClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "Happy path: a transient failure", err: errTransient},
		{name: "Edge: an unclassified failure", err: errors.New("connection reset")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			handle := h.queue(body(t, emailtest.Message()))
			h.sender.SetFailure(tc.err)
			h.republisher.EXPECT().PublishTo(mock.Anything, email.DestinationRetry30s, mock.Anything).Return(nil).Once()
			h.observer.EXPECT().SendDuration(mock.Anything).Once()
			h.observer.EXPECT().SendResult(worker.ResultRetried).Once()
			h.expectAck(handle)

			h.run()
		})
	}
}

// TestWorker_TerminalPaths proves the permanent failure, the exhausted cap and
// a count already at the cap each reach the dead-letter route, acknowledged and
// counted, with no second send; and that expiry is decided first (WKR-11,
// WKR-12, SC-003).
func TestWorker_TerminalPaths(t *testing.T) {
	created := emailtest.Message().CreatedAt

	tests := []struct {
		name        string
		attempt     int
		maxAttempts int
		failure     error
		now         time.Time
		wantSends   int
		wantExpired bool
	}{
		{name: "Happy path: a permanent failure is dead-lettered at once", attempt: 0, failure: fmt.Errorf("%w: invalid recipient", email.ErrPermanent), wantSends: 1},
		{name: "Edge: the count at the cap is dead-lettered without a send", attempt: 5, wantSends: 0},
		{name: "Edge: a count far above the cap is dead-lettered without a send", attempt: 99, wantSends: 0},
		{name: "Edge: a custom cap of two dead-letters at two", attempt: 2, maxAttempts: 2, wantSends: 0},
		{name: "Edge: an expired message at the cap is expired, not dead-lettered", attempt: 5, now: created.Add(time.Hour), wantSends: 0, wantExpired: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, func(c *worker.Config) {
				c.MaxAttempts = tc.maxAttempts
				if !tc.now.IsZero() {
					c.Now = func() time.Time { return tc.now }
				}
			})
			msg := emailtest.Message()
			msg.Attempt = tc.attempt
			handle := h.queue(body(t, msg))
			h.sender.SetFailure(tc.failure)
			h.expectAck(handle)

			if tc.wantExpired {
				h.observer.EXPECT().SendResult(worker.ResultExpired).Once()
				h.observer.EXPECT().MessageExpired().Once()
			} else {
				h.republisher.EXPECT().PublishTo(mock.Anything, email.DestinationDeadLetter, msg).Return(nil).Once()
				h.observer.EXPECT().SendResult(worker.ResultDeadLettered).Once()
				h.observer.EXPECT().MessageDead().Once()
			}
			if tc.wantSends > 0 {
				h.observer.EXPECT().SendDuration(mock.Anything).Times(tc.wantSends)
			}

			h.run()

			assert.Equal(t, tc.wantSends, h.sender.Count())
		})
	}
}

// TestWorker_UnconfirmedPublishLeavesTheOriginal proves a republish or a
// dead-letter publish the broker did not confirm leaves the original
// unacknowledged and uncounted, so the broker redelivers it (WKR-13, SC-001).
func TestWorker_UnconfirmedPublishLeavesTheOriginal(t *testing.T) {
	unconfirmed := fmt.Errorf("%w: broker nacked", email.ErrTransient)

	t.Run("Error: an unconfirmed retry republish", func(t *testing.T) {
		h := newHarness(t)
		h.queue(body(t, emailtest.Message()))
		h.sender.SetFailure(errTransient)
		h.republisher.EXPECT().PublishTo(mock.Anything, email.DestinationRetry30s, mock.Anything).Return(unconfirmed).Once()
		h.observer.EXPECT().SendDuration(mock.Anything).Once()

		h.run()

		assert.Contains(t, h.log.text(), "not confirmed")
	})

	t.Run("Error: an unconfirmed dead-letter publish", func(t *testing.T) {
		h := newHarness(t)
		msg := emailtest.Message()
		msg.Attempt = 5
		h.queue(body(t, msg))
		h.republisher.EXPECT().PublishTo(mock.Anything, email.DestinationDeadLetter, mock.Anything).Return(unconfirmed).Once()

		h.run()

		assert.Zero(t, h.sender.Count())
	})

	t.Run("Error: an unconfirmed dead-letter publish of an undecodable body", func(t *testing.T) {
		h := newHarness(t)
		payload := []byte("not json")
		h.queue(payload)
		h.republisher.EXPECT().PublishBodyTo(mock.Anything, email.DestinationDeadLetter, payload).Return(unconfirmed).Once()

		h.run()
	})
}

// TestWorker_AckFailureIsNotFatal proves a failed acknowledgement is logged and
// does not stop the worker: the broker redelivers and the provider de-duplicates
// (WKR-08).
func TestWorker_AckFailureIsNotFatal(t *testing.T) {
	h := newHarness(t)
	handle := h.queue(body(t, emailtest.Message()))
	h.observer.EXPECT().SendDuration(mock.Anything).Once()
	h.observer.EXPECT().SendResult(worker.ResultSent).Once()
	handle.EXPECT().Ack().Return(errors.New("channel closed")).Once()

	h.run()

	assert.Contains(t, h.log.text(), "acknowledgement failed")
}
