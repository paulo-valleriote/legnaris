package worker_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/emailtest"
	"legnaris/backend/internal/email/worker"
	"legnaris/backend/internal/email/worker/mocks"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestWorker_DeadLettersUnusableMessages pins the decode-and-validate matrix:
// every message that can never be delivered is dead-lettered as it arrived,
// acknowledged, counted, and never sent (WKR-04, WKR-05, WKR-07).
func TestWorker_DeadLettersUnusableMessages(t *testing.T) {
	valid := emailtest.Message()

	tests := []struct {
		name       string
		body       []byte
		noTemplate bool
		withBody   func(t *testing.T) []byte
	}{
		{name: "Edge: an empty body", body: []byte{}},
		{name: "Edge: a nil body", body: nil},
		{name: "Edge: a body that is not JSON", body: []byte("not json")},
		{name: "Edge: a JSON array instead of an object", body: []byte(`[1,2]`)},
		{name: "Edge: trailing data after the message", body: append(body(t, valid), []byte(`{}`)...)},
		{name: "Edge: the code has the wrong type", withBody: func(t *testing.T) []byte {
			return bodyWith(t, valid, func(o map[string]any) { dataOf(o)["code"] = 42917 })
		}},
		{name: "Edge: the lifetime has the wrong type", withBody: func(t *testing.T) []byte {
			return bodyWith(t, valid, func(o map[string]any) { dataOf(o)["expires_in_minutes"] = "ten" })
		}},
		{name: "Edge: the creation time is not a timestamp", withBody: func(t *testing.T) []byte {
			return bodyWith(t, valid, func(o map[string]any) { o["created_at"] = "yesterday" })
		}},
		{name: "Edge: a blank identity", withBody: func(t *testing.T) []byte {
			msg := valid
			msg.ID = ""
			return body(t, msg)
		}},
		{name: "Edge: an uppercase identity", withBody: func(t *testing.T) []byte {
			msg := valid
			msg.ID = "018F4A1E-5C6B-7D8E-9F01-23456789ABCD"
			return body(t, msg)
		}},
		{name: "Edge: a UUID v4 identity", withBody: func(t *testing.T) []byte {
			msg := valid
			msg.ID = "3f2b8c1e-5c6b-4d8e-9f01-23456789abcd"
			return body(t, msg)
		}},
		{name: "Edge: a missing code", withBody: func(t *testing.T) []byte {
			msg := valid
			msg.Data.Code = ""
			return body(t, msg)
		}},
		{name: "Edge: a blank code", withBody: func(t *testing.T) []byte {
			msg := valid
			msg.Data.Code = "   "
			return body(t, msg)
		}},
		{name: "Edge: a purpose that selects no template", noTemplate: true, withBody: func(t *testing.T) []byte {
			msg := valid
			msg.Data.Purpose = "reset_password"
			return body(t, msg)
		}},
		{name: "Edge: a missing purpose", noTemplate: true, withBody: func(t *testing.T) []byte {
			msg := valid
			msg.Data.Purpose = ""
			return body(t, msg)
		}},
		{name: "Edge: a type that is not for this pipeline", withBody: func(t *testing.T) []byte {
			msg := valid
			msg.Type = "email.newsletter"
			return body(t, msg)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			payload := tc.body
			if tc.withBody != nil {
				payload = tc.withBody(t)
			}
			handle := h.queue(payload)

			if tc.noTemplate {
				h.republisher.EXPECT().PublishTo(mock.Anything, email.DestinationDeadLetter, mock.Anything).Return(nil).Once()
			} else {
				h.republisher.EXPECT().PublishBodyTo(mock.Anything, email.DestinationDeadLetter, payload).Return(nil).Once()
			}
			h.observer.EXPECT().SendResult(worker.ResultDeadLettered).Once()
			h.observer.EXPECT().MessageDead().Once()
			h.expectAck(handle)

			h.run()

			assert.Zero(t, h.sender.Count(), "an unusable message must never be sent")
		})
	}
}

// TestWorker_ToleratesUnknownFields proves a newer publisher's message is not
// dead-lettered during a rolling deploy (WKR-04).
func TestWorker_ToleratesUnknownFields(t *testing.T) {
	h := newHarness(t)
	handle := h.queue(bodyWith(t, emailtest.Message(), func(o map[string]any) { o["priority"] = "high" }))
	h.observer.EXPECT().SendDuration(mock.Anything).Once()
	h.observer.EXPECT().SendResult(worker.ResultSent).Once()
	h.expectAck(handle)

	h.run()

	assert.Equal(t, 1, h.sender.Count())
}

