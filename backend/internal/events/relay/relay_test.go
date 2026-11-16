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
	"legnaris/backend/internal/sweeper"
	sweepermocks "legnaris/backend/internal/sweeper/mocks"
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
		Interval:       5 * time.Minute,
		Batch:          25,
		ConfirmTimeout: 5 * time.Second,
		AttemptCap:     5,
	}
}

func runQueue(t *testing.T, repo outbox.Repository, pub relay.Publisher, obs sweeper.Observer, cfg relay.Config) error {
	t.Helper()
	queue, err := relay.NewQueue(repo, pub, cfg)
	require.NoError(t, err)
	sw, err := sweeper.New([]sweeper.Queue{queue}, "relay-test", obs, logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	return sw.RunOnce(context.Background(), fixedNow)
}

func expectPass(repo *outboxmocks.MockRepository, obs *sweepermocks.MockObserver) {
	repo.EXPECT().OldestPending(mock.Anything).Return(time.Time{}, false, nil).Once()
	obs.EXPECT().OldestPending(relay.QueueName, time.Duration(0)).Once()
}

func TestNewQueue(t *testing.T) {
	repo := outboxmocks.NewMockRepository(t)
	pub := relaymocks.NewMockPublisher(t)

	tests := []struct {
		name    string
		repo    outbox.Repository
		pub     relay.Publisher
		mutate  func(cfg *relay.Config)
		wantErr bool
	}{
		{name: "Happy path: a complete config registers the queue", repo: repo, pub: pub, mutate: func(*relay.Config) {}},
		{name: "Edge: a nil repository is rejected", repo: nil, pub: pub, mutate: func(*relay.Config) {}, wantErr: true},
		{name: "Edge: a nil publisher is rejected", repo: repo, pub: nil, mutate: func(*relay.Config) {}, wantErr: true},
		{name: "Edge: a zero interval is rejected", repo: repo, pub: pub, mutate: func(c *relay.Config) { c.Interval = 0 }, wantErr: true},
		{name: "Edge: a zero batch is rejected", repo: repo, pub: pub, mutate: func(c *relay.Config) { c.Batch = 0 }, wantErr: true},
		{name: "Edge: a zero attempt cap is rejected", repo: repo, pub: pub, mutate: func(c *relay.Config) { c.AttemptCap = 0 }, wantErr: true},
		{name: "Edge: a zero confirm timeout is rejected", repo: repo, pub: pub, mutate: func(c *relay.Config) { c.ConfirmTimeout = 0 }, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			tc.mutate(&cfg)

			queue, err := relay.NewQueue(tc.repo, tc.pub, cfg)
			if tc.wantErr {
				assert.ErrorIs(t, err, sweeper.ErrInvalidQueue)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, relay.QueueName, queue.Name)
			assert.Equal(t, 5*time.Minute, queue.Interval)
		})
	}
}

func TestQueuePublish(t *testing.T) {
	t.Run("Happy path: a confirmed entry is marked published at the pass time", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)
		entry := newEntry(t, 0)

		repo.EXPECT().OldestPending(mock.Anything).Return(time.Time{}, false, nil).Once()
		obs.EXPECT().OldestPending(relay.QueueName, time.Duration(0)).Once()
		repo.EXPECT().Claim(mock.Anything, "relay-test", fixedNow, sweeper.Lease, 25).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, entry.ID, fixedNow).Return(nil).Once()

		require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
	})

	t.Run("Happy path: a batch publishes every claimed entry", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)
		first, second := newEntry(t, 0), newEntry(t, 0)

		expectPass(repo, obs)
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{first, second}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, first).Return(nil).Once()
		pub.EXPECT().Publish(mock.Anything, second).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, first.ID, fixedNow).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, second.ID, fixedNow).Return(nil).Once()

		require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
	})

	t.Run("Happy path: the publish runs under the confirm timeout", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)
		entry := newEntry(t, 0)

		expectPass(repo, obs)
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.MatchedBy(func(ctx context.Context) bool {
			deadline, ok := ctx.Deadline()
			return ok && time.Until(deadline) <= 5*time.Second
		}), entry).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, entry.ID, fixedNow).Return(nil).Once()

		require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
	})

	t.Run("Edge: an entry with a non-canonical identity is dead and never published", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)
		entry := outbox.Entry{ID: "not-a-uuid", Body: []byte(`{}`)}

		expectPass(repo, obs)
		obs.EXPECT().Failed(relay.QueueName).Once()
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		repo.EXPECT().MarkTerminal(mock.Anything, entry.ID, fixedNow, mock.MatchedBy(func(reason string) bool {
			return reason != ""
		})).Return(nil).Once()

		require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
	})

	t.Run("Edge: an empty claim does no publish and no state write", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)

		expectPass(repo, obs)
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil).Once()

		require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
	})

	t.Run("Edge: a mark-published failure reschedules the entry for republish", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)
		entry := newEntry(t, 0)

		expectPass(repo, obs)
		obs.EXPECT().Failed(relay.QueueName).Once()
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(nil).Once()
		repo.EXPECT().MarkPublished(mock.Anything, entry.ID, fixedNow).Return(errStoreFail).Once()
		repo.EXPECT().Reschedule(mock.Anything, entry.ID, fixedNow, fixedNow, errStoreFail.Error()).Return(nil).Once()

		require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
	})
}

