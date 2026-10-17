package env

import (
	"log/slog"
	"os"
	"testing"
	"time"

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

func TestLookupDurationEnv(t *testing.T) {
	e := newTestEnv()
	const defaultDur = 2 * time.Second

	t.Run("unset falls back to default", func(t *testing.T) {
		unsetEnv(t, "TEST_DURATION")
		require.Equal(t, defaultDur, e.lookupDurationEnv("TEST_DURATION", defaultDur))
	})

	t.Run("empty value falls back to default", func(t *testing.T) {
		t.Setenv("TEST_DURATION", "")
		require.Equal(t, defaultDur, e.lookupDurationEnv("TEST_DURATION", defaultDur))
	})

	t.Run("valid duration is parsed", func(t *testing.T) {
		t.Setenv("TEST_DURATION", "3s")
		require.Equal(t, 3*time.Second, e.lookupDurationEnv("TEST_DURATION", defaultDur))
	})

	t.Run("valid duration with ms is parsed", func(t *testing.T) {
		t.Setenv("TEST_DURATION", "500ms")
		require.Equal(t, 500*time.Millisecond, e.lookupDurationEnv("TEST_DURATION", defaultDur))
	})

	t.Run("invalid duration falls back to default", func(t *testing.T) {
		t.Setenv("TEST_DURATION", "not-a-duration")
		require.Equal(t, defaultDur, e.lookupDurationEnv("TEST_DURATION", defaultDur))
	})

	t.Run("zero duration falls back to default", func(t *testing.T) {
		t.Setenv("TEST_DURATION", "0s")
		require.Equal(t, defaultDur, e.lookupDurationEnv("TEST_DURATION", defaultDur))
	})

	t.Run("negative duration falls back to default", func(t *testing.T) {
		t.Setenv("TEST_DURATION", "-1s")
		require.Equal(t, defaultDur, e.lookupDurationEnv("TEST_DURATION", defaultDur))
	})
}

func TestLookupRequiredStringEnv(t *testing.T) {
	e := newTestEnv()

	t.Run("unset returns empty string", func(t *testing.T) {
		unsetEnv(t, "REQUIRED_VAR")
		require.Empty(t, e.lookupRequiredStringEnv("REQUIRED_VAR"))
	})

	t.Run("empty value returns empty string", func(t *testing.T) {
		t.Setenv("REQUIRED_VAR", "")
		require.Empty(t, e.lookupRequiredStringEnv("REQUIRED_VAR"))
	})

	t.Run("set value is returned", func(t *testing.T) {
		t.Setenv("REQUIRED_VAR", "my-value")
		require.Equal(t, "my-value", e.lookupRequiredStringEnv("REQUIRED_VAR"))
	})
}

func TestInitVariablesDefaults(t *testing.T) {
	e := newTestEnv()

	t.Run("DatabaseURI is empty when unset (required)", func(t *testing.T) {
		unsetEnv(t, "DATABASE_URI")
		vars := e.initVariables()
		require.Empty(t, vars.DatabaseURI)
	})

	t.Run("DatabaseName defaults to legnaris", func(t *testing.T) {
		unsetEnv(t, "DATABASE_NAME")
		vars := e.initVariables()
		require.Equal(t, "legnaris", vars.DatabaseName)
	})

	t.Run("DatabasePingTimeout defaults to 1s", func(t *testing.T) {
		unsetEnv(t, "DATABASE_PING_TIMEOUT")
		vars := e.initVariables()
		require.Equal(t, 1*time.Second, vars.DatabasePingTimeout)
	})

	t.Run("DatabaseServerSelectionTimeout defaults to 2s", func(t *testing.T) {
		unsetEnv(t, "DATABASE_SERVER_SELECTION_TIMEOUT")
		vars := e.initVariables()
		require.Equal(t, 2*time.Second, vars.DatabaseServerSelectionTimeout)
	})

	t.Run("DatabaseConnectTimeout defaults to 2s", func(t *testing.T) {
		unsetEnv(t, "DATABASE_CONNECT_TIMEOUT")
		vars := e.initVariables()
		require.Equal(t, 2*time.Second, vars.DatabaseConnectTimeout)
	})

	t.Run("DatabaseCloseTimeout defaults to 5s", func(t *testing.T) {
		unsetEnv(t, "DATABASE_CLOSE_TIMEOUT")
		vars := e.initVariables()
		require.Equal(t, 5*time.Second, vars.DatabaseCloseTimeout)
	})

	t.Run("JWTSecret is empty when unset (required)", func(t *testing.T) {
		unsetEnv(t, "JWT_SECRET")
		vars := e.initVariables()
		require.Empty(t, vars.JWTSecret)
	})

	t.Run("JWTAccessTTL defaults to 15m", func(t *testing.T) {
		unsetEnv(t, "JWT_ACCESS_TTL")
		vars := e.initVariables()
		require.Equal(t, 15*time.Minute, vars.JWTAccessTTL)
	})

	t.Run("AuthRefreshTTL defaults to 720h", func(t *testing.T) {
		unsetEnv(t, "AUTH_REFRESH_TTL")
		vars := e.initVariables()
		require.Equal(t, 720*time.Hour, vars.AuthRefreshTTL)
	})

	t.Run("AuthBcryptCost defaults to 12", func(t *testing.T) {
		unsetEnv(t, "AUTH_BCRYPT_COST")
		vars := e.initVariables()
		require.Equal(t, 12, vars.AuthBcryptCost)
	})

	t.Run("AuthRateLimitAttempts defaults to 10", func(t *testing.T) {
		unsetEnv(t, "AUTH_RATE_LIMIT_ATTEMPTS")
		vars := e.initVariables()
		require.Equal(t, 10, vars.AuthRateLimitAttempts)
	})

	t.Run("AuthRateLimitWindow defaults to 1m", func(t *testing.T) {
		unsetEnv(t, "AUTH_RATE_LIMIT_WINDOW")
		vars := e.initVariables()
		require.Equal(t, time.Minute, vars.AuthRateLimitWindow)
	})
}

// TestInitVariablesRateLimitOverrides covers the override path and the
// invalid-value fallback for the rate-limit pair: an absent or unusable value
// keeps the documented default rather than a magic zero (AUTH-20, AUTH-24).
func TestInitVariablesRateLimitOverrides(t *testing.T) {
	e := newTestEnv()

	t.Run("Happy path: valid values override the defaults", func(t *testing.T) {
		t.Setenv("AUTH_RATE_LIMIT_ATTEMPTS", "25")
		t.Setenv("AUTH_RATE_LIMIT_WINDOW", "2m")
		vars := e.initVariables()
		require.Equal(t, 25, vars.AuthRateLimitAttempts)
		require.Equal(t, 2*time.Minute, vars.AuthRateLimitWindow)
	})

	t.Run("Edge: a zero attempts value falls back to 10", func(t *testing.T) {
		t.Setenv("AUTH_RATE_LIMIT_ATTEMPTS", "0")
		unsetEnv(t, "AUTH_RATE_LIMIT_WINDOW")
		vars := e.initVariables()
		require.Equal(t, 10, vars.AuthRateLimitAttempts)
	})

	t.Run("Edge: a negative attempts value falls back to 10", func(t *testing.T) {
		t.Setenv("AUTH_RATE_LIMIT_ATTEMPTS", "-3")
		vars := e.initVariables()
		require.Equal(t, 10, vars.AuthRateLimitAttempts)
	})

	t.Run("Edge: a non-numeric attempts value falls back to 10", func(t *testing.T) {
		t.Setenv("AUTH_RATE_LIMIT_ATTEMPTS", "many")
		vars := e.initVariables()
		require.Equal(t, 10, vars.AuthRateLimitAttempts)
	})

	t.Run("Edge: an invalid window falls back to 1m", func(t *testing.T) {
		t.Setenv("AUTH_RATE_LIMIT_WINDOW", "not-a-duration")
		vars := e.initVariables()
		require.Equal(t, time.Minute, vars.AuthRateLimitWindow)
	})

	t.Run("Edge: a zero window falls back to 1m", func(t *testing.T) {
		t.Setenv("AUTH_RATE_LIMIT_WINDOW", "0s")
		vars := e.initVariables()
		require.Equal(t, time.Minute, vars.AuthRateLimitWindow)
	})

	t.Run("Edge: a negative window falls back to 1m", func(t *testing.T) {
		t.Setenv("AUTH_RATE_LIMIT_WINDOW", "-5s")
		vars := e.initVariables()
		require.Equal(t, time.Minute, vars.AuthRateLimitWindow)
	})
}

func TestLookupIntEnv(t *testing.T) {
	e := newTestEnv()
	const def = 12

	tests := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{name: "Edge: unset falls back to default", set: false, want: def},
		{name: "Edge: empty value falls back to default", set: true, val: "", want: def},
		{name: "Edge: non-numeric value falls back to default", set: true, val: "twelve", want: def},
		{name: "Edge: zero falls back to default", set: true, val: "0", want: def},
		{name: "Edge: negative falls back to default", set: true, val: "-3", want: def},
		{name: "Happy path: positive integer is parsed", set: true, val: "10", want: 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("TEST_INT", tc.val)
			} else {
				unsetEnv(t, "TEST_INT")
			}
			require.Equal(t, tc.want, e.lookupIntEnv("TEST_INT", def))
		})
	}
}
