package amqp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/shared/idgen"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testActor = "0190a5f0-0000-7000-8000-000000000002"
	testSheet = "0190a5f0-0000-7000-8000-000000000003"
)

var errFake = errors.New("fake broker failure")

type confirmMode int

const (
	confirmAck confirmMode = iota
	confirmNack
	confirmSilent
)

type fakeRecorder struct {
	mu    sync.Mutex
	drops map[string]int
}

func (r *fakeRecorder) EventDropped(eventType, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.drops == nil {
		r.drops = make(map[string]int)
	}
	r.drops[eventType+"|"+reason]++
}

func (r *fakeRecorder) count(eventType, reason string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.drops[eventType+"|"+reason]
}

func (r *fakeRecorder) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	sum := 0
	for _, n := range r.drops {
		sum += n
	}
	return sum
}

type published struct {
	exchange, key string
	msg           amqp091.Publishing
}

type fakeChannel struct {
	mu         sync.Mutex
	mode       confirmMode
	publishErr error
	confirms   chan amqp091.Confirmation
	declared   []string
	exchanges  map[string]amqp091.Table
	binds      []string
	published  []published
	closed     bool
}

func newFakeChannel(mode confirmMode) *fakeChannel {
	return &fakeChannel{mode: mode, exchanges: map[string]amqp091.Table{}}
}

func (c *fakeChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.declared = append(c.declared, "exchange:"+name+":"+kind)
	c.exchanges[name] = args
	return nil
}

func (c *fakeChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.declared = append(c.declared, "queue:"+name+":"+args["x-queue-type"].(string))
	return amqp091.Queue{Name: name}, nil
}

func (c *fakeChannel) QueueBind(name, key, exchange string, noWait bool, args amqp091.Table) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.binds = append(c.binds, name+"<-"+exchange)
	return nil
}

func (c *fakeChannel) Confirm(noWait bool) error { return nil }

func (c *fakeChannel) NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.confirms = confirm
	return confirm
}

func (c *fakeChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.publishErr != nil {
		return c.publishErr
	}
	c.published = append(c.published, published{exchange: exchange, key: key, msg: msg})
	switch c.mode {
	case confirmAck:
		c.confirms <- amqp091.Confirmation{DeliveryTag: uint64(len(c.published)), Ack: true}
	case confirmNack:
		c.confirms <- amqp091.Confirmation{DeliveryTag: uint64(len(c.published)), Ack: false}
	}
	return nil
}

func (c *fakeChannel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeChannel) publishedMessages() []published {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]published(nil), c.published...)
}

type fakeConn struct {
	ch    *fakeChannel
	chErr error
}

func (c *fakeConn) Channel() (channel, error) {
	if c.chErr != nil {
		return nil, c.chErr
	}
	return c.ch, nil
}

func (c *fakeConn) Close() error { return nil }

type fakeDialer struct {
	mu      sync.Mutex
	results []dialResult
	calls   int
	block   chan struct{}
	entered chan struct{}
}

type dialResult struct {
	conn *fakeConn
	err  error
}

func (d *fakeDialer) dial(string) (connection, error) {
	if d.entered != nil {
		d.entered <- struct{}{}
	}
	if d.block != nil {
		<-d.block
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	if len(d.results) == 0 {
		return nil, errFake
	}
	r := d.results[0]
	if len(d.results) > 1 {
		d.results = d.results[1:]
	}
	if r.err != nil {
		return nil, r.err
	}
	return r.conn, nil
}

func (d *fakeDialer) dialCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

func healthyDialer(ch *fakeChannel) *fakeDialer {
	return &fakeDialer{results: []dialResult{{conn: &fakeConn{ch: ch}}}}
}

func sampleEvent(eventType string) events.Event {
	actor := testActor
	return events.Event{
		Type:             eventType,
		ActorID:          &actor,
		AggregateType:    "sheet",
		AggregateID:      testSheet,
		AggregateVersion: 3,
		Data:             map[string]any{"changed_keys": []string{"hp"}},
	}
}

func newTestPublisher(t *testing.T, d *fakeDialer, rec *fakeRecorder, mutate func(*Config)) *Publisher {
	t.Helper()
	cfg := Config{
		URL:            "amqp://user:pass@broker:5672/",
		Recorder:       rec,
		Buffer:         16,
		ConfirmTimeout: time.Second,
		RetryInterval:  time.Hour,
		dial:           d.dial,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	p := New(cfg)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		p.Shutdown(ctx)
	})
	return p
}

