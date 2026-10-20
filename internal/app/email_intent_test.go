package app_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"legnaris/internal/app"
	"legnaris/internal/auth"
	"legnaris/internal/email/outbox"
	outboxmocks "legnaris/internal/email/outbox/mocks"
	"legnaris/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// captureLogger records every structured line so a suite can assert the code
// and the unmasked recipient never reach a log (OTP-15).
type captureLogger struct{ lines []string }

func (l *captureLogger) append(msg string, args []logger.Attribute) {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, msg)
	for _, attribute := range args {
		parts = append(parts, attribute.Key+"="+attribute.Value)
	}
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

func (l *captureLogger) text() string { return strings.Join(l.lines, "\n") }

// TestEmailIntent_Record proves the adapter maps the challenge's send intent
// onto the message contract exactly, never logs the code, and masks the
// recipient (OTP-10, OTP-12, OTP-15).
func TestEmailIntent_Record(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	const (
		intentID = "01900000-0000-7000-8000-0000000000aa"
		code     = "042917"
		to       = "player@example.com"
	)

	t.Run("Happy path: the intention carries the identity, recipient, template and payload", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		log := &captureLogger{}

		var stored outbox.Intent
		repo.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(intent outbox.Intent) bool {
				stored = intent
				return true
			})).
			Return(nil).
			Once()

		adapter := app.NewEmailIntent(repo, log)
		require.NoError(t, adapter.Record(ctx, intentID, to, auth.PurposeLoginOTP, code, 10, now))

		assert.Equal(t, intentID, stored.ID)
		assert.Equal(t, to, stored.To)
		assert.Equal(t, string(auth.PurposeLoginOTP), stored.Template)
		assert.Equal(t, code, stored.Payload.Code)
		assert.Equal(t, 10, stored.Payload.ExpiresInMinutes)
		assert.Equal(t, string(auth.PurposeLoginOTP), stored.Payload.Purpose)
		assert.Equal(t, outbox.StatusPending, stored.Status)
		assert.Equal(t, now, stored.NextAttemptAt)
	})

	t.Run("Edge: the recipient is masked and the code never reaches the log", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		log := &captureLogger{}
		repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

		adapter := app.NewEmailIntent(repo, log)
		require.NoError(t, adapter.Record(ctx, intentID, to, auth.PurposeVerifyEmail, code, 10, now))

		recorded := log.text()
		assert.Contains(t, recorded, "p***@example.com", "the log must mask the recipient")
		assert.NotContains(t, recorded, to, "the full address must never appear")
		assert.NotContains(t, recorded, code, "the code must never appear in a log line")
	})

	t.Run("Error: a non-canonical identity is rejected before the store", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t) // no expectations: the store is never reached

		adapter := app.NewEmailIntent(repo, &captureLogger{})
		err := adapter.Record(ctx, "not-a-uuid", to, auth.PurposeLoginOTP, code, 10, now)

		assert.ErrorIs(t, err, outbox.ErrInvalidIntentID)
	})

	t.Run("Error: a store failure is returned unwrapped", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		storeErr := errors.New("mongo down")
		repo.EXPECT().Create(mock.Anything, mock.Anything).Return(storeErr).Once()

		adapter := app.NewEmailIntent(repo, &captureLogger{})
		err := adapter.Record(ctx, intentID, to, auth.PurposeLoginOTP, code, 10, now)

		assert.ErrorIs(t, err, storeErr)
	})
}

// TestEmailIntent_Requeue proves a resend records a new intention and returns
// its fresh identity, never the original one (OTP-10).
func TestEmailIntent_Requeue(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	const (
		originalID = "01900000-0000-7000-8000-0000000000ab"
		clonedID   = "01900000-0000-7000-8000-0000000000ac"
	)

	t.Run("Happy path: the clone's fresh identity is returned", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		repo.EXPECT().
			Clone(mock.Anything, originalID, now).
			Return(outbox.Intent{ID: clonedID, To: "player@example.com"}, nil).
			Once()

		adapter := app.NewEmailIntent(repo, &captureLogger{})
		got, err := adapter.Requeue(ctx, originalID, now)

		require.NoError(t, err)
		assert.Equal(t, clonedID, got)
		assert.NotEqual(t, originalID, got, "the resend must not reuse the idempotency key")
	})

	t.Run("Error: an unknown identity propagates the typed not-found", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		repo.EXPECT().Clone(mock.Anything, originalID, now).Return(outbox.Intent{}, outbox.ErrIntentNotFound).Once()

		adapter := app.NewEmailIntent(repo, &captureLogger{})
		_, err := adapter.Requeue(ctx, originalID, now)

		assert.ErrorIs(t, err, outbox.ErrIntentNotFound)
	})
}

// TestEmailIntent_SentSince proves the daily-cap window reaches the store
// unchanged (OTP-13).
func TestEmailIntent_SentSince(t *testing.T) {
	ctx := context.Background()
	since := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)

	t.Run("Happy path: the window is forwarded and the count returned", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		repo.EXPECT().CountSince(mock.Anything, since).Return(7, nil).Once()

		adapter := app.NewEmailIntent(repo, &captureLogger{})
		count, err := adapter.SentSince(ctx, since)

		require.NoError(t, err)
		assert.Equal(t, 7, count)
	})

	t.Run("Error: a store failure is returned unwrapped", func(t *testing.T) {
		repo := outboxmocks.NewMockRepository(t)
		storeErr := errors.New("mongo down")
		repo.EXPECT().CountSince(mock.Anything, since).Return(0, storeErr).Once()

		adapter := app.NewEmailIntent(repo, &captureLogger{})
		_, err := adapter.SentSince(ctx, since)

		assert.ErrorIs(t, err, storeErr)
	})
}

// TestEmailIntent_DisclosureSweep asserts a six-digit code never appears in any
// line the adapter writes, whatever the outcome (OTP-15).
func TestEmailIntent_DisclosureSweep(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	const code = "000042"

	repo := outboxmocks.NewMockRepository(t)
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	log := &captureLogger{}
	adapter := app.NewEmailIntent(repo, log)
	require.NoError(t, adapter.Record(ctx, "01900000-0000-7000-8000-0000000000ad", "a@b.co", auth.PurposeLoginOTP, code, 10, now))

	assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), log.text())
}
