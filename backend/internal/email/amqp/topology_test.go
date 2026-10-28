package amqp

import (
	"legnaris/backend/internal/email"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func queueByName(t *testing.T, name string) queueSpec {
	t.Helper()

	for _, queue := range emailTopology().Queues {
		if queue.Name == name {
			return queue
		}
	}
	t.Fatalf("queue %s is not declared", name)
	return queueSpec{}
}

// TestEmailTopology_Queues pins the declared queue arguments: every queue is a
// durable quorum queue, the work queue rejects on overflow and dead-letters to
// the first retry tier, and each retry tier carries its TTL back to the main
// exchange (EML-06).
func TestEmailTopology_Queues(t *testing.T) {
	t.Run("Happy path: the work queue is bounded quorum with a retry dead-letter", func(t *testing.T) {
		queue := queueByName(t, queueWork)

		assert.True(t, queue.Durable)
		assert.Equal(t, queueTypeQuorum, queue.Args[argQueueType])
		assert.Equal(t, int32(workQueueMaxLength), queue.Args[argMaxLength])
		assert.Equal(t, overflowReject, queue.Args[argOverflow])
		assert.Equal(t, exchangeRetry, queue.Args[argDeadLetterExchange])
		assert.Equal(t, routingKey30s, queue.Args[argDeadLetterKey])
	})

	t.Run("Happy path: each delay tier carries its TTL and dead-letters back", func(t *testing.T) {
		tiers := []struct {
			name string
			ttl  int32
		}{
			{queueRetry30s, 30000},
			{queueRetry2m, 120000},
			{queueRetry5m, 300000},
		}
		for _, tier := range tiers {
			queue := queueByName(t, tier.name)
			assert.Equal(t, queueTypeQuorum, queue.Args[argQueueType], tier.name)
			assert.Equal(t, tier.ttl, queue.Args[argMessageTTL], tier.name)
			assert.Equal(t, exchangeMain, queue.Args[argDeadLetterExchange], tier.name)
			assert.Equal(t, routingKeyOTP, queue.Args[argDeadLetterKey], tier.name)
		}
	})

	t.Run("Happy path: the dead-letter queue is durable quorum", func(t *testing.T) {
		queue := queueByName(t, queueDead)
		assert.True(t, queue.Durable)
		assert.Equal(t, queueTypeQuorum, queue.Args[argQueueType])
	})
}

// TestEmailTopology_Declare proves the declaration reaches the seam with the
// exchanged set and bindings the topology describes, idempotently (EML-06).
func TestEmailTopology_Declare(t *testing.T) {
	fake := &fakeChannel{}
	require.NoError(t, emailTopology().declare(fake))

	t.Run("Happy path: both exchanges are declared direct and durable", func(t *testing.T) {
		require.Len(t, fake.exchanges, 2)
		for _, exchange := range fake.exchanges {
			assert.Equal(t, "direct", exchange.kind)
			assert.True(t, exchange.durable)
		}
	})

	t.Run("Happy path: every queue and binding is declared", func(t *testing.T) {
		assert.Len(t, fake.queues, 5)
		assert.Len(t, fake.bindings, 5)
	})

	t.Run("Happy path: the work queue is bound to the main exchange", func(t *testing.T) {
		found := false
		for _, binding := range fake.bindings {
			if binding.queue == queueWork && binding.exchange == exchangeMain && binding.key == routingKeyOTP {
				found = true
			}
		}
		assert.True(t, found, "the work queue must be bound to %s keyed %s", exchangeMain, routingKeyOTP)
	})
}

// TestRoute_MatchesTheDeclaredTopology proves every destination reaches a queue
// the topology declares and binds, so a routing key spelled in route can never
// drift from the declared bindings (WKR-09, WKR-11).
func TestRoute_MatchesTheDeclaredTopology(t *testing.T) {
	tests := []struct {
		name        string
		destination email.Destination
		wantQueue   string
	}{
		{name: "Happy path: the first retry tier", destination: email.DestinationRetry30s, wantQueue: queueRetry30s},
		{name: "Happy path: the second retry tier", destination: email.DestinationRetry2m, wantQueue: queueRetry2m},
		{name: "Happy path: the last retry tier", destination: email.DestinationRetry5m, wantQueue: queueRetry5m},
		{name: "Happy path: the dead-letter route", destination: email.DestinationDeadLetter, wantQueue: queueDead},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exchange, key, err := route(tc.destination)
			require.NoError(t, err)

			queue := queueByName(t, tc.wantQueue)
			assert.Contains(t, queue.Bindings, bindingSpec{Exchange: exchange, RoutingKey: key})
		})
	}

	t.Run("Edge: the work queue is never a destination", func(t *testing.T) {
		_, _, err := route(email.Destination(queueWork))
		assert.ErrorIs(t, err, ErrUnknownDestination)
	})
}
