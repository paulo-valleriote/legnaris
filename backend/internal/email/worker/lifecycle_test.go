package worker_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/emailtest"
	"legnaris/backend/internal/email/worker"
	"legnaris/backend/internal/email/worker/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// gateSender blocks every Send until released or its context ends, and tracks
// how many sends overlap.
type gateSender struct {
	release chan struct{}
	started chan struct{}
	active  atomic.Int32
	peak    atomic.Int32
}

func newGateSender() *gateSender {
	return &gateSender{release: make(chan struct{}), started: make(chan struct{}, 64)}
}

func (s *gateSender) Send(ctx context.Context, _ email.Outbound) error {
	current := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		peak := s.peak.Load()
		if current <= peak || s.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	s.started <- struct{}{}

	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *harness) buildWith(source worker.DeliverySource, sender email.Sender) *worker.Worker {
	renderer, err := email.NewRenderer()
	require.NoError(h.t, err)
	return worker.New(source, h.republisher, sender, renderer, h.observer, h.config, h.log)
}

func sourceReturning(t *testing.T, stream chan worker.Delivery) *mocks.MockDeliverySource {
	t.Helper()
	source := mocks.NewMockDeliverySource(t)
	source.EXPECT().Consume(mock.Anything).RunAndReturn(
		func(ctx context.Context) (<-chan worker.Delivery, error) {
			return stream, nil
		}).Maybe()
	return source
}

func waitFor(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

// TestWorker_RegistersAgainWhenTheStreamEnds proves the end of the stream is
// "register and continue", not a failure (WKR-17, SC-007).
func TestWorker_RegistersAgainWhenTheStreamEnds(t *testing.T) {
	h := newHarness(t)
	payload := body(t, emailtest.Message())

	first := make(chan worker.Delivery, 1)
	second := make(chan worker.Delivery, 1)
	handle := mocks.NewMockDeliveryHandle(t)
	handle.EXPECT().Ack().Return(nil).Once()
	second <- worker.Delivery{Body: payload, Handle: handle}
	close(first)

	var calls atomic.Int32
	source := mocks.NewMockDeliverySource(t)
	source.EXPECT().Consume(mock.Anything).RunAndReturn(
		func(ctx context.Context) (<-chan worker.Delivery, error) {
			switch calls.Add(1) {
			case 1:
				return first, nil
			case 2:
				return second, nil
			default:
				<-ctx.Done()
				return nil, ctx.Err()
			}
		})
	h.observer.EXPECT().SendDuration(mock.Anything).Once()
	h.observer.EXPECT().SendResult(worker.ResultSent).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.buildWith(source, h.sender).Run(ctx) }()

	require.Eventually(t, func() bool { return h.sender.Count() == 1 }, 5*time.Second, 5*time.Millisecond)
	cancel()
	require.NoError(t, <-done)
	assert.GreaterOrEqual(t, calls.Load(), int32(2))
}

// TestWorker_RetriesFailedRegistration proves a refused registration is logged
// and retried until it succeeds, never fatal (WKR-17).
func TestWorker_RetriesFailedRegistration(t *testing.T) {
	h := newHarness(t)

	var calls atomic.Int32
	registered := make(chan struct{})
	source := mocks.NewMockDeliverySource(t)
	source.EXPECT().Consume(mock.Anything).RunAndReturn(
		func(ctx context.Context) (<-chan worker.Delivery, error) {
			if calls.Add(1) <= 3 {
				return nil, errors.New("broker unavailable")
			}
			close(registered)
			<-ctx.Done()
			return nil, ctx.Err()
		}).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.buildWith(source, h.sender).Run(ctx) }()

	waitFor(t, registered)
	cancel()
	require.NoError(t, <-done)
	assert.Contains(t, h.log.text(), "consumer registration failed")
}

