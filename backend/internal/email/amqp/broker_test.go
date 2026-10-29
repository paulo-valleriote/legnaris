package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/shared/logger"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exchangeRecord, queueRecord and bindingRecord capture what the adapter asked
// the channel to declare, so the topology can be asserted without a broker.
type exchangeRecord struct {
	name       string
	kind       string
	durable    bool
	autoDelete bool
	internal   bool
	args       amqp.Table
}

type queueRecord struct {
	name       string
	durable    bool
	autoDelete bool
	exclusive  bool
	args       amqp.Table
}

type bindingRecord struct {
	queue    string
	exchange string
	key      string
}

// fakeConfirmation drives the publisher-confirm outcome.
type fakeConfirmation struct {
	acked bool
	err   error
	wait  func(ctx context.Context) (bool, error)
}

func (f *fakeConfirmation) WaitContext(ctx context.Context) (bool, error) {
	if f.wait != nil {
		return f.wait(ctx)
	}
	return f.acked, f.err
}

// fakeChannel implements the channel seam, recording declarations and
// publishings and driving confirm/nack/return/transport outcomes (EML-07).
type fakeChannel struct {
	confirmed  bool
	closed     bool
	declareErr error

	exchanges []exchangeRecord
	queues    []queueRecord
	bindings  []bindingRecord

	returns chan amqp.Return

	publishErr          error
	returnBeforeConfirm bool
	confirm             confirmation

	mu         sync.Mutex
	deliveries chan amqp.Delivery
	qosCount   int
	qosCalled  bool
	qosErr     error
	consumeErr error
	cancelErr  error
	cancelled  []string

	consumeQueue   string
	consumeTag     string
	consumeAutoAck bool

	published     []amqp.Publishing
	lastExchange  string
	lastKey       string
	lastMandatory bool
}

func (f *fakeChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	if f.declareErr != nil {
		return f.declareErr
	}
	f.exchanges = append(f.exchanges, exchangeRecord{
		name: name, kind: kind, durable: durable, autoDelete: autoDelete, internal: internal, args: args,
	})
	return nil
}

func (f *fakeChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if f.declareErr != nil {
		return amqp.Queue{}, f.declareErr
	}
	f.queues = append(f.queues, queueRecord{
		name: name, durable: durable, autoDelete: autoDelete, exclusive: exclusive, args: args,
	})
	return amqp.Queue{Name: name}, nil
}

func (f *fakeChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	if f.declareErr != nil {
		return f.declareErr
	}
	f.bindings = append(f.bindings, bindingRecord{queue: name, exchange: exchange, key: key})
	return nil
}

func (f *fakeChannel) Confirm(noWait bool) error {
	f.confirmed = true
	return nil
}

func (f *fakeChannel) NotifyReturn(recv chan amqp.Return) chan amqp.Return {
	f.returns = recv
	return recv
}

func (f *fakeChannel) PublishWithDeferredConfirmWithContext(_ context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (confirmation, error) {
	f.published = append(f.published, msg)
	f.lastExchange, f.lastKey, f.lastMandatory = exchange, key, mandatory

	if f.publishErr != nil {
		return nil, f.publishErr
	}
	if f.returnBeforeConfirm && f.returns != nil {
		f.returns <- amqp.Return{
			ReplyCode:  312,
			ReplyText:  "NO_ROUTE",
			Exchange:   exchange,
			RoutingKey: key,
			MessageId:  msg.MessageId,
		}
	}
	if f.confirm != nil {
		return f.confirm, nil
	}
	return &fakeConfirmation{acked: true}, nil
}

func (f *fakeChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	f.qosCalled = true
	f.qosCount = prefetchCount
	return f.qosErr
}

func (f *fakeChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	f.consumeQueue, f.consumeTag, f.consumeAutoAck = queue, consumer, autoAck
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	return f.deliveries, nil
}

func (f *fakeChannel) Cancel(consumer string, noWait bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelled = append(f.cancelled, consumer)
	return f.cancelErr
}

func (f *fakeChannel) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// fakeConnection implements the connection seam over one fake channel.
type fakeConnection struct {
	channel *fakeChannel
	closed  bool

	// consumers are handed out, in order, after the first channel, so the
	// publisher and the consumer get channels of their own.
	consumers  []*fakeChannel
	opened     int
	channelErr error
}

func (c *fakeConnection) Channel() (channel, error) {
	if c.channelErr != nil && c.opened > 0 {
		return nil, c.channelErr
	}
	c.opened++
	if c.opened > 1 && c.opened-2 < len(c.consumers) {
		return c.consumers[c.opened-2], nil
	}
	return c.channel, nil
}
func (c *fakeConnection) NotifyStateChange(chan *amqp.StateChanged) {}
func (c *fakeConnection) NotifyBlocked(recv chan amqp.Blocking) chan amqp.Blocking {
	return recv
}
func (c *fakeConnection) Close() error {
	c.closed = true
	return nil
}

func newFakeBroker(t *testing.T, fake *fakeChannel, cfg Config) (*Broker, *fakeConnection) {
	t.Helper()

	conn := &fakeConnection{channel: fake}
	dial := func(_ string, _ amqp.Config) (connection, error) { return conn, nil }

	if cfg.ConfirmTimeout == 0 {
		cfg.ConfirmTimeout = time.Second
	}
	cfg.Logger = logger.NewLogger(slog.LevelError)

	broker, err := dialWith(dial, "amqp://guest:guest@localhost:5672/", cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Close() })
	return broker, conn
}

