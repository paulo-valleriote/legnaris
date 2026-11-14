package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/generation"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConfirm answers a deferred confirm with a fixed outcome.
type fakeConfirm struct {
	acked bool
	err   error
	block bool
}

func (f fakeConfirm) WaitContext(ctx context.Context) (bool, error) {
	if f.block {
		<-ctx.Done()
		return false, ctx.Err()
	}
	return f.acked, f.err
}

// fakeChannel records declarations and publishes and answers each publish from
// its script.
type fakeChannel struct {
	mu          sync.Mutex
	exchanges   []string
	queues      map[string]amqp091.Table
	bindings    []string
	published   []publishCall
	confirm     confirmation
	publishErr  error
	returns     chan amqp091.Return
	confirmErr  error
	closed      bool
	confirmMode bool
}

type publishCall struct {
	exchange, key string
	mandatory     bool
	msg           amqp091.Publishing
}

func newFakeChannel() *fakeChannel {
	return &fakeChannel{queues: map[string]amqp091.Table{}, confirm: fakeConfirm{acked: true}}
}

func (f *fakeChannel) ExchangeDeclare(name, _ string, _, _, _, _ bool, _ amqp091.Table) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exchanges = append(f.exchanges, name)
	return nil
}

func (f *fakeChannel) QueueDeclare(name string, _, _, _, _ bool, args amqp091.Table) (amqp091.Queue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queues[name] = args
	return amqp091.Queue{Name: name}, nil
}

func (f *fakeChannel) QueueBind(name, key, exchange string, _ bool, _ amqp091.Table) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings = append(f.bindings, name+"|"+key+"|"+exchange)
	return nil
}

func (f *fakeChannel) Confirm(bool) error {
	f.confirmMode = true
	return f.confirmErr
}

func (f *fakeChannel) NotifyReturn(chan amqp091.Return) chan amqp091.Return {
	return f.returns
}

func (f *fakeChannel) PublishWithDeferredConfirmWithContext(_ context.Context, exchange, key string, mandatory, _ bool, msg amqp091.Publishing) (confirmation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	f.published = append(f.published, publishCall{exchange: exchange, key: key, mandatory: mandatory, msg: msg})
	return f.confirm, nil
}

func (f *fakeChannel) Close() error {
	f.closed = true
	return nil
}

// fakeConnection hands out one channel and counts how often it is dialled.
type fakeConnection struct {
	ch     *fakeChannel
	closed bool
	err    error
}

func (c *fakeConnection) Channel() (channel, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.ch, nil
}

func (c *fakeConnection) Close() error {
	c.closed = true
	return nil
}

// dialScript returns a dialer that yields the connections in order, one per dial.
func dialScript(t *testing.T, conns ...*fakeConnection) (dialer, *int) {
	t.Helper()
	dials := 0
	return func(string) (connection, error) {
		if dials >= len(conns) {
			return nil, errors.New("no broker")
		}
		c := conns[dials]
		dials++
		if c.err != nil {
			return nil, c.err
		}
		return c, nil
	}, &dials
}

func newTestPublisher(dial dialer, timeout time.Duration) *Publisher {
	return &Publisher{url: "amqp://test", confirmTimeout: timeout, dial: dial}
}

func TestRoutingKey(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want string
	}{
		{name: "Happy path: character jobs use the character key", kind: "character", want: "generation.character.requested"},
		{name: "Edge: an empty kind still yields the fixed shape", kind: "", want: "generation..requested"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RoutingKey(tc.kind))
		})
	}
}

func TestPublisher_Publish(t *testing.T) {
	const jobID = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	tests := []struct {
		name       string
		setup      func(ch *fakeChannel)
		wantErr    error
		wantRouted bool
	}{
		{name: "Happy path: an acknowledged job is routed on the kind key", setup: func(*fakeChannel) {}, wantRouted: true},
		{name: "Error: a nacked job is refused", setup: func(ch *fakeChannel) { ch.confirm = fakeConfirm{acked: false} }, wantErr: ErrPublishRefused},
		{name: "Error: a confirm that never arrives is refused", setup: func(ch *fakeChannel) { ch.confirm = fakeConfirm{block: true} }, wantErr: ErrPublishRefused},
		{name: "Error: a job the broker returns as unroutable is refused", setup: func(ch *fakeChannel) {
			ch.returns = make(chan amqp091.Return, 1)
			ch.returns <- amqp091.Return{MessageId: jobID}
		}, wantErr: ErrPublishRefused},
		{name: "Edge: a return for another job does not refuse this one", setup: func(ch *fakeChannel) {
			ch.returns = make(chan amqp091.Return, 1)
			ch.returns <- amqp091.Return{MessageId: "other"}
		}, wantRouted: true},
		{name: "Error: a publish transport error is reported", setup: func(ch *fakeChannel) { ch.publishErr = errors.New("channel closed") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel()
			tc.setup(ch)
			dial, _ := dialScript(t, &fakeConnection{ch: ch})
			pub := newTestPublisher(dial, 50*time.Millisecond)

			err := pub.Publish(context.Background(), generation.Message{ID: jobID, Kind: "character"})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			}
			if !tc.wantRouted {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Len(t, ch.published, 1)
			call := ch.published[0]
			assert.Equal(t, ExchangeGeneration, call.exchange)
			assert.Equal(t, "generation.character.requested", call.key)
			assert.True(t, call.mandatory, "publishes must be mandatory so unroutable jobs come back")
			assert.Equal(t, amqp091.Persistent, call.msg.DeliveryMode)
			assert.Equal(t, jobID, call.msg.MessageId)

			var body map[string]string
			require.NoError(t, json.Unmarshal(call.msg.Body, &body))
			assert.Equal(t, map[string]string{"generation_id": jobID, "kind": "character"}, body)
		})
	}
}

