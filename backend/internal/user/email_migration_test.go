package user_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/user"
	"legnaris/backend/internal/user/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	migrationIDOne = "01900000-0000-7000-8000-000000000101"
	migrationIDTwo = "01900000-0000-7000-8000-000000000102"
)

// TestPlanEmailCanonicalization is the normative planner table: it turns stored
// rows into validated intent without touching the store (PRQ-11, SC-008).
func TestPlanEmailCanonicalization(t *testing.T) {
	t.Run("Happy path: already canonical rows plan nothing", func(t *testing.T) {
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "player@example.com"},
			{ID: migrationIDTwo, Email: "other@example.com"},
		})

		assert.Empty(t, plan.Changes)
		assert.Empty(t, plan.Collisions)
		assert.Empty(t, plan.Invalid)
		assert.False(t, plan.Blocked())
	})

	t.Run("Happy path: display name, case and whitespace plan one change each", func(t *testing.T) {
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "  Player@Example.COM  "},
			{ID: migrationIDTwo, Email: "Name <Other@Example.com>"},
		})

		assert.Equal(t, []user.EmailChange{
			{ID: migrationIDOne, From: "  Player@Example.COM  ", To: "player@example.com"},
			{ID: migrationIDTwo, From: "Name <Other@Example.com>", To: "other@example.com"},
		}, plan.Changes)
		assert.Empty(t, plan.Collisions)
		assert.Empty(t, plan.Invalid)
		assert.False(t, plan.Blocked())
	})

	t.Run("Edge: +tag and dotted spellings are not collapsed", func(t *testing.T) {
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "player+news@example.com"},
			{ID: migrationIDTwo, Email: "p.layer@example.com"},
		})

		assert.Empty(t, plan.Changes)
		assert.False(t, plan.Blocked())
	})

	t.Run("Error: two rows mapping to one canonical value are a collision", func(t *testing.T) {
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "Player@Example.com"},
			{ID: migrationIDTwo, Email: "player@example.com"},
		})

		require.Len(t, plan.Collisions, 1)
		assert.Equal(t, "player@example.com", plan.Collisions[0].Canonical)
		assert.Equal(t, []string{migrationIDOne, migrationIDTwo}, plan.Collisions[0].IDs)
		assert.True(t, plan.Blocked())
	})

	t.Run("Error: an unparseable row is invalid with an English reason", func(t *testing.T) {
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "not-an-email"},
		})

		require.Len(t, plan.Invalid, 1)
		assert.Equal(t, migrationIDOne, plan.Invalid[0].ID)
		assert.Equal(t, "not-an-email", plan.Invalid[0].Raw)
		assert.Equal(t, "address cannot be parsed", plan.Invalid[0].Reason)
		assert.True(t, plan.Blocked())
	})

	t.Run("Error: an over-long row is invalid with the length reason", func(t *testing.T) {
		overLong := strings.Repeat("a", user.MaxEmailLength-len("@example.com")+1) + "@example.com"
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: overLong},
		})

		require.Len(t, plan.Invalid, 1)
		assert.Equal(t, "address exceeds 254 bytes", plan.Invalid[0].Reason)
		assert.True(t, plan.Blocked())
	})

	t.Run("Edge: a finding blocks the whole plan even when changes exist", func(t *testing.T) {
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "Player@Example.com"},
			{ID: migrationIDTwo, Email: "broken"},
		})

		assert.NotEmpty(t, plan.Changes)
		assert.True(t, plan.Blocked())
	})
}

// TestEmailMigrationPlanApply proves the write gate: nothing is written for a
// blocked plan, a dry run never writes, and an apply sets only the address plus
// the supplied timestamp, one row at a time (PRQ-11, SC-008).
func TestEmailMigrationPlanApply(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	t.Run("Error: a blocked plan writes nothing", func(t *testing.T) {
		store := mocks.NewMockEmailMigrationStore(t)
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "Player@Example.com"},
			{ID: migrationIDTwo, Email: "player@example.com"},
		})
		require.True(t, plan.Blocked())

		written, err := plan.Apply(ctx, store, now, false)
		assert.ErrorIs(t, err, user.ErrMigrationBlocked)
		assert.Zero(t, written)
	})

	t.Run("Edge: a dry run reports zero writes and calls the store zero times", func(t *testing.T) {
		store := mocks.NewMockEmailMigrationStore(t)
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "Player@Example.com"},
		})
		require.False(t, plan.Blocked())

		written, err := plan.Apply(ctx, store, now, true)
		require.NoError(t, err)
		assert.Zero(t, written)
	})

	t.Run("Happy path: apply writes one row per change with the supplied time", func(t *testing.T) {
		store := mocks.NewMockEmailMigrationStore(t)
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "  Player@Example.COM  "},
			{ID: migrationIDTwo, Email: "Name <Other@Example.com>"},
		})
		require.False(t, plan.Blocked())

		store.EXPECT().UpdateEmail(mock.Anything, migrationIDOne, "player@example.com", now).Return(nil).Once()
		store.EXPECT().UpdateEmail(mock.Anything, migrationIDTwo, "other@example.com", now).Return(nil).Once()

		written, err := plan.Apply(ctx, store, now, false)
		require.NoError(t, err)
		assert.Equal(t, 2, written)
	})

	t.Run("Edge: no changes means no writes", func(t *testing.T) {
		store := mocks.NewMockEmailMigrationStore(t)
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "player@example.com"},
		})

		written, err := plan.Apply(ctx, store, now, false)
		require.NoError(t, err)
		assert.Zero(t, written)
	})

	t.Run("Edge: a second run over repaired rows plans zero changes", func(t *testing.T) {
		original := []user.EmailIdentity{
			{ID: migrationIDOne, Email: "  Player@Example.COM  "},
			{ID: migrationIDTwo, Email: "Name <Other@Example.com>"},
		}
		first := user.PlanEmailCanonicalization(original)
		require.Len(t, first.Changes, 2)

		repaired := make([]user.EmailIdentity, 0, len(original))
		for _, change := range first.Changes {
			repaired = append(repaired, user.EmailIdentity{ID: change.ID, Email: change.To})
		}

		second := user.PlanEmailCanonicalization(repaired)
		assert.Empty(t, second.Changes)
		assert.False(t, second.Blocked())
	})

	t.Run("Error: a store failure surfaces and stops the run", func(t *testing.T) {
		store := mocks.NewMockEmailMigrationStore(t)
		plan := user.PlanEmailCanonicalization([]user.EmailIdentity{
			{ID: migrationIDOne, Email: "Player@Example.com"},
			{ID: migrationIDTwo, Email: "Other@Example.com"},
		})

		store.EXPECT().UpdateEmail(mock.Anything, migrationIDOne, "player@example.com", now).Return(nil).Once()
		store.EXPECT().UpdateEmail(mock.Anything, migrationIDTwo, "other@example.com", now).Return(errors.New("mongo down")).Once()

		written, err := plan.Apply(ctx, store, now, false)
		require.Error(t, err)
		assert.Equal(t, 1, written, "the prefix already written is reported")
		assert.NotErrorIs(t, err, user.ErrMigrationBlocked)
	})
}