func TestPublisherPublish(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		mode       confirmMode
		wantRouted bool
	}{
		{name: "Happy path: confirmed event is published with its type as routing key", eventType: "sheet.updated", mode: confirmAck, wantRouted: true},
		{name: "Happy path: user-level event is published", eventType: "user.registered", mode: confirmAck, wantRouted: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel(tc.mode)
			rec := &fakeRecorder{}
			p := newTestPublisher(t, healthyDialer(ch), rec, nil)

			p.Publish(sampleEvent(tc.eventType))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			p.Shutdown(ctx)

			msgs := ch.publishedMessages()
			require.Len(t, msgs, 1)
			assert.Equal(t, ExchangeEvents, msgs[0].exchange)
			assert.Equal(t, tc.eventType, msgs[0].key)
			assert.Equal(t, uint8(amqp091.Persistent), msgs[0].msg.DeliveryMode)
			assert.Equal(t, "application/json", msgs[0].msg.ContentType)
			assert.Equal(t, tc.eventType, msgs[0].msg.Type)
			require.NoError(t, idgen.Validate(msgs[0].msg.MessageId), "message id must be a UUID v7")
			env, err := events.Decode(msgs[0].msg.Body)
			require.NoError(t, err)
			assert.Equal(t, msgs[0].msg.MessageId, env.ID)
			assert.Equal(t, 0, rec.total())
		})
	}
}

func TestPublisherTopology(t *testing.T) {
	ch := newFakeChannel(confirmAck)
	rec := &fakeRecorder{}
	p := newTestPublisher(t, healthyDialer(ch), rec, nil)

	p.Publish(sampleEvent("sheet.created"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p.Shutdown(ctx)

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{name: "Happy path: main topic exchange is declared", check: func(t *testing.T) {
			assert.Contains(t, ch.declared, "exchange:legnaris.events:topic")
		}},
		{name: "Happy path: unrouted fanout exchange is declared", check: func(t *testing.T) {
			assert.Contains(t, ch.declared, "exchange:legnaris.events.unrouted:fanout")
		}},
		{name: "Happy path: unrouted queue is declared as quorum and bound", check: func(t *testing.T) {
			assert.Contains(t, ch.declared, "queue:events.unrouted:quorum")
			assert.Contains(t, ch.binds, "events.unrouted<-legnaris.events.unrouted")
		}},
		{name: "Edge: main exchange names the unrouted exchange as alternate", check: func(t *testing.T) {
			assert.Equal(t, "legnaris.events.unrouted", ch.exchanges[ExchangeEvents]["alternate-exchange"])
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}
}

func TestDeclareTopologyIdempotent(t *testing.T) {
	tests := []struct {
		name  string
		calls int
	}{
		{name: "Happy path: a single declaration succeeds", calls: 1},
		{name: "Edge: repeated declarations succeed and record identical specs", calls: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel(confirmAck)
			var first []string
			for i := 0; i < tc.calls; i++ {
				require.NoError(t, declareTopology(ch))
				if i == 0 {
					first = append([]string(nil), ch.declared...)
				}
			}
			assert.Equal(t, first, ch.declared[:len(first)])
			assert.Len(t, ch.declared, len(first)*tc.calls)
		})
	}
}

func TestPublisherDropsAndCounts(t *testing.T) {
	tests := []struct {
		name       string
		dialer     func() *fakeDialer
		channel    func() *fakeChannel
		mutate     func(*Config)
		eventType  string
		wantReason string
	}{
		{
			name:       "Error: broker down at dial drops with broker_unavailable",
			dialer:     func() *fakeDialer { return &fakeDialer{results: []dialResult{{err: errFake}}} },
			eventType:  "sheet.updated",
			wantReason: events.ReasonBrokerUnavailable,
		},
		{
			name:       "Error: empty URL drops with broker_unavailable without dialing",
			dialer:     func() *fakeDialer { return &fakeDialer{results: []dialResult{{err: errFake}}} },
			mutate:     func(c *Config) { c.URL = "" },
			eventType:  "sheet.updated",
			wantReason: events.ReasonBrokerUnavailable,
		},
		{
			name: "Error: publish failure drops with broker_unavailable",
			dialer: func() *fakeDialer {
				ch := newFakeChannel(confirmAck)
				ch.publishErr = errFake
				return healthyDialer(ch)
			},
			eventType:  "sheet.updated",
			wantReason: events.ReasonBrokerUnavailable,
		},
		{
			name:       "Error: broker nack drops with publish_failed",
			dialer:     func() *fakeDialer { return healthyDialer(newFakeChannel(confirmNack)) },
			eventType:  "sheet.deleted",
			wantReason: events.ReasonPublishFailed,
		},
		{
			name:       "Error: missing confirm within timeout drops with publish_failed",
			dialer:     func() *fakeDialer { return healthyDialer(newFakeChannel(confirmSilent)) },
			mutate:     func(c *Config) { c.ConfirmTimeout = 20 * time.Millisecond },
			eventType:  "sheet.created",
			wantReason: events.ReasonPublishFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &fakeRecorder{}
			p := newTestPublisher(t, tc.dialer(), rec, tc.mutate)

			p.Publish(sampleEvent(tc.eventType))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			p.Shutdown(ctx)

			assert.Equal(t, 1, rec.count(tc.eventType, tc.wantReason))
			assert.Equal(t, 1, rec.total())
		})
	}
}

func TestPublisherInvalidEventIsDropped(t *testing.T) {
	tests := []struct {
		name string
		ev   events.Event
	}{
		{name: "Edge: empty event is dropped as invalid", ev: events.Event{}},
		{name: "Edge: event with malformed aggregate id is dropped as invalid", ev: events.Event{Type: "sheet.updated", AggregateType: "sheet", AggregateID: "nope"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel(confirmAck)
			rec := &fakeRecorder{}
			d := healthyDialer(ch)
			p := newTestPublisher(t, d, rec, nil)

			p.Publish(tc.ev)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			p.Shutdown(ctx)

			assert.Equal(t, 1, rec.count(tc.ev.Type, events.ReasonInvalidEvent))
			assert.Empty(t, ch.publishedMessages())
			assert.Equal(t, 0, d.dialCount())
		})
	}
}

func TestPublisherRecoversAfterBrokerReturns(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "Edge: an event dropped while the broker is down is followed by delivery once it is back"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel(confirmAck)
			d := &fakeDialer{results: []dialResult{{err: errFake}, {conn: &fakeConn{ch: ch}}}}
			rec := &fakeRecorder{}
			p := newTestPublisher(t, d, rec, func(c *Config) { c.RetryInterval = 0 })

			p.Publish(sampleEvent("sheet.created"))
			p.Publish(sampleEvent("sheet.updated"))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			p.Shutdown(ctx)

			assert.Equal(t, 1, rec.count("sheet.created", events.ReasonBrokerUnavailable))
			msgs := ch.publishedMessages()
			require.Len(t, msgs, 1)
			assert.Equal(t, "sheet.updated", msgs[0].key)
		})
	}
}

