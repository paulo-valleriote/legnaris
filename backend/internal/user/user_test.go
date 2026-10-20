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

	t.Run("Happy path: the DTO exposes exactly id, email, username and created_at", func(t *testing.T) {
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
		assert.Len(t, fields, 4, "the DTO must not grow a verification field")
		assert.NotContains(t, fields, "email_verified_at")
		assert.NotContains(t, fields, "email_verified")
		assert.NotContains(t, string(raw), "password", "the hash must never be serialized")
	})
}
