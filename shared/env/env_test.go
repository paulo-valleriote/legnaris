package env

import (
	"log/slog"
	"os"
	"testing"

	"legnaris/shared/logger"

	"github.com/stretchr/testify/require"
)

func newTestEnv() Env {
	return NewEnv(logger.NewLogger(slog.LevelError))
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, found := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env %s: %v", key, err)
	}
	t.Cleanup(func() {
		if found {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestLookupStringEnv(t *testing.T) {
	e := newTestEnv()

	t.Run("unset falls back to default", func(t *testing.T) {
		unsetEnv(t, "PORT")
		require.Equal(t, "8080", e.lookupStringEnv("PORT", "8080"))
	})

	t.Run("empty value falls back to default", func(t *testing.T) {
		t.Setenv("PORT", "")
		require.Equal(t, "8080", e.lookupStringEnv("PORT", "8080"))
	})

	t.Run("set value is respected", func(t *testing.T) {
		t.Setenv("PORT", "9090")
		require.Equal(t, "9090", e.lookupStringEnv("PORT", "8080"))
	})
}
