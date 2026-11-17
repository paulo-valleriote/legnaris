package db_test

import (
	"context"
	"testing"

	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/require"
)

func TestEnsureIndexes_Campaigns(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))

	campaigns := listIndexes(t, mdb, "campaigns")

	t.Run("Happy path: campaigns are indexed by member user and status", func(t *testing.T) {
		_, ok := campaigns["campaigns_members_user_id_status"]
		require.True(t, ok, "the member lookup index must exist, got %v", campaigns)
	})

	t.Run("Happy path: invite codes are unique across campaigns", func(t *testing.T) {
		_, ok := campaigns["campaigns_invite_code_unique"]
		require.True(t, ok, "the invite code index must exist, got %v", campaigns)
	})

	t.Run("Edge: ensuring the campaign indexes twice is idempotent", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	})
}
