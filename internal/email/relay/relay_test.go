package relay_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/internal/email"
	"legnaris/internal/email/outbox"
	outboxmocks "legnaris/internal/email/outbox/mocks"
	"legnaris/internal/email/relay"
	relaymocks "legnaris/internal/email/relay/mocks"
	"legnaris/shared/logger"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func relayNow() time.Time {
	return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
}

func relayConfig() relay.Config {
	now := relayNow()
	return relay.Config{
		PollInterval:   time.Hour,
		Lease:          30 * time.Second,
		Batch:          10,
		ConfirmTimeout: 5 * time.Second,
		MaxAttempts:    3,
		Backoff:        []time.Duration{5 * time.Second, 15 * time.Second, time.Minute},
		Now:            func() time.Time { return now },
		ClaimerID:      "relay-test",
	}
}

func relayIntent(t *testing.T, attempts int) outbox.Intent {
	t.Helper()

	intent, err := outbox.NewIntent("", "player@example.com", "login_otp",
		outbox.Payload{Code: "042917", ExpiresInMinutes: 10, Purpose: "login_otp"}, relayNow())
	require.NoError(t, err)
	intent.Attempts = attempts
	return intent
}

func newRelay(t *testing.T, repo outbox.Repository, publisher relay.Publisher, observer relay.Observer) *relay.Relay {
	t.Helper()
	return relay.New(repo, publisher, observer, relayConfig(), logger.NewLogger(slog.LevelError))
}

// TestRelay_PublishConfirmed proves a confirmed publish marks the row published
// and is reported as published (EML-02, EML-23).
func TestRelay_PublishConfirmed(t *testing.T) {
	intent := relayIntent(t, 0)

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, "relay-test", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(1, nil).Once()
	publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(m email.Message) bool {
		return m.ID == intent.ID && m.Attempt == 0 && m.Type == email.TypeOTP && m.Data.Code == "042917"
	})).Return(nil).Once()
	repo.EXPECT().MarkPublished(mock.Anything, intent.ID, relayNow()).Return(nil).Once()
	observer.EXPECT().BacklogPending(1).Return().Once()
	observer.EXPECT().PublishResult(relay.ResultPublished).Return().Once()

	newRelay(t, repo, publisher, observer).RunOnce(context.Background())
}

// TestRelay_TransientReschedules proves a transient publish failure reschedules
// with the first backoff tier and is reported as retried (EML-04, EML-23).
func TestRelay_TransientReschedules(t *testing.T) {
	intent := relayIntent(t, 0)

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(1, nil).Once()
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).
		Return(errors.New("broker nacked")).Once()
	repo.EXPECT().Reschedule(mock.Anything, intent.ID, relayNow(), relayNow().Add(5*time.Second), "broker nacked").
		Return(nil).Once()
	observer.EXPECT().BacklogPending(1).Return().Once()
	observer.EXPECT().PublishResult(relay.ResultRetried).Return().Once()

	newRelay(t, repo, publisher, observer).RunOnce(context.Background())
}

// TestRelay_SecondFailureUsesSecondTier proves the backoff schedule advances
// with the attempt count (EML-04).
func TestRelay_SecondFailureUsesSecondTier(t *testing.T) {
	intent := relayIntent(t, 1)

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(1, nil).Once()
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("broker nacked")).Once()
	repo.EXPECT().Reschedule(mock.Anything, intent.ID, relayNow(), relayNow().Add(15*time.Second), "broker nacked").
		Return(nil).Once()
	observer.EXPECT().BacklogPending(1).Return().Once()
	observer.EXPECT().PublishResult(relay.ResultRetried).Return().Once()

	newRelay(t, repo, publisher, observer).RunOnce(context.Background())
}

// TestRelay_AttemptCapMarksTerminal proves reaching the publication-attempt cap
// fails the row instead of rescheduling it (EML-04).
func TestRelay_AttemptCapMarksTerminal(t *testing.T) {
	intent := relayIntent(t, 2) // MaxAttempts is 3, so the next failure is the third.

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(1, nil).Once()
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("broker nacked")).Once()
	repo.EXPECT().MarkTerminal(mock.Anything, intent.ID, relayNow(), "broker nacked").Return(nil).Once()
	observer.EXPECT().BacklogPending(1).Return().Once()
	observer.EXPECT().PublishResult(relay.ResultFailed).Return().Once()

	newRelay(t, repo, publisher, observer).RunOnce(context.Background())
}