func TestQueueFailures(t *testing.T) {
	tests := []struct {
		name       string
		attempts   int
		attemptCap int
		wantDead   bool
	}{
		{name: "Happy path: a first failed publish is rescheduled", attempts: 0, attemptCap: 5},
		{name: "Edge: the attempt just below the cap is rescheduled", attempts: 3, attemptCap: 5},
		{name: "Edge: the attempt that reaches the cap is dead", attempts: 4, attemptCap: 5, wantDead: true},
		{name: "Edge: a cap of one makes the first failure dead", attempts: 0, attemptCap: 1, wantDead: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := outboxmocks.NewMockRepository(t)
			pub := relaymocks.NewMockPublisher(t)
			obs := sweepermocks.NewMockObserver(t)
			entry := newEntry(t, tc.attempts)
			cfg := testConfig()
			cfg.AttemptCap = tc.attemptCap

			expectPass(repo, obs)
			obs.EXPECT().Failed(relay.QueueName).Once()
			repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]outbox.Entry{entry}, nil).Once()
			pub.EXPECT().Publish(mock.Anything, entry).Return(errBroker).Once()
			if tc.wantDead {
				repo.EXPECT().MarkTerminal(mock.Anything, entry.ID, fixedNow, errBroker.Error()).Return(nil).Once()
			} else {
				repo.EXPECT().Reschedule(mock.Anything, entry.ID, fixedNow, fixedNow, errBroker.Error()).Return(nil).Once()
			}

			require.NoError(t, runQueue(t, repo, pub, obs, cfg))
		})
	}
}

func TestQueueStoreErrors(t *testing.T) {
	t.Run("Error: a claim failure is returned and nothing is published", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)

		expectPass(repo, obs)
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errStoreFail).Once()

		assert.ErrorIs(t, runQueue(t, repo, pub, obs, testConfig()), errStoreFail)
	})

	t.Run("Edge: an oldest-pending probe failure is returned and the claim still runs", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)

		repo.EXPECT().OldestPending(mock.Anything).Return(time.Time{}, false, errStoreFail).Once()
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil).Once()

		assert.ErrorIs(t, runQueue(t, repo, pub, obs, testConfig()), errStoreFail)
	})

	t.Run("Edge: a reschedule failure is logged and the pass returns nil", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		pub := relaymocks.NewMockPublisher(t)
		obs := sweepermocks.NewMockObserver(t)
		entry := newEntry(t, 0)

		expectPass(repo, obs)
		obs.EXPECT().Failed(relay.QueueName).Once()
		repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]outbox.Entry{entry}, nil).Once()
		pub.EXPECT().Publish(mock.Anything, entry).Return(errBroker).Once()
		repo.EXPECT().Reschedule(mock.Anything, entry.ID, mock.Anything, mock.Anything, mock.Anything).
			Return(errStoreFail).Once()

		require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
	})
}

func TestQueueOldestPending(t *testing.T) {
	tests := []struct {
		name    string
		oldest  time.Time
		pending bool
		want    time.Duration
	}{
		{name: "Happy path: the age is the pass time minus the oldest creation", oldest: fixedNow.Add(-90 * time.Second), pending: true, want: 90 * time.Second},
		{name: "Edge: an entry created at the pass time is zero seconds old", oldest: fixedNow, pending: true, want: 0},
		{name: "Edge: an empty outbox reports zero", pending: false, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := outboxmocks.NewMockRepository(t)
			pub := relaymocks.NewMockPublisher(t)
			obs := sweepermocks.NewMockObserver(t)

			repo.EXPECT().OldestPending(mock.Anything).Return(tc.oldest, tc.pending, nil).Once()
			obs.EXPECT().OldestPending(relay.QueueName, tc.want).Once()
			repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, nil).Once()

			require.NoError(t, runQueue(t, repo, pub, obs, testConfig()))
		})
	}
}
