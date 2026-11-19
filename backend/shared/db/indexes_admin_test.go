package db_test

import (
	"context"
	"testing"

	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureIndexes_AdminAudit(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()
	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	audit := listIndexes(t, mdb, "admin_audit")

	t.Run("Happy path: the audit timeline is indexed by creation time", func(t *testing.T) {
		_, ok := audit["admin_audit_created_at"]
		require.True(t, ok, "the created_at index must exist, got %v", audit)
		assert.Equal(t, []string{"created_at"}, indexKeyNames(t, mdb, "admin_audit", "admin_audit_created_at"))
	})

	t.Run("Happy path: target and actor histories are indexed", func(t *testing.T) {
		_, ok := audit["admin_audit_target_created_at"]
		require.True(t, ok, "the target timeline index must exist, got %v", audit)
		assert.Equal(t, []string{"target_user_id", "created_at"},
			indexKeyNames(t, mdb, "admin_audit", "admin_audit_target_created_at"))

		_, ok = audit["admin_audit_actor_created_at"]
		require.True(t, ok, "the actor timeline index must exist, got %v", audit)
		assert.Equal(t, []string{"actor_user_id", "created_at"},
			indexKeyNames(t, mdb, "admin_audit", "admin_audit_actor_created_at"))
	})

	t.Run("Edge: calling twice is idempotent", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexes(ctx, mdb))
	})
}
