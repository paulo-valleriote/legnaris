package worker_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/emailtest"
	"legnaris/backend/internal/email/worker"
	"legnaris/backend/internal/email/worker/mocks"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const secretCode = "042917"

type captureLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *captureLogger) append(msg string, args []logger.Attribute) {
	parts := []string{msg}
	for _, attribute := range args {
		parts = append(parts, attribute.Key+"="+attribute.Value)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, strings.Join(parts, " "))
}

func (l *captureLogger) Info(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLogger) Debug(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLogger) Warn(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLogger) Error(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLogger) Fatal(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLogger) With(string, string) logger.StructuredLogger { return l }

func (l *captureLogger) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) add(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, name)
}

func (e *events) all() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.list...)
}

type orderedSender struct {
	events *events
	inner  *emailtest.Recorder
}

func (s *orderedSender) Send(ctx context.Context, m email.Outbound) error {
	s.events.add("send")
	return s.inner.Send(ctx, m)
}

type harness struct {
	t           *testing.T
	events      *events
	sender      *emailtest.Recorder
	republisher *mocks.MockRepublisher
	observer    *mocks.MockObserver
	log         *captureLogger
	config      worker.Config
	queued      []worker.Delivery
}

func newHarness(t *testing.T, mutate ...func(*worker.Config)) *harness {
	t.Helper()

	h := &harness{
		t:           t,
		events:      &events{},
		sender:      emailtest.NewRecorder(),
		republisher: mocks.NewMockRepublisher(t),
		observer:    mocks.NewMockObserver(t),
		log:         &captureLogger{},
		config: worker.Config{
			Now:                func() time.Time { return emailtest.Message().CreatedAt.Add(5 * time.Minute) },
			DrainTimeout:       2 * time.Second,
			RegisterRetryDelay: time.Millisecond,
		},
	}
	for _, apply := range mutate {
		apply(&h.config)
	}
	return h
}

func (h *harness) build(source worker.DeliverySource) *worker.Worker {
	renderer, err := email.NewRenderer()
	require.NoError(h.t, err)
	sender := &orderedSender{events: h.events, inner: h.sender}
	return worker.New(source, h.republisher, sender, renderer, h.observer, h.config, h.log)
}

func (h *harness) queue(body []byte) *mocks.MockDeliveryHandle {
	handle := mocks.NewMockDeliveryHandle(h.t)
	h.queued = append(h.queued, worker.Delivery{Body: body, Handle: handle})
	return handle
}

func (h *harness) expectAck(handle *mocks.MockDeliveryHandle) {
	handle.EXPECT().Ack().Run(func() { h.events.add("ack") }).Return(nil).Once()
}

func (h *harness) run() {
	h.t.Helper()

	stream := make(chan worker.Delivery, len(h.queued))
	for _, delivery := range h.queued {
		stream <- delivery
	}
	close(stream)

	drained := make(chan struct{})
	var calls int
	source := mocks.NewMockDeliverySource(h.t)
	source.EXPECT().Consume(mock.Anything).RunAndReturn(
		func(ctx context.Context) (<-chan worker.Delivery, error) {
			calls++
			if calls == 1 {
				return stream, nil
			}
			select {
			case <-drained:
			default:
				close(drained)
			}
			<-ctx.Done()
			return nil, ctx.Err()
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finished := make(chan error, 1)
	w := h.build(source)
	go func() { finished <- w.Run(ctx) }()

	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		h.t.Fatal("the worker never drained the stream")
	}
	cancel()

	select {
	case err := <-finished:
		require.NoError(h.t, err)
	case <-time.After(5 * time.Second):
		h.t.Fatal("the worker never stopped")
	}
}

func body(t *testing.T, msg email.Message) []byte {
	t.Helper()

	encoded, err := json.Marshal(msg)
	require.NoError(t, err)
	return encoded
}

func bodyWith(t *testing.T, msg email.Message, rewrite func(map[string]any)) []byte {
	t.Helper()

	var object map[string]any
	require.NoError(t, sharedjson.DecodeBytes(body(t, msg), &object))
	rewrite(object)
	encoded, err := json.Marshal(object)
	require.NoError(t, err)
	return encoded
}

func dataOf(object map[string]any) map[string]any {
	return object["data"].(map[string]any)
}
