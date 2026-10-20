package user_test

import (
	"encoding/json"
	"testing"
	"time"

	"legnaris/backend/internal/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUser_VerificationTimestamp guards the store-only field: a zero-value or
// freshly registered account carries no timestamp, and the wire DTO never
// exposes one (PRQ-12, PRQ-14, PRQ-15).
func TestUser_VerificationTimestamp(t *testing.T) {
	createdAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	t.Run("Edge: a zero-value account has no verification timestamp", func(t *testing.T) {
		var u user.User
		assert.Nil(t, u.EmailVerifiedAt)
	})

	t.Run("Edge: a registration-shaped account leaves the timestamp nil", func(t *testing.T) {
		u := user.User{
			ID:           "01900000-0000-7000-8000-000000000001",
			Email:        "player@example.com",
			Username:     "player",
			PasswordHash: "hash",
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}
		assert.Nil(t, u.EmailVerifiedAt, "registration must never set the verification timestamp")
	})

	t.Run("Happy path: the DTO exposes exactly id, email, username, email_verified and created_at", func(t *testing.T) {
		verifiedAt := createdAt.Add(time.Hour)
		u := user.User{
			ID:              "01900000-0000-7000-8000-000000000001",
			Email:           "player@example.com",
			Username:        "player",
			PasswordHash:    "hash",
			EmailVerifiedAt: &verifiedAt,
			CreatedAt:       createdAt,
			UpdatedAt:       createdAt,
		}

		dto := u.ToDTO()
		assert.Equal(t, u.ID, dto.ID)
		assert.Equal(t, u.Email, dto.Email)
		assert.Equal(t, u.Username, dto.Username)
		assert.Equal(t, u.CreatedAt, dto.CreatedAt)

		raw, err := json.Marshal(dto)
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw, &fields))
		assert.Len(t, fields, 5, "the DTO must not grow a field beyond the verification state")
		assert.NotContains(t, fields, "email_verified_at")
		assert.NotContains(t, string(raw), "password", "the hash must never be serialized")
	})
}

// TestUser_VerificationProjection covers the derived verification state on the
// wire shape: the boolean follows the stored instant, the instant itself never
// reaches the wire and the hash is never serialized (TFA-08, TFA-09).
func TestUser_VerificationProjection(t *testing.T) {
	createdAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	t.Run("Edge: an account that never verified reports false", func(t *testing.T) {
		u := user.User{
			ID:           "01900000-0000-7000-8000-000000000001",
			Email:        "player@example.com",
			Username:     "player",
			PasswordHash: "hash",
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		dto := u.ToDTO()
		assert.False(t, dto.EmailVerified)

		raw, err := json.Marshal(dto)
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw, &fields))
		assert.Equal(t, false, fields["email_verified"])
		assert.NotContains(t, fields, "email_verified_at", "the instant never reaches the wire")
		assert.NotContains(t, string(raw), "password", "the hash must never be serialized")
	})

	t.Run("Happy path: a verified account reports true and no instant", func(t *testing.T) {
		verifiedAt := createdAt.Add(time.Hour)
		u := user.User{
			ID:              "01900000-0000-7000-8000-000000000001",
			Email:           "player@example.com",
			Username:        "player",
			PasswordHash:    "hash",
			EmailVerifiedAt: &verifiedAt,
			CreatedAt:       createdAt,
			UpdatedAt:       createdAt,
		}

		dto := u.ToDTO()
		assert.True(t, dto.EmailVerified)

		raw, err := json.Marshal(dto)
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw, &fields))
		assert.Equal(t, true, fields["email_verified"])
		assert.NotContains(t, fields, "email_verified_at", "the instant never reaches the wire")
		assert.NotContains(t, string(raw), verifiedAt.Format(time.RFC3339), "the instant must not be disclosed")
		assert.NotContains(t, string(raw), "password", "the hash must never be serialized")
	})
}