func TestPublisherRetryIsRateLimited(t *testing.T) {
	tests := []struct {
		name   string
		events int
	}{
		{name: "Edge: a down broker is dialed once per retry interval, not per event", events: 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &fakeDialer{results: []dialResult{{err: errFake}}}
			rec := &fakeRecorder{}
			p := newTestPublisher(t, d, rec, nil)

			for i := 0; i < tc.events; i++ {
				p.Publish(sampleEvent("sheet.updated"))
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			p.Shutdown(ctx)

			assert.Equal(t, 1, d.dialCount())
			assert.Equal(t, tc.events, rec.count("sheet.updated", events.ReasonBrokerUnavailable))
		})
	}
}

func TestPublisherNeverBlocksCaller(t *testing.T) {
	tests := []struct {
		name           string
		buffer         int
		sends          int
		wantBufferFull int
	}{
		{name: "Edge: a full buffer drops the excess and returns immediately", buffer: 1, sends: 4, wantBufferFull: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			release := make(chan struct{})
			entered := make(chan struct{}, 8)
			d := &fakeDialer{results: []dialResult{{err: errFake}}, block: release, entered: entered}
			rec := &fakeRecorder{}
			p := newTestPublisher(t, d, rec, func(c *Config) { c.Buffer = tc.buffer })

			p.Publish(sampleEvent("sheet.updated"))
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("worker never started dialing")
			}

			returned := make(chan struct{})
			go func() {
				for i := 0; i < tc.sends-1; i++ {
					p.Publish(sampleEvent("sheet.updated"))
				}
				close(returned)
			}()
			select {
			case <-returned:
			case <-time.After(2 * time.Second):
				t.Fatal("Publish blocked the caller while the worker was stuck")
			}
			close(release)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			p.Shutdown(ctx)

			assert.Equal(t, tc.wantBufferFull, rec.count("sheet.updated", events.ReasonBufferFull))
		})
	}
}

func TestPublisherShutdown(t *testing.T) {
	tests := []struct {
		name          string
		publishAfter  bool
		wantPublished int
		wantShutdown  int
	}{
		{name: "Happy path: queued events are drained before shutdown returns", publishAfter: false, wantPublished: 3, wantShutdown: 0},
		{name: "Edge: an event published after shutdown is dropped as shutdown", publishAfter: true, wantPublished: 3, wantShutdown: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel(confirmAck)
			rec := &fakeRecorder{}
			p := newTestPublisher(t, healthyDialer(ch), rec, nil)

			for i := 0; i < 3; i++ {
				p.Publish(sampleEvent("sheet.updated"))
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			p.Shutdown(ctx)
			if tc.publishAfter {
				p.Publish(sampleEvent("sheet.deleted"))
			}
			p.Shutdown(ctx)

			assert.Len(t, ch.publishedMessages(), tc.wantPublished)
			assert.Equal(t, tc.wantShutdown, rec.count("sheet.deleted", events.ReasonShutdown))
		})
	}
}

func TestPublisherShutdownDeadlineAbandonsQueue(t *testing.T) {
	tests := []struct {
		name   string
		queued int
	}{
		{name: "Edge: an expired drain deadline leaves nothing published and counts every event as dropped", queued: 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel(confirmAck)
			d := &fakeDialer{results: []dialResult{{err: errFake}}}
			rec := &fakeRecorder{}
			p := newTestPublisher(t, d, rec, nil)

			for i := 0; i < tc.queued; i++ {
				p.Publish(sampleEvent("sheet.updated"))
			}
			expired, cancel := context.WithCancel(context.Background())
			cancel()

			done := make(chan struct{})
			go func() {
				p.Shutdown(expired)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Shutdown did not return after the deadline")
			}

			assert.Empty(t, ch.publishedMessages())
			assert.Equal(t, tc.queued, rec.total())
		})
	}
}
