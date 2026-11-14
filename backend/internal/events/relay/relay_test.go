package relay_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/events/outbox"
	outboxmocks "legnaris/backend/internal/events/outbox/mocks"
	"legnaris/backend/internal/events/relay"
	relaymocks "legnaris/backend/internal/events/relay/mocks"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	fixedNow     = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	errBroker    = errors.New("broker nacked")
	errStoreFail = errors.New("store unavailable")
)

func newEntry(t *testing.T, attempts int) outbox.Entry {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return outbox.Entry{
		ID:        id,
		EventType: "user.deleted",
		Body:      []byte(`{"id":"x"}`),
		Status:    outbox.StatusPending,
		Attempts:  attempts,
	}
}

func testConfig() relay.Config {
	return relay.Config{
		Now:         func() time.Time { return fixedNow },
		ClaimerID:   "relay-test",
		MaxAttempts: 5,
	}
}

func newRelay(repo outbox.Repository, pub relay.Publisher, cfg relay.Config) *relay.Relay {
	return relay.New(repo, pub, cfg, logger.NewLogger(slog.LevelError))
}

// TestRelayRunOnce covers one publication pass over a claimed batch.
func TestRelayRunOnce(t *testing.T) {
	t.Run("Happy path: a confirmed entry is marked published", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		entry := newEntry(t, 0)

		repo.EXPECT().Claim(mock.Anything, "relay-test", fixedNow, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, entry.ID, fixedNow).Return(nil).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Happy path: a batch publishes every claimed entry", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		first, second := newEntry(t, 0), newEntry(t, 0)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{first, second}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, first).Return(nil).Once()
		pub.EXPECT().Publish(mock.Anything, second).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, first.ID, fixedNow).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, second.ID, fixedNow).Return(nil).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Error: a failed publish reschedules with the first backoff tier", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		entry := newEntry(t, 0)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(errBroker).Once()
		repo.EXPECT().Reschedule(mock.Anything, entry.ID, fixedNow, fixedNow.Add(5*time.Second), errBroker.Error()).
			Return(nil).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Error: the attempt that reaches the cap marks the entry terminal", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		entry := newEntry(t, 4)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(errBroker).Once()
		repo.EXPECT().MarkTerminal(mock.Anything, entry.ID, fixedNow, errBroker.Error()).Return(nil).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Edge: an entry with a non-canonical identity is terminal and never published", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		entry := outbox.Entry{ID: "not-a-uuid", Body: []byte(`{}`)}

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		repo.EXPECT().MarkTerminal(mock.Anything, entry.ID, fixedNow, "invalid identity").Return(nil).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Edge: an empty claim does no publish and no state write", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Error: a claim failure stops the pass without publishing", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errStoreFail).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Edge: a mark-published failure is logged and does not panic the pass", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		entry := newEntry(t, 0)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, entry.ID, fixedNow).Return(errStoreFail).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})

	t.Run("Edge: a reschedule failure is logged and the entry is left for the next claim", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		entry := newEntry(t, 0)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(errBroker).Once()
		repo.EXPECT().Reschedule(mock.Anything, entry.ID, mock.Anything, mock.Anything, mock.Anything).
			Return(errStoreFail).Once()

		newRelay(repo, pub, testConfig()).RunOnce(context.Background())
	})
}

// TestRelayBackoff covers the bounded backoff schedule: each attempt waits the
// next tier, and the last tier repeats once the schedule runs out.
func TestRelayBackoff(t *testing.T) {
	tests := []struct {
		name     string
		attempts int
		want     time.Duration
	}{
		{name: "Happy path: the first failure waits the first tier", attempts: 0, want: 5 * time.Second},
		{name: "Happy path: the second failure waits the second tier", attempts: 1, want: 15 * time.Second},
		{name: "Happy path: the third failure waits the third tier", attempts: 2, want: time.Minute},
		{name: "Edge: the last tier is reached on the fourth failure", attempts: 3, want: 5 * time.Minute},
		{name: "Edge: the last tier repeats past the end of the schedule", attempts: 20, want: 5 * time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := outboxmocks.NewMockRepository(t)
			pub := relaymocks.NewMockPublisher(t)
			entry := newEntry(t, tc.attempts)

			repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]outbox.Entry{entry}, nil).Once()
			pub.EXPECT().Publish(mock.Anything, entry).Return(errBroker).Once()
			repo.EXPECT().Reschedule(mock.Anything, entry.ID, fixedNow, fixedNow.Add(tc.want), errBroker.Error()).
				Return(nil).Once()

			cfg := testConfig()
			cfg.MaxAttempts = 100
			newRelay(repo, pub, cfg).RunOnce(context.Background())
		})
	}
}

// TestRelayRun covers the polling loop's shutdown behaviour.
func TestRelayRun(t *testing.T) {
	t.Run("Happy path: a cancelled context finishes one pass and returns nil", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)

		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil).Once()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.NoError(t, newRelay(repo, pub, testConfig()).Run(ctx))
	})
}
