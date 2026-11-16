package relay_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/outbox"
	outboxmocks "legnaris/backend/internal/email/outbox/mocks"
	"legnaris/backend/internal/email/relay"
	relaymocks "legnaris/backend/internal/email/relay/mocks"
	"legnaris/backend/shared/logger"

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

func TestRelay_AttemptCapMarksTerminal(t *testing.T) {
	intent := relayIntent(t, 2)

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