// TestWorker_StopsDuringRegistrationBackoff proves the stop signal ends the
// wait between registration attempts.
func TestWorker_StopsDuringRegistrationBackoff(t *testing.T) {
	h := newHarness(t, func(c *worker.Config) { c.RegisterRetryDelay = time.Hour })

	failed := make(chan struct{}, 1)
	source := mocks.NewMockDeliverySource(t)
	source.EXPECT().Consume(mock.Anything).RunAndReturn(
		func(context.Context) (<-chan worker.Delivery, error) {
			select {
			case failed <- struct{}{}:
			default:
			}
			return nil, errors.New("broker unavailable")
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.buildWith(source, h.sender).Run(ctx) }()

	waitFor(t, failed)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the stop signal did not end the registration backoff")
	}
}

// TestWorker_GracefulStopFinishesTheMessageInFlight proves a stop takes no new
// message but lets the one in flight finish and be acknowledged (WKR-18,
// SC-008).
func TestWorker_GracefulStopFinishesTheMessageInFlight(t *testing.T) {
	h := newHarness(t)
	sender := newGateSender()
	stream := make(chan worker.Delivery, 1)
	handle := mocks.NewMockDeliveryHandle(t)
	handle.EXPECT().Ack().Return(nil).Once()
	stream <- worker.Delivery{Body: body(t, emailtest.Message()), Handle: handle}
	h.observer.EXPECT().SendDuration(mock.Anything).Once()
	h.observer.EXPECT().SendResult(worker.ResultSent).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.buildWith(sourceReturning(t, stream), sender).Run(ctx) }()

	waitFor(t, sender.started)
	cancel()

	select {
	case <-done:
		t.Fatal("the worker stopped while a send was in flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(sender.release)
	require.NoError(t, <-done)
}

// TestWorker_StopLeavesAnUnfinishedMessageUnacknowledged proves a send that does
// not finish within the drain budget is abandoned and left with the broker
// (WKR-18, SC-008).
func TestWorker_StopLeavesAnUnfinishedMessageUnacknowledged(t *testing.T) {
	h := newHarness(t, func(c *worker.Config) { c.DrainTimeout = 50 * time.Millisecond })
	sender := newGateSender()
	stream := make(chan worker.Delivery, 1)
	stream <- worker.Delivery{Body: body(t, emailtest.Message()), Handle: mocks.NewMockDeliveryHandle(t)}
	h.observer.EXPECT().SendDuration(mock.Anything).Maybe()
	h.republisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, _ email.Destination, _ email.Message) error { return ctx.Err() }).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.buildWith(sourceReturning(t, stream), sender).Run(ctx) }()

	waitFor(t, sender.started)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the worker did not stop within the drain budget")
	}
}

// TestWorker_PrefetchBoundsTheMessagesInFlight proves no more messages than the
// prefetch are handled at once (WKR-16).
func TestWorker_PrefetchBoundsTheMessagesInFlight(t *testing.T) {
	const prefetch, total = 2, 6

	h := newHarness(t, func(c *worker.Config) { c.Prefetch = prefetch })
	sender := newGateSender()
	stream := make(chan worker.Delivery, total)
	for range total {
		handle := mocks.NewMockDeliveryHandle(t)
		handle.EXPECT().Ack().Return(nil).Once()
		stream <- worker.Delivery{Body: body(t, emailtest.Message()), Handle: handle}
	}
	h.observer.EXPECT().SendDuration(mock.Anything).Times(total)
	h.observer.EXPECT().SendResult(worker.ResultSent).Times(total)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.buildWith(sourceReturning(t, stream), sender).Run(ctx) }()

	for range prefetch {
		waitFor(t, sender.started)
	}
	select {
	case <-sender.started:
		t.Fatal("a message beyond the prefetch bound was started")
	case <-time.After(50 * time.Millisecond):
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range total - prefetch {
			<-sender.started
		}
	}()
	close(sender.release)
	wg.Wait()
	cancel()
	require.NoError(t, <-done)

	assert.LessOrEqual(t, sender.peak.Load(), int32(prefetch))
}
