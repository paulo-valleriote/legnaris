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

func TestLookupStringSliceEnv(t *testing.T) {
	e := newTestEnv()

	t.Run("unset falls back to a cloned default", func(t *testing.T) {
		unsetEnv(t, "CORS_ALLOWED_ORIGINS")
		got := e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", []string{"https://a.example"})
		require.Equal(t, []string{"https://a.example"}, got)
	})

	t.Run("trims whitespace, drops empties, and strips trailing slash", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", " https://a.example/ , ,https://b.example, ")
		got := e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", nil)
		require.Equal(t, []string{"https://a.example", "https://b.example"}, got)
	})

	t.Run("returned slice does not alias the default", func(t *testing.T) {
		unsetEnv(t, "CORS_ALLOWED_ORIGINS")
		def := []string{"https://a.example"}
		got := e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", def)
		got[0] = "https://mutated.example"
		require.Equal(t, "https://a.example", def[0], "mutating the result must not mutate the default")
	})
}
