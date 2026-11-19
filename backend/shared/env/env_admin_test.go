package env

import (
	"testing"

	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	adminEnvOne = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"
	adminEnvTwo = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e10"
)

func TestAdminUserIDs(t *testing.T) {
	t.Run("Edge: unset is an empty allowlist", func(t *testing.T) {
		unsetEnv(t, "ADMIN_USER_IDS")
		e := newTestEnv()

		assert.Empty(t, e.Variables.AdminUserIDs)
		require.NoError(t, e.Validate())
	})

	t.Run("Edge: blanks and stray commas are dropped", func(t *testing.T) {
		t.Setenv("ADMIN_USER_IDS", " , ,")
		e := newTestEnv()

		assert.Empty(t, e.Variables.AdminUserIDs)
		require.NoError(t, e.Validate())
	})

	t.Run("Happy path: comma separated ids are trimmed and kept in order", func(t *testing.T) {
		t.Setenv("ADMIN_USER_IDS", " "+adminEnvOne+" , "+adminEnvTwo+" ")
		e := newTestEnv()

		assert.Equal(t, []string{adminEnvOne, adminEnvTwo}, e.Variables.AdminUserIDs)
		require.NoError(t, e.Validate())
	})

	t.Run("Error: a malformed id fails validation", func(t *testing.T) {
		t.Setenv("ADMIN_USER_IDS", adminEnvOne+",not-a-uuid")

		require.ErrorIs(t, newTestEnv().Validate(), idgen.ErrInvalidID)
	})

	t.Run("Error: a v4 id fails validation", func(t *testing.T) {
		t.Setenv("ADMIN_USER_IDS", "f47ac10b-58cc-4372-a567-0e02b2c3d479")

		require.Error(t, newTestEnv().Validate())
	})
}