func TestPublisher_Topology(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, ch *fakeChannel)
	}{
		{
			name: "Happy path: the request queue dead-letters to generation.dead",
			check: func(t *testing.T, ch *fakeChannel) {
				args := ch.queues[QueueRequests]
				assert.Equal(t, queueTypeQuorum, args[argQueueType])
				assert.Equal(t, ExchangeDead, args[argDeadLetterExch])
				assert.Equal(t, KeyDead, args[argDeadLetterKey])
			},
		},
		{
			name: "Happy path: the requests queue is bound by the kind pattern on the topic exchange",
			check: func(t *testing.T, ch *fakeChannel) {
				assert.Contains(t, ch.bindings, QueueRequests+"|generation.*.requested|"+ExchangeGeneration)
			},
		},
		{
			name: "Edge: the dead-letter queue is bound to the dead exchange",
			check: func(t *testing.T, ch *fakeChannel) {
				assert.Contains(t, ch.bindings, QueueDead+"|"+KeyDead+"|"+ExchangeDead)
				assert.Equal(t, queueTypeQuorum, ch.queues[QueueDead][argQueueType])
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := newFakeChannel()
			dial, _ := dialScript(t, &fakeConnection{ch: ch})
			require.NoError(t, newTestPublisher(dial, time.Second).DeclareTopology())
			tc.check(t, ch)
		})
	}
}

func TestPublisher_Connection(t *testing.T) {
	const jobID = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	tests := []struct {
		name      string
		conns     func() []*fakeConnection
		wantErr   bool
		wantDials int
	}{
		{
			name: "Edge: a dial failure is an error and the next publish redials",
			conns: func() []*fakeConnection {
				ch := newFakeChannel()
				return []*fakeConnection{{err: errors.New("refused")}, {ch: ch}}
			},
			wantErr:   true,
			wantDials: 1,
		},
		{
			name: "Edge: a channel whose confirm mode fails is closed and not reused",
			conns: func() []*fakeConnection {
				ch := newFakeChannel()
				ch.confirmErr = errors.New("no confirms")
				return []*fakeConnection{{ch: ch}}
			},
			wantErr:   true,
			wantDials: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conns := tc.conns()
			dial, dials := dialScript(t, conns...)
			pub := newTestPublisher(dial, time.Second)

			err := pub.Publish(context.Background(), generation.Message{ID: jobID, Kind: "character"})
			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.wantDials, *dials)
		})
	}
}

func TestPublisher_Close(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
	}{
		{name: "Edge: closing a publisher that never connected is a no-op", connected: false},
		{name: "Happy path: closing a connected publisher closes its connection", connected: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conn := &fakeConnection{ch: newFakeChannel()}
			dial, _ := dialScript(t, conn)
			pub := newTestPublisher(dial, time.Second)
			if tc.connected {
				require.NoError(t, pub.DeclareTopology())
			}
			require.NoError(t, pub.Close())
			assert.Equal(t, tc.connected, conn.closed)
		})
	}
}

func TestNew_DefaultsConfirmTimeout(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want time.Duration
	}{
		{name: "Edge: a zero timeout uses the default", cfg: Config{}, want: DefaultConfirmTimeout},
		{name: "Edge: a negative timeout uses the default", cfg: Config{ConfirmTimeout: -time.Second}, want: DefaultConfirmTimeout},
		{name: "Happy path: a set timeout is kept", cfg: Config{ConfirmTimeout: time.Second}, want: time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, New(tc.cfg).confirmTimeout)
		})
	}
}

// TestPublisher_RedialsAfterFailure pins the recovery path: a failed publish drops
// the connection, so the next publish dials again and is routed on the new channel.
func TestPublisher_RedialsAfterFailure(t *testing.T) {
	const jobID = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	tests := []struct {
		name      string
		firstErr  error
		wantDials int
	}{
		{name: "Happy path: the publish after a transport failure reaches a fresh channel", firstErr: errors.New("channel closed"), wantDials: 2},
		{name: "Edge: a healthy first publish keeps its connection", firstErr: nil, wantDials: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := newFakeChannel()
			first.publishErr = tc.firstErr
			second := newFakeChannel()
			dial, dials := dialScript(t, &fakeConnection{ch: first}, &fakeConnection{ch: second})
			pub := newTestPublisher(dial, time.Second)
			msg := generation.Message{ID: jobID, Kind: "character"}

			err := pub.Publish(context.Background(), msg)
			assert.Equal(t, tc.firstErr != nil, err != nil)

			require.NoError(t, pub.Publish(context.Background(), msg))
			assert.Equal(t, tc.wantDials, *dials)
			if tc.firstErr == nil {
				assert.Len(t, first.published, 2, "a healthy connection is reused for both publishes")
				return
			}
			assert.Len(t, second.published, 1, "the publish after a failure goes out on the fresh channel")
		})
	}
}