// TestWorker_ExpiryDecision pins the deadline against the injected clock: the
// message is dropped, not sent, once now reaches CreatedAt + lifetime (WKR-03,
// WKR-07, SC-002).
func TestWorker_ExpiryDecision(t *testing.T) {
	created := emailtest.Message().CreatedAt

	tests := []struct {
		name        string
		now         time.Time
		lifetime    int
		createdAt   time.Time
		wantExpired bool
	}{
		{name: "Happy path: before the deadline the message is sent", now: created.Add(9 * time.Minute), lifetime: 10, createdAt: created},
		{name: "Edge: one nanosecond before the deadline is still sent", now: created.Add(10*time.Minute - time.Nanosecond), lifetime: 10, createdAt: created},
		{name: "Edge: exactly at the deadline is expired", now: created.Add(10 * time.Minute), lifetime: 10, createdAt: created, wantExpired: true},
		{name: "Edge: past the deadline is expired", now: created.Add(time.Hour), lifetime: 10, createdAt: created, wantExpired: true},
		{name: "Edge: a zero lifetime is already expired", now: created, lifetime: 0, createdAt: created, wantExpired: true},
		{name: "Edge: a negative lifetime is already expired", now: created, lifetime: -5, createdAt: created, wantExpired: true},
		{name: "Edge: a creation time in the future is sent", now: created, lifetime: 10, createdAt: created.Add(time.Hour)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, func(c *worker.Config) { c.Now = func() time.Time { return tc.now } })
			msg := emailtest.Message()
			msg.CreatedAt = tc.createdAt
			msg.Data.ExpiresInMinutes = tc.lifetime
			handle := h.queue(body(t, msg))
			h.expectAck(handle)

			if tc.wantExpired {
				h.observer.EXPECT().SendResult(worker.ResultExpired).Once()
				h.observer.EXPECT().MessageExpired().Once()
			} else {
				h.observer.EXPECT().SendDuration(mock.Anything).Once()
				h.observer.EXPECT().SendResult(worker.ResultSent).Once()
			}

			h.run()

			if tc.wantExpired {
				assert.Zero(t, h.sender.Count(), "an expired code must not be sent")
			} else {
				assert.Equal(t, 1, h.sender.Count())
			}
		})
	}
}

// TestWorker_RendersAndSends proves the rendered subject, both bodies and the
// identity as the idempotency key reach the sending port, and that the
// acknowledgement comes last (WKR-02, WKR-06, WKR-08, SC-004).
func TestWorker_RendersAndSends(t *testing.T) {
	for _, purpose := range []email.Purpose{email.PurposeLoginOTP, email.PurposeVerifyEmail} {
		t.Run(fmt.Sprintf("Happy path: %s", purpose), func(t *testing.T) {
			h := newHarness(t)
			msg := emailtest.Message()
			msg.Template = string(purpose)
			msg.Data.Purpose = purpose
			handle := h.queue(body(t, msg))
			h.observer.EXPECT().SendDuration(mock.Anything).Once()
			h.observer.EXPECT().SendResult(worker.ResultSent).Once()
			h.expectAck(handle)

			h.run()

			sent := h.sender.Sent()
			require.Len(t, sent, 1)
			assert.Equal(t, msg.ID, sent[0].IdempotencyKey)
			assert.Equal(t, msg.To, sent[0].To)
			assert.NotEmpty(t, sent[0].Subject)
			assert.NotContains(t, sent[0].Subject, secretCode, "the subject must never carry the code")
			assert.Contains(t, sent[0].Text, secretCode)
			assert.Contains(t, sent[0].HTML, secretCode)
			assert.Equal(t, []string{"send", "ack"}, h.events.all(), "the acknowledgement is the last step")
		})
	}
}

// TestWorker_RedeliveryKeepsTheIdempotencyKey proves a redelivered message is
// sent again under the same key, so the provider de-duplicates it (WKR-08).
func TestWorker_RedeliveryKeepsTheIdempotencyKey(t *testing.T) {
	h := newHarness(t)
	payload := body(t, emailtest.Message())
	first := h.queue(payload)
	second := h.queue(payload)
	h.observer.EXPECT().SendDuration(mock.Anything).Twice()
	h.observer.EXPECT().SendResult(worker.ResultSent).Twice()
	h.expectAck(first)
	h.expectAck(second)

	h.run()

	sent := h.sender.Sent()
	require.Len(t, sent, 2)
	assert.Equal(t, sent[0].IdempotencyKey, sent[1].IdempotencyKey)
}

