package outbox_test

import (
	"testing"
	"time"

	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleNow() time.Time {
	return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
}

func samplePayload() outbox.Payload {
	return outbox.Payload{Code: "042917", ExpiresInMinutes: 10, Purpose: "login_otp"}
}

// TestNewIntent_GeneratesIdentity pins the created row's invariants: a
// canonical UUID v7 identity and a pending, immediately eligible, unleased row
// (EML-01, EML-18).
func TestNewIntent_GeneratesIdentity(t *testing.T) {
	now := sampleNow()
	intent, err := outbox.NewIntent("", "player@example.com", "login_otp", samplePayload(), now)
	require.NoError(t, err)

	t.Run("Happy path: the generated identity is a canonical UUID v7", func(t *testing.T) {
		require.NoError(t, idgen.Validate(intent.ID))
	})

	t.Run("Happy path: the row starts pending and immediately eligible", func(t *testing.T) {
		assert.Equal(t, outbox.StatusPending, intent.Status)
		assert.Equal(t, 0, intent.Attempts)
		assert.Equal(t, now, intent.NextAttemptAt)
		assert.Equal(t, now, intent.CreatedAt)
	})

	t.Run("Edge: no lease and no publication is recorded at creation", func(t *testing.T) {
		assert.Nil(t, intent.LockedUntil)
		assert.Empty(t, intent.LockedBy)
		assert.Nil(t, intent.PublishedAt)
	})

	t.Run("Happy path: the payload round-trips", func(t *testing.T) {
		assert.Equal(t, samplePayload(), intent.Payload)
		assert.Equal(t, "042917", intent.Payload.Code)
	})
}

// TestNewIntent_AcceptsSuppliedIdentity proves a caller-supplied canonical
// identity is honoured as the provider idempotency key (EML-18).
func TestNewIntent_AcceptsSuppliedIdentity(t *testing.T) {
	const id = "018f4a1e-5c6b-7d8e-9f01-23456789abcd"

	intent, err := outbox.NewIntent(id, "player@example.com", "verify_email", samplePayload(), sampleNow())
	require.NoError(t, err)
	assert.Equal(t, id, intent.ID)
}

// TestNewIntent_RejectsNonCanonicalIdentity: a supplied identity that is not
// exactly what backend/shared/idgen.New produces is rejected, never stored (EML-18).
func TestNewIntent_RejectsNonCanonicalIdentity(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "Error: garbage string", id: "not-a-uuid"},
		{name: "Error: uppercase", id: "018F4A1E-5C6B-7D8E-9F01-23456789ABCD"},
		{name: "Error: UUID v4", id: "9f2c8a1b-3d4e-4f60-8a1b-2c3d4e5f6071"},
		{name: "Error: urn encoding", id: "urn:uuid:018f4a1e-5c6b-7d8e-9f01-23456789abcd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			intent, err := outbox.NewIntent(tc.id, "player@example.com", "login_otp", samplePayload(), sampleNow())

			require.Error(t, err)
			assert.ErrorIs(t, err, outbox.ErrInvalidIntentID)
			assert.Empty(t, intent.ID, "a rejected identity must not produce a row")
		})
	}
}

// TestNewIntent_GeneratesDistinctIdentities guards against a constant identity
// masquerading as generated (EML-18).
func TestNewIntent_GeneratesDistinctIdentities(t *testing.T) {
	first, err := outbox.NewIntent("", "a@example.com", "login_otp", samplePayload(), sampleNow())
	require.NoError(t, err)
	second, err := outbox.NewIntent("", "a@example.com", "login_otp", samplePayload(), sampleNow())
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID, "each intention must get its own identity")
}

// TestNewIntent_DefaultsToInitialKind pins that a freshly recorded intention is
// a first send, so only resends written by Clone count against the resend share
// (D5).
func TestNewIntent_DefaultsToInitialKind(t *testing.T) {
	intent, err := outbox.NewIntent("", "player@example.com", "verify_email", samplePayload(), sampleNow())
	require.NoError(t, err)

	assert.Equal(t, outbox.KindInitial, intent.Kind)
}