func testMessage() email.Message {
	return email.Message{
		ID:        "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
		Type:      email.TypeOTP,
		To:        "player@example.com",
		Template:  string(email.PurposeLoginOTP),
		Data:      email.Data{Code: "042917", ExpiresInMinutes: 10, Purpose: email.PurposeLoginOTP},
		CreatedAt: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		Attempt:   0,
	}
}

// TestDial_RejectsBlankURL proves a blank broker URL fails fast at startup
// (EML-15).
func TestDial_RejectsBlankURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "Error: empty URL", url: ""},
		{name: "Error: whitespace URL", url: "   "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Dial(tc.url, Config{Logger: logger.NewLogger(slog.LevelError)})
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrBrokerUnavailable)
		})
	}
}

// TestDial_UnreachableBrokerFailsFast proves a dial failure is a typed startup
// error, not a publish-time surprise (EML-15, EML-19).
func TestDial_UnreachableBrokerFailsFast(t *testing.T) {
	dial := func(_ string, _ amqp.Config) (connection, error) {
		return nil, errors.New("connection refused")
	}

	_, err := dialWith(dial, "amqp://guest:guest@localhost:5672/", Config{Logger: logger.NewLogger(slog.LevelError)})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBrokerUnavailable)
}

// TestBroker_PublishConfirmed proves a confirmed publish carries the contract
// properties and is published mandatorily to the main exchange (EML-02,
// EML-07).
func TestBroker_PublishConfirmed(t *testing.T) {
	fake := &fakeChannel{}
	broker, _ := newFakeBroker(t, fake, Config{})

	message := testMessage()
	require.NoError(t, broker.Publish(context.Background(), message))

	require.Len(t, fake.published, 1)
	props := fake.published[0]

	t.Run("Happy path: the AMQP properties are the contract's", func(t *testing.T) {
		assert.Equal(t, "application/json", props.ContentType)
		assert.Equal(t, amqp.Persistent, props.DeliveryMode)
		assert.Equal(t, message.ID, props.MessageId)
		assert.True(t, fake.lastMandatory, "routing must be mandatory")
		assert.Equal(t, exchangeMain, fake.lastExchange)
		assert.Equal(t, routingKeyOTP, fake.lastKey)
	})

	t.Run("Happy path: the body is the encoded message contract", func(t *testing.T) {
		var decoded email.Message
		require.NoError(t, json.Unmarshal(props.Body, &decoded))
		assert.Equal(t, message, decoded)
	})

	t.Run("Edge: the code never appears in the properties", func(t *testing.T) {
		assert.NotContains(t, props.MessageId, "042917")
		assert.NotContains(t, props.ContentType, "042917")
	})
}

// TestBroker_PublishTransient proves every unconfirmed outcome is classified
// transient, so the relay reschedules instead of marking the row published
// (EML-07, EML-19).
func TestBroker_PublishTransient(t *testing.T) {
	tests := []struct {
		name  string
		fake  *fakeChannel
		block bool
		cfg   Config
	}{
		{
			name: "Error: a nack is transient",
			fake: &fakeChannel{confirm: &fakeConfirmation{acked: false}},
		},
		{
			name: "Error: a confirm wait timeout is transient",
			fake: &fakeChannel{confirm: &fakeConfirmation{wait: func(ctx context.Context) (bool, error) {
				<-ctx.Done()
				return false, ctx.Err()
			}}},
			cfg: Config{ConfirmTimeout: 10 * time.Millisecond},
		},
		{
			name: "Error: a transport error is transient",
			fake: &fakeChannel{publishErr: errors.New("connection reset")},
		},
		{
			name: "Error: a recorded basic.return is transient",
			fake: &fakeChannel{returnBeforeConfirm: true},
		},
		{
			name:  "Error: a blocked connection is transient",
			fake:  &fakeChannel{},
			block: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			broker, _ := newFakeBroker(t, tc.fake, tc.cfg)
			if tc.block {
				broker.setBlocked(true)
			}

			err := broker.Publish(context.Background(), testMessage())

			require.Error(t, err)
			assert.ErrorIs(t, err, email.ErrTransient)
		})
	}
}