// TestWorker_TwoConsumersDeliverEachMessageOnce runs two workers over one
// delivery stream and asserts each message is sent exactly once (WKR-08,
// WKR-16).
func TestWorker_TwoConsumersDeliverEachMessageOnce(t *testing.T) {
	const total = 20

	stream := make(chan worker.Delivery, total)
	wantIDs := make(map[string]struct{}, total)
	for range total {
		id, err := idgen.New()
		require.NoError(t, err)
		wantIDs[id] = struct{}{}

		msg := emailtest.Message()
		msg.ID = id
		handle := mocks.NewMockDeliveryHandle(t)
		handle.EXPECT().Ack().Return(nil).Once()
		stream <- worker.Delivery{Body: body(t, msg), Handle: handle}
	}

	sender := emailtest.NewRecorder()
	observer := mocks.NewMockObserver(t)
	observer.EXPECT().SendDuration(mock.Anything).Times(total)
	observer.EXPECT().SendResult(worker.ResultSent).Times(total)

	source := mocks.NewMockDeliverySource(t)
	source.EXPECT().Consume(mock.Anything).RunAndReturn(
		func(context.Context) (<-chan worker.Delivery, error) { return stream, nil })

	renderer, err := email.NewRenderer()
	require.NoError(t, err)
	config := worker.Config{
		Now:          func() time.Time { return emailtest.Message().CreatedAt.Add(time.Minute) },
		DrainTimeout: 2 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for range 2 {
		w := worker.New(source, mocks.NewMockRepublisher(t), sender, renderer, observer, config, &captureLogger{})
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, w.Run(ctx))
		}()
	}

	require.Eventually(t, func() bool { return sender.Count() == total }, 5*time.Second, 5*time.Millisecond)
	cancel()
	wg.Wait()

	gotIDs := make(map[string]struct{}, total)
	for _, outbound := range sender.Sent() {
		gotIDs[outbound.IdempotencyKey] = struct{}{}
	}
	assert.Equal(t, wantIDs, gotIDs)
	assert.Equal(t, total, sender.Count(), "each message is sent exactly once")
}

// TestWorker_Logs proves a message is identified by its identity and purpose,
// the recipient is masked, and the code appears in no log line (WKR-06,
// WKR-21, WKR-22, SC-005).
func TestWorker_Logs(t *testing.T) {
	failures := []struct {
		name string
		err  error
	}{
		{name: "Happy path: a sent message", err: nil},
		{name: "Edge: a retryable failure", err: fmt.Errorf("%w: provider said code %s", email.ErrTransient, secretCode)},
		{name: "Edge: a permanent failure", err: fmt.Errorf("%w: rejected %s", email.ErrPermanent, emailtest.Message().To)},
	}

	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			msg := emailtest.Message()
			handle := h.queue(body(t, msg))
			h.sender.SetFailure(tc.err)
			h.observer.EXPECT().SendDuration(mock.Anything).Maybe()
			h.observer.EXPECT().SendResult(mock.Anything).Maybe()
			h.observer.EXPECT().MessageDead().Maybe()
			h.republisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			handle.EXPECT().Ack().Return(nil).Maybe()

			h.run()

			logged := h.log.text()
			require.NotEmpty(t, logged)
			assert.Contains(t, logged, msg.ID)
			assert.Contains(t, logged, string(msg.Data.Purpose))
			assert.Contains(t, logged, email.MaskRecipient(msg.To))
			assert.NotContains(t, logged, msg.To, "the full address must never be logged")
			assert.NotContains(t, logged, secretCode, "the code must never be logged")
		})
	}
}

// TestWorker_LogsNeverCarryAnUndecodableBody proves a body that could not be
// decoded is described by its size only: it may still hold a code.
func TestWorker_LogsNeverCarryAnUndecodableBody(t *testing.T) {
	h := newHarness(t)
	payload := []byte(`{"data":{"code":"042917"`)
	handle := h.queue(payload)
	h.republisher.EXPECT().PublishBodyTo(mock.Anything, email.DestinationDeadLetter, payload).Return(nil).Once()
	h.observer.EXPECT().SendResult(worker.ResultDeadLettered).Once()
	h.observer.EXPECT().MessageDead().Once()
	h.expectAck(handle)

	h.run()

	assert.NotContains(t, h.log.text(), secretCode)
}

// TestWorker_ExpiryIsDecidedBeforeTheTemplate proves an expired message with a
// purpose that selects no template is dropped as expired, never dead-lettered
// (WKR-03, WKR-12).
func TestWorker_ExpiryIsDecidedBeforeTheTemplate(t *testing.T) {
	h := newHarness(t, func(c *worker.Config) {
		c.Now = func() time.Time { return emailtest.Message().CreatedAt.Add(time.Hour) }
	})
	msg := emailtest.Message()
	msg.Data.Purpose = "reset_password"
	handle := h.queue(body(t, msg))
	h.observer.EXPECT().SendResult(worker.ResultExpired).Once()
	h.observer.EXPECT().MessageExpired().Once()
	h.expectAck(handle)

	h.run()

	assert.Zero(t, h.sender.Count())
}
