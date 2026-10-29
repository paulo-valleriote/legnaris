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

// captureLogger records every structured line so a suite can assert the code
// and the unmasked recipient never reach a log (WKR-06, WKR-22).
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

// events records the order of the side effects one delivery performs.
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

// orderedSender is a Sender that records "send" in the shared event list and
// then delegates to a Recorder.
type orderedSender struct {
	events *events
	inner  *emailtest.Recorder
}

func (s *orderedSender) Send(ctx context.Context, m email.Outbound) error {
	s.events.add("send")
	return s.inner.Send(ctx, m)
}

// harness wires a worker over mockery doubles, a recording sender, the real
// renderer and a fixed clock, and drives it through the delivery stream.
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

// queue adds one delivery and returns its handle so the caller can state
// whether it is acknowledged. The acknowledgement is recorded as an event.
func (h *harness) queue(body []byte) *mocks.MockDeliveryHandle {
	handle := mocks.NewMockDeliveryHandle(h.t)
	h.queued = append(h.queued, worker.Delivery{Body: body, Handle: handle})
	return handle
}

// expectAck states that the delivery is acknowledged exactly once.
func (h *harness) expectAck(handle *mocks.MockDeliveryHandle) {
	handle.EXPECT().Ack().Run(func() { h.events.add("ack") }).Return(nil).Once()
}

// run delivers every queued message through one stream, then stops the worker
// once the stream is drained and waits for it to finish.
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

// body encodes a message as the publisher would.
func body(t *testing.T, msg email.Message) []byte {
	t.Helper()

	encoded, err := json.Marshal(msg)
	require.NoError(t, err)
	return encoded
}

// bodyWith encodes a message and lets the caller rewrite the generic JSON
// object, to produce shapes the typed struct cannot express.
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