// TestBroker_DeclareTopology proves the topology is declared through the seam
// and a declaration failure is surfaced (EML-06).
func TestBroker_DeclareTopology(t *testing.T) {
	t.Run("Happy path: the topology declares without error", func(t *testing.T) {
		fake := &fakeChannel{}
		broker, _ := newFakeBroker(t, fake, Config{})

		require.NoError(t, broker.DeclareTopology())
		assert.Len(t, fake.exchanges, 2)
		assert.Len(t, fake.queues, 5)
	})

	t.Run("Error: a declaration failure is surfaced", func(t *testing.T) {
		fake := &fakeChannel{declareErr: errors.New("channel closed")}
		broker, _ := newFakeBroker(t, fake, Config{})

		require.Error(t, broker.DeclareTopology())
	})
}

// TestBroker_DeclareTopology_RetryDelays proves the delay tiers follow the
// configured delays, fall back to the documented defaults, and an unusable
// delay is refused before anything is declared (WKR-14).
func TestBroker_DeclareTopology_RetryDelays(t *testing.T) {
	ttlOf := func(t *testing.T, fake *fakeChannel, name string) any {
		t.Helper()
		for _, queue := range fake.queues {
			if queue.name == name {
				return queue.args[argMessageTTL]
			}
		}
		t.Fatalf("queue %s was not declared", name)
		return nil
	}

	tests := []struct {
		name    string
		delays  RetryDelays
		want    [3]int32
		wantErr error
	}{
		{name: "Happy path: configured delays become the tier TTLs", delays: RetryDelays{First: time.Second, Second: 4 * time.Second, Later: 9 * time.Second}, want: [3]int32{1000, 4000, 9000}},
		{name: "Edge: zero delays fall back to 30s, 2m and 5m", delays: RetryDelays{}, want: [3]int32{30000, 120000, 300000}},
		{name: "Edge: a single configured tier keeps the other defaults", delays: RetryDelays{Second: time.Minute}, want: [3]int32{30000, 60000, 300000}},
		{name: "Error: a negative delay is refused", delays: RetryDelays{First: -time.Second}, wantErr: ErrInvalidRetryDelay},
		{name: "Error: a delay below one millisecond is refused", delays: RetryDelays{First: time.Microsecond}, wantErr: ErrInvalidRetryDelay},
		{name: "Error: a delay beyond the broker TTL range is refused", delays: RetryDelays{Later: 30 * 24 * time.Hour}, wantErr: ErrInvalidRetryDelay},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeChannel{}
			broker, _ := newFakeBroker(t, fake, Config{RetryDelays: tc.delays})

			err := broker.DeclareTopology()

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, fake.queues, "nothing may be declared with an unusable delay")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want[0], ttlOf(t, fake, queueRetry30s))
			assert.Equal(t, tc.want[1], ttlOf(t, fake, queueRetry2m))
			assert.Equal(t, tc.want[2], ttlOf(t, fake, queueRetry5m))
		})
	}
}

// TestBroker_Close proves Close closes the channel and connection and stops the
// supervisor, so no goroutine leaks (EML-05).
func TestBroker_Close(t *testing.T) {
	fake := &fakeChannel{}
	conn := &fakeConnection{channel: fake}

	dial := func(_ string, _ amqp.Config) (connection, error) { return conn, nil }
	broker, err := dialWith(dial, "amqp://guest:guest@localhost:5672/", Config{Logger: logger.NewLogger(slog.LevelError)})
	require.NoError(t, err)

	require.NoError(t, broker.Close())

	assert.True(t, fake.closed, "the channel must be closed")
	assert.True(t, conn.closed, "the connection must be closed")

	t.Run("Edge: closing twice is safe", func(t *testing.T) {
		require.NoError(t, broker.Close())
	})
}

// TestValidateURL pins the startup validation shared by the relay and the
// worker (EML-15, WKR-19).
func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "Happy path: a plain AMQP URI", url: "amqp://guest:guest@rabbitmq:5672/"},
		{name: "Happy path: a TLS URI with a vhost", url: "amqps://user:pass@broker.example:5671/vhost"},
		{name: "Error: empty", url: "", wantErr: true},
		{name: "Error: blank", url: "   ", wantErr: true},
		{name: "Error: not a URI", url: "not-a-uri", wantErr: true},
		{name: "Error: a foreign scheme", url: "http://localhost:5672/", wantErr: true},
		{name: "Error: a missing host", url: "amqp://", wantErr: true},
		{name: "Error: an unparseable URI", url: "amqp://%zz", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateURL(tc.url)

			if tc.wantErr {
				assert.ErrorIs(t, err, ErrBrokerUnavailable)
				return
			}
			assert.NoError(t, err)
		})
	}
}