// TestRelay_InvalidIdentityMarksTerminal proves a non-canonical identity is
// failed without ever being published (EML-18, EML-19).
func TestRelay_InvalidIdentityMarksTerminal(t *testing.T) {
	intent := relayIntent(t, 0)
	intent.ID = "not-a-uuid"

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(1, nil).Once()
	repo.EXPECT().MarkTerminal(mock.Anything, intent.ID, relayNow(), mock.Anything).Return(nil).Once()
	observer.EXPECT().BacklogPending(1).Return().Once()
	observer.EXPECT().PublishResult(relay.ResultFailed).Return().Once()

	newRelay(t, repo, publisher, observer).RunOnce(context.Background())

	publisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

// TestRelay_CrashBetweenPublishAndMark proves the same identity is republished
// after a crash between publish and mark and converges to published; the
// duplicate is harmless because the identity is stable (EML-03, EML-18, SC-001).
func TestRelay_CrashBetweenPublishAndMark(t *testing.T) {
	intent := relayIntent(t, 0)

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Twice()
	repo.EXPECT().Backlog(mock.Anything).Return(1, nil).Twice()
	publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(m email.Message) bool {
		return m.ID == intent.ID
	})).Return(nil).Twice()
	repo.EXPECT().MarkPublished(mock.Anything, intent.ID, relayNow()).
		Return(errors.New("write lost")).Once()
	repo.EXPECT().MarkPublished(mock.Anything, intent.ID, relayNow()).
		Return(nil).Once()
	observer.EXPECT().BacklogPending(1).Return().Twice()
	observer.EXPECT().PublishResult(relay.ResultPublished).Return().Twice()

	relayUnderTest := newRelay(t, repo, publisher, observer)
	relayUnderTest.RunOnce(context.Background())
	relayUnderTest.RunOnce(context.Background())
}

// TestRelay_TwoRelaysNoDoublePublish models the atomic claim: the shared store
// gives the row to exactly one relay, so the identity is published once
// (EML-03, SC-002).
func TestRelay_TwoRelaysNoDoublePublish(t *testing.T) {
	intent := relayIntent(t, 0)

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Once()
	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(0, nil).Twice()
	publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(m email.Message) bool {
		return m.ID == intent.ID
	})).Return(nil).Once()
	repo.EXPECT().MarkPublished(mock.Anything, intent.ID, relayNow()).Return(nil).Once()
	observer.EXPECT().BacklogPending(0).Return().Twice()
	observer.EXPECT().PublishResult(relay.ResultPublished).Return().Once()

	first := newRelay(t, repo, publisher, observer)
	second := newRelay(t, repo, publisher, observer)
	first.RunOnce(context.Background())
	second.RunOnce(context.Background())

	publisher.AssertNumberOfCalls(t, "Publish", 1)
}

// TestRelay_EmptyOutbox proves an empty outbox is a no-op, not an error
// (EML-05).
func TestRelay_EmptyOutbox(t *testing.T) {
	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(0, nil).Once()
	observer.EXPECT().BacklogPending(0).Return().Once()

	newRelay(t, repo, publisher, observer).RunOnce(context.Background())

	publisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

// TestRelay_CancelledContextDrainsBatch proves Run finishes the in-flight batch
// and returns once the context is cancelled (EML-05).
func TestRelay_CancelledContextDrainsBatch(t *testing.T) {
	intent := relayIntent(t, 0)

	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]outbox.Intent{intent}, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(1, nil).Once()
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
	repo.EXPECT().MarkPublished(mock.Anything, intent.ID, relayNow()).Return(nil).Once()
	observer.EXPECT().BacklogPending(1).Return().Once()
	observer.EXPECT().PublishResult(relay.ResultPublished).Return().Once()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, newRelay(t, repo, publisher, observer).Run(ctx))
}

// TestRelay_ConfigDefaults proves a partial configuration is filled from the
// documented defaults before the store is queried (EML-04, EML-05).
func TestRelay_ConfigDefaults(t *testing.T) {
	repo := outboxmocks.NewMockRepository(t)
	publisher := relaymocks.NewMockPublisher(t)
	observer := relaymocks.NewMockObserver(t)

	repo.EXPECT().Claim(mock.Anything, "relay-zero", mock.Anything, relay.DefaultLease, relay.DefaultBatch).
		Return(nil, nil).Once()
	repo.EXPECT().Backlog(mock.Anything).Return(0, nil).Once()
	observer.EXPECT().BacklogPending(0).Return().Once()

	relayUnderTest := relay.New(repo, publisher, observer, relay.Config{ClaimerID: "relay-zero"}, logger.NewLogger(slog.LevelError))
	relayUnderTest.RunOnce(context.Background())
}
