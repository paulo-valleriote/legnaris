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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// scriptedSender answers each send with the next scripted result and succeeds
// once the script is exhausted.
type scriptedSender struct {
	mu      sync.Mutex
	results []error
	calls   int
}

func (s *scriptedSender) Send(context.Context, email.Outbound) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if len(s.results) == 0 {
		return nil
	}
	next := s.results[0]
	s.results = s.results[1:]
	return next
}

// breakerRun starts the worker in the background and returns the stop function
// that cancels it and waits for it to finish.
func breakerRun(t *testing.T, w *worker.Worker) func() {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- w.Run(ctx) }()

	return func() {
		cancel()
		select {
		case err := <-finished:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("the worker never stopped")
		}
	}
}

// breakerWorker builds a worker over one stream the test keeps open, with a real
// clock and a breaker that opens on the first outage. It returns the doubles the
// suites observe.
func breakerWorker(t *testing.T, sender email.Sender, probeInterval time.Duration, stream chan worker.Delivery) (*worker.Worker, *mocks.MockRepublisher, *mocks.MockObserver) {
	t.Helper()

	republisher := mocks.NewMockRepublisher(t)
	observer := mocks.NewMockObserver(t)
	renderer, err := email.NewRenderer()
	require.NoError(t, err)

	source := mocks.NewMockDeliverySource(t)
	source.EXPECT().Consume(mock.Anything).Return(stream, nil).Once()

	w := worker.New(source, republisher, sender, renderer, observer, worker.Config{
		Prefetch:             1,
		DrainTimeout:         2 * time.Second,
		BreakerThreshold:     1,
		BreakerProbeInterval: probeInterval,
	}, &captureLogger{})
	return w, republisher, observer
}

// freshBody encodes a message created now, so the expiry decision on the real
// clock never drops it.
func freshBody(t *testing.T) []byte {
	t.Helper()
	msg := emailtest.Message()
	msg.CreatedAt = time.Now()
	return body(t, msg)
}

func TestWorker_BreakerPausesConsumptionAfterOutage(t *testing.T) {
	sender := &scriptedSender{results: []error{fmt.Errorf("%w: 503", email.ErrTransient)}}
	first := mocks.NewMockDeliveryHandle(t)
	stream := make(chan worker.Delivery, 2)
	w, republisher, observer := breakerWorker(t, sender, time.Hour, stream)

	opened := make(chan struct{})
	observer.EXPECT().SendDuration(mock.Anything).Once()
	observer.EXPECT().SendResult(worker.ResultRetried).Once()
	observer.EXPECT().BreakerOpened().Run(func() { close(opened) }).Once()
	republisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	first.EXPECT().Ack().Return(nil).Once()

	stream <- worker.Delivery{Body: freshBody(t), Handle: first}
	stream <- worker.Delivery{Body: freshBody(t), Handle: mocks.NewMockDeliveryHandle(t)}
	stop := breakerRun(t, w)

	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("the breaker never opened")
	}
	time.Sleep(50 * time.Millisecond)
	stop()

	sender.mu.Lock()
	defer sender.mu.Unlock()
	require.Equal(t, 1, sender.calls, "an open breaker must not read the second delivery")
}

func TestWorker_BreakerResumesAfterSuccessfulProbe(t *testing.T) {
	sender := &scriptedSender{results: []error{fmt.Errorf("%w: 503", email.ErrTransient)}}
	first := mocks.NewMockDeliveryHandle(t)
	second := mocks.NewMockDeliveryHandle(t)
	stream := make(chan worker.Delivery, 2)
	w, republisher, observer := breakerWorker(t, sender, 20*time.Millisecond, stream)

	closed := make(chan struct{})
	acked := make(chan struct{})
	observer.EXPECT().SendDuration(mock.Anything).Twice()
	observer.EXPECT().SendResult(worker.ResultRetried).Once()
	observer.EXPECT().SendResult(worker.ResultSent).Once()
	observer.EXPECT().BreakerOpened().Once()
	observer.EXPECT().BreakerClosed().Run(func() { close(closed) }).Once()
	republisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	first.EXPECT().Ack().Return(nil).Once()
	second.EXPECT().Ack().Run(func() { close(acked) }).Return(nil).Once()

	stream <- worker.Delivery{Body: freshBody(t), Handle: first}
	stream <- worker.Delivery{Body: freshBody(t), Handle: second}
	stop := breakerRun(t, w)
	defer stop()

	for _, signal := range []chan struct{}{closed, acked} {
		select {
		case <-signal:
		case <-time.After(5 * time.Second):
			t.Fatal("the probe never resolved the breaker")
		}
	}
}

func TestWorker_BreakerProbeThatNeverReachesTheProviderDoesNotWedge(t *testing.T) {
	sender := &scriptedSender{results: []error{fmt.Errorf("%w: 503", email.ErrTransient)}}
	first := mocks.NewMockDeliveryHandle(t)
	stream := make(chan worker.Delivery, 3)
	w, republisher, observer := breakerWorker(t, sender, 20*time.Millisecond, stream)

	closed := make(chan struct{})
	acked := make(chan struct{})
	probeHandle := mocks.NewMockDeliveryHandle(t)
	recovered := mocks.NewMockDeliveryHandle(t)
	observer.EXPECT().SendDuration(mock.Anything).Twice()
	observer.EXPECT().SendResult(worker.ResultRetried).Once()
	observer.EXPECT().SendResult(worker.ResultSent).Once()
	observer.EXPECT().SendResult(worker.ResultDeadLettered).Once()
	observer.EXPECT().MessageDead().Once()
	observer.EXPECT().BreakerOpened().Once()
	observer.EXPECT().BreakerClosed().Run(func() { close(closed) }).Once()
	republisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	republisher.EXPECT().PublishBodyTo(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	first.EXPECT().Ack().Return(nil).Once()
	probeHandle.EXPECT().Ack().Return(nil).Once()
	recovered.EXPECT().Ack().Run(func() { close(acked) }).Return(nil).Once()

	stream <- worker.Delivery{Body: freshBody(t), Handle: first}
	stream <- worker.Delivery{Body: []byte(`{"not":"a message"`), Handle: probeHandle}
	stream <- worker.Delivery{Body: freshBody(t), Handle: recovered}
	stop := breakerRun(t, w)
	defer stop()

	for _, signal := range []chan struct{}{closed, acked} {
		select {
		case <-signal:
		case <-time.After(5 * time.Second):
			t.Fatal("a probe that never reached the provider wedged the breaker")
		}
	}
}
