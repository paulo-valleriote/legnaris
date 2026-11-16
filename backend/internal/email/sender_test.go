package email_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type senderFunc func(ctx context.Context, m email.Outbound) error

func (f senderFunc) Send(ctx context.Context, m email.Outbound) error { return f(ctx, m) }

func TestSenderClassification(t *testing.T) {
	t.Run("Happy path: the sentinels are distinct", func(t *testing.T) {
		assert.NotErrorIs(t, email.ErrTransient, email.ErrPermanent)
		assert.NotErrorIs(t, email.ErrPermanent, email.ErrTransient)
	})

	t.Run("Happy path: an adapter error wrapping ErrTransient is recognised", func(t *testing.T) {
		err := errors.Join(errors.New("context"), email.ErrTransient)
		assert.ErrorIs(t, err, email.ErrTransient)
		assert.NotErrorIs(t, err, email.ErrPermanent)
	})

	t.Run("Happy path: an adapter error wrapping ErrPermanent is recognised", func(t *testing.T) {
		err := errors.Join(errors.New("invalid recipient"), email.ErrPermanent)
		assert.ErrorIs(t, err, email.ErrPermanent)
		assert.NotErrorIs(t, err, email.ErrTransient)
	})

	t.Run("Edge: an unrelated error matches neither sentinel", func(t *testing.T) {
		err := errors.New("some other failure")
		assert.NotErrorIs(t, err, email.ErrTransient)
		assert.NotErrorIs(t, err, email.ErrPermanent)
	})
}

func TestSenderPort(t *testing.T) {
	var captured email.Outbound
	var sender email.Sender = senderFunc(func(_ context.Context, m email.Outbound) error {
		captured = m
		return nil
	})

	outbound := email.Outbound{
		IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
		To:             "player@example.com",
		Subject:        "Your Legnaris code",
		Text:           "code 042917",
		HTML:           "<p>code 042917</p>",
	}

	require.NoError(t, sender.Send(context.Background(), outbound))

	t.Run("Happy path: the outbound value round-trips through the port", func(t *testing.T) {
		assert.Equal(t, outbound, captured)
	})

	t.Run("Edge: the idempotency key is the stable identity", func(t *testing.T) {
		assert.Equal(t, outbound.IdempotencyKey, captured.IdempotencyKey)
	})
}
