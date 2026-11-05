package env

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"legnaris/backend/shared/logger"

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

// TestRequiredKeys pins the required-variable set: the removed email
// verification flag never joins it.
func TestRequiredKeys(t *testing.T) {
	t.Run("Edge: only the three secrets are required", func(t *testing.T) {
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, RequiredKeys())
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

	t.Run("ShutdownTimeout defaults to 5s", func(t *testing.T) {
		unsetEnv(t, "SHUTDOWN_TIMEOUT")
		vars := e.initVariables()
		require.Equal(t, 5*time.Second, vars.ShutdownTimeout)
	})

	t.Run("ShutdownTimeout honors a valid override", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "12s")
		vars := e.initVariables()
		require.Equal(t, 12*time.Second, vars.ShutdownTimeout)
	})

	t.Run("ShutdownTimeout falls back to 5s on non-positive or invalid values", func(t *testing.T) {
		for _, raw := range []string{"0s", "-3s", "soon"} {
			t.Setenv("SHUTDOWN_TIMEOUT", raw)
			vars := e.initVariables()
			require.Equalf(t, 5*time.Second, vars.ShutdownTimeout, "raw=%q", raw)
		}
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

func TestInitVariablesScopedRateLimits(t *testing.T) {
	e := newTestEnv()

	tests := []struct {
		name           string
		attemptsKey    string
		windowKey      string
		attempts       func(Variables) int
		window         func(Variables) time.Duration
		defaultAttempt int
		defaultWindow  time.Duration
	}{
		{
			name:           "register ip",
			attemptsKey:    "AUTH_REGISTER_IP_RATE_LIMIT_ATTEMPTS",
			windowKey:      "AUTH_REGISTER_IP_RATE_LIMIT_WINDOW",
			attempts:       func(v Variables) int { return v.RegisterIPRateLimitAttempts },
			window:         func(v Variables) time.Duration { return v.RegisterIPRateLimitWindow },
			defaultAttempt: 5,
			defaultWindow:  time.Hour,
		},
		{
			name:           "register email",
			attemptsKey:    "AUTH_REGISTER_EMAIL_RATE_LIMIT_ATTEMPTS",
			windowKey:      "AUTH_REGISTER_EMAIL_RATE_LIMIT_WINDOW",
			attempts:       func(v Variables) int { return v.RegisterEmailRateLimitAttempts },
			window:         func(v Variables) time.Duration { return v.RegisterEmailRateLimitWindow },
			defaultAttempt: 3,
			defaultWindow:  time.Hour,
		},
		{
			name:           "user",
			attemptsKey:    "USER_RATE_LIMIT_ATTEMPTS",
			windowKey:      "USER_RATE_LIMIT_WINDOW",
			attempts:       func(v Variables) int { return v.UserRateLimitAttempts },
			window:         func(v Variables) time.Duration { return v.UserRateLimitWindow },
			defaultAttempt: 80,
			defaultWindow:  time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Happy path: defaults apply when unset", func(t *testing.T) {
				unsetEnv(t, tc.attemptsKey)
				unsetEnv(t, tc.windowKey)
				vars := e.initVariables()
				require.Equal(t, tc.defaultAttempt, tc.attempts(vars))
				require.Equal(t, tc.defaultWindow, tc.window(vars))
			})

			t.Run("Happy path: valid values override the defaults", func(t *testing.T) {
				t.Setenv(tc.attemptsKey, "5")
				t.Setenv(tc.windowKey, "30s")
				vars := e.initVariables()
				require.Equal(t, 5, tc.attempts(vars))
				require.Equal(t, 30*time.Second, tc.window(vars))
			})

			for _, bad := range []string{"0", "-1", "many"} {
				t.Run("Edge: attempts "+bad+" falls back to the default", func(t *testing.T) {
					t.Setenv(tc.attemptsKey, bad)
					vars := e.initVariables()
					require.Equal(t, tc.defaultAttempt, tc.attempts(vars))
				})
			}

			for _, bad := range []string{"0s", "-5s", "not-a-duration"} {
				t.Run("Edge: window "+bad+" falls back to the default", func(t *testing.T) {
					t.Setenv(tc.windowKey, bad)
					vars := e.initVariables()
					require.Equal(t, tc.defaultWindow, tc.window(vars))
				})
			}
		})
	}
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

// TestTrustedProxyCIDRs covers the optional proxy set: unset and blank select
// the documented default (nil), a configured list is trimmed, and the
// configured-empty value stays distinct from "unset" (PRQ-03, PRQ-18).
func TestTrustedProxyCIDRs(t *testing.T) {
	e := newTestEnv()

	tests := []struct {
		name string
		set  bool
		val  string
		want []string
	}{
		{name: "Edge: unset selects the default (nil)", set: false, want: nil},
		{name: "Edge: an empty value selects the default (nil)", set: true, val: "", want: nil},
		{name: "Happy path: a comma-separated list is trimmed", set: true, val: " 10.0.0.0/8 , 172.16.0.0/12 ", want: []string{"10.0.0.0/8", "172.16.0.0/12"}},
		{name: "Edge: a configured-empty list stays empty and non-nil", set: true, val: ",", want: []string{}},
		{name: "Edge: an invalid value is preserved for the boot-time policy", set: true, val: "not-a-cidr", want: []string{"not-a-cidr"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("TRUSTED_PROXY_CIDRS", tc.val)
			} else {
				unsetEnv(t, "TRUSTED_PROXY_CIDRS")
			}

			got := e.initVariables().TrustedProxyCIDRs
			require.Equal(t, tc.want, got)
			if tc.set && tc.val == "," {
				require.NotNil(t, got, "a configured-empty set must not collapse into the default")
			}
		})
	}

	t.Run("Edge: the required key list is unchanged", func(t *testing.T) {
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, RequiredKeys())
	})
}

// TestInitVariablesEmailDefaults covers the optional email and broker surface:
// an absent value selects the documented default and the required set is
// unchanged, so the API still boots with none of them set (EML-14).
func TestInitVariablesEmailDefaults(t *testing.T) {
	e := newTestEnv()

	for _, key := range []string{
		"EMAIL_PROVIDER", "RESEND_API_KEY", "EMAIL_FROM", "EMAIL_SEND_TIMEOUT",
		"RABBITMQ_URL", "EMAIL_RELAY_POLL_INTERVAL", "EMAIL_RELAY_LEASE",
		"EMAIL_RELAY_BATCH", "EMAIL_RELAY_CONFIRM_TIMEOUT", "EMAIL_RELAY_MAX_ATTEMPTS",
		"EMAIL_OUTBOX_RETENTION", "EMAIL_RELAY_METRICS_ADDR",
	} {
		unsetEnv(t, key)
	}

	vars := e.initVariables()

	t.Run("Happy path: the provider defaults to the development sender", func(t *testing.T) {
		require.Equal(t, "log", vars.EmailProvider)
	})

	t.Run("Edge: APP_ENV defaults to empty, which is not production", func(t *testing.T) {
		unsetEnv(t, "APP_ENV")
		require.Empty(t, e.initVariables().AppEnv)
	})

	t.Run("Happy path: the provider secrets and sender are empty by default", func(t *testing.T) {
		require.Empty(t, vars.ResendAPIKey)
		require.Empty(t, vars.EmailFrom)
		require.Empty(t, vars.RabbitMQURL)
	})

	t.Run("Happy path: the durations default to the documented values", func(t *testing.T) {
		require.Equal(t, 10*time.Second, vars.EmailSendTimeout)
		require.Equal(t, 1*time.Second, vars.EmailRelayPollInterval)
		require.Equal(t, 30*time.Second, vars.EmailRelayLease)
		require.Equal(t, 5*time.Second, vars.EmailRelayConfirmTimeout)
		require.Equal(t, DefaultEmailOutboxRetention, vars.EmailOutboxRetention)
		require.Equal(t, 168*time.Hour, vars.EmailOutboxRetention)
	})

	t.Run("Happy path: the integer and address defaults are the documented values", func(t *testing.T) {
		require.Equal(t, 25, vars.EmailRelayBatch)
		require.Equal(t, 5, vars.EmailRelayMaxAttempts)
		require.Equal(t, ":9100", vars.EmailRelayMetricsAddr)
	})

	t.Run("Edge: the required key list gains only the digest secret", func(t *testing.T) {
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, RequiredKeys())
	})
}

// TestInitVariablesEmailOverrides covers the override and fallback paths for the
// new variables: valid values are parsed and a blank or unusable one keeps the
// default rather than a magic zero (EML-14).
func TestInitVariablesEmailOverrides(t *testing.T) {
	e := newTestEnv()

	t.Run("Happy path: valid values override the defaults", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("EMAIL_PROVIDER", "resend")
		t.Setenv("RESEND_API_KEY", "re_test")
		t.Setenv("EMAIL_FROM", "no-reply@example.com")
		t.Setenv("EMAIL_SEND_TIMEOUT", "3s")
		t.Setenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
		t.Setenv("EMAIL_RELAY_BATCH", "50")
		t.Setenv("EMAIL_RELAY_MAX_ATTEMPTS", "9")
		t.Setenv("EMAIL_OUTBOX_RETENTION", "24h")
		t.Setenv("EMAIL_RELAY_METRICS_ADDR", ":9200")

		vars := e.initVariables()

		require.Equal(t, "resend", vars.EmailProvider)
		require.Equal(t, "production", vars.AppEnv)
		require.Equal(t, "re_test", vars.ResendAPIKey)
		require.Equal(t, "no-reply@example.com", vars.EmailFrom)
		require.Equal(t, 3*time.Second, vars.EmailSendTimeout)
		require.Equal(t, "amqp://guest:guest@rabbitmq:5672/", vars.RabbitMQURL)
		require.Equal(t, 50, vars.EmailRelayBatch)
		require.Equal(t, 9, vars.EmailRelayMaxAttempts)
		require.Equal(t, 24*time.Hour, vars.EmailOutboxRetention)
		require.Equal(t, ":9200", vars.EmailRelayMetricsAddr)
	})

	t.Run("Edge: blank values select the defaults", func(t *testing.T) {
		t.Setenv("EMAIL_PROVIDER", "")
		t.Setenv("RABBITMQ_URL", "")
		t.Setenv("EMAIL_SEND_TIMEOUT", "")

		vars := e.initVariables()

		require.Equal(t, "log", vars.EmailProvider)
		require.Empty(t, vars.RabbitMQURL)
		require.Equal(t, 10*time.Second, vars.EmailSendTimeout)
	})

	t.Run("Edge: an invalid duration falls back to the default", func(t *testing.T) {
		t.Setenv("EMAIL_SEND_TIMEOUT", "not-a-duration")
		require.Equal(t, 10*time.Second, e.initVariables().EmailSendTimeout)
	})

	t.Run("Edge: a non-positive batch falls back to the default", func(t *testing.T) {
		t.Setenv("EMAIL_RELAY_BATCH", "0")
		require.Equal(t, 25, e.initVariables().EmailRelayBatch)
	})

	t.Run("Edge: a non-numeric attempt cap falls back to the default", func(t *testing.T) {
		t.Setenv("EMAIL_RELAY_MAX_ATTEMPTS", "many")
		require.Equal(t, 5, e.initVariables().EmailRelayMaxAttempts)
	})
}

// TestInitVariablesChallengeDefaults covers the optional challenge surface: an
// absent value selects the documented default and the required set gains only
// the digest secret, so the API boots with none of the optional settings set
// (OTP-18).
func TestInitVariablesChallengeDefaults(t *testing.T) {
	e := newTestEnv()

	for _, key := range []string{
		"AUTH_OTP_TTL", "AUTH_OTP_CEILING", "AUTH_OTP_RESEND_COOLDOWN",
		"AUTH_OTP_MAX_ATTEMPTS", "AUTH_OTP_DAILY_SEND_CAP",
		"AUTH_OTP_USER_WINDOW", "AUTH_OTP_USER_LIMIT",
	} {
		unsetEnv(t, key)
	}

	vars := e.initVariables()

	t.Run("Happy path: the durations default to the documented values", func(t *testing.T) {
		require.Equal(t, 10*time.Minute, vars.AuthOTPTTL)
		require.Equal(t, 30*time.Minute, vars.AuthOTPCeiling)
		require.Equal(t, 60*time.Second, vars.AuthOTPResendCooldown)
		require.Equal(t, time.Hour, vars.AuthOTPUserWindow)
	})

	t.Run("Happy path: the integer caps default to the documented values", func(t *testing.T) {
		require.Equal(t, 5, vars.AuthOTPMaxAttempts)
		require.Equal(t, 80, vars.AuthOTPDailySendCap)
		require.Equal(t, 10, vars.AuthOTPUserLimit)
	})

	t.Run("Edge: the required key list includes the digest secret", func(t *testing.T) {
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, RequiredKeys())
	})

	t.Run("Edge: the secret is empty when unset (required)", func(t *testing.T) {
		unsetEnv(t, "EMAIL_CODE_SECRET")
		require.Empty(t, e.initVariables().EmailCodeSecret)
	})
}

// TestInitVariablesChallengeOverrides covers the override and fallback paths:
// valid values are parsed and a blank or unusable one keeps the default rather
// than a magic zero (OTP-18).
func TestInitVariablesChallengeOverrides(t *testing.T) {
	e := newTestEnv()

	t.Run("Happy path: valid values override the defaults", func(t *testing.T) {
		t.Setenv("EMAIL_CODE_SECRET", "0123456789abcdef0123456789abcdef")
		t.Setenv("AUTH_OTP_TTL", "5m")
		t.Setenv("AUTH_OTP_CEILING", "20m")
		t.Setenv("AUTH_OTP_RESEND_COOLDOWN", "30s")
		t.Setenv("AUTH_OTP_MAX_ATTEMPTS", "3")
		t.Setenv("AUTH_OTP_DAILY_SEND_CAP", "40")
		t.Setenv("AUTH_OTP_USER_WINDOW", "2h")
		t.Setenv("AUTH_OTP_USER_LIMIT", "25")

		vars := e.initVariables()

		require.Equal(t, "0123456789abcdef0123456789abcdef", vars.EmailCodeSecret)
		require.Equal(t, 5*time.Minute, vars.AuthOTPTTL)
		require.Equal(t, 20*time.Minute, vars.AuthOTPCeiling)
		require.Equal(t, 30*time.Second, vars.AuthOTPResendCooldown)
		require.Equal(t, 3, vars.AuthOTPMaxAttempts)
		require.Equal(t, 40, vars.AuthOTPDailySendCap)
		require.Equal(t, 2*time.Hour, vars.AuthOTPUserWindow)
		require.Equal(t, 25, vars.AuthOTPUserLimit)
	})

	t.Run("Edge: an invalid duration falls back to the default", func(t *testing.T) {
		t.Setenv("AUTH_OTP_TTL", "not-a-duration")
		require.Equal(t, 10*time.Minute, e.initVariables().AuthOTPTTL)
	})

	t.Run("Edge: a zero duration falls back to the default", func(t *testing.T) {
		t.Setenv("AUTH_OTP_RESEND_COOLDOWN", "0s")
		require.Equal(t, 60*time.Second, e.initVariables().AuthOTPResendCooldown)
	})

	t.Run("Edge: a zero attempt cap falls back to the default", func(t *testing.T) {
		t.Setenv("AUTH_OTP_MAX_ATTEMPTS", "0")
		require.Equal(t, 5, e.initVariables().AuthOTPMaxAttempts)
	})

	t.Run("Edge: a non-numeric daily cap falls back to the default", func(t *testing.T) {
		t.Setenv("AUTH_OTP_DAILY_SEND_CAP", "many")
		require.Equal(t, 80, e.initVariables().AuthOTPDailySendCap)
	})
}

// TestInitVariablesResendCapRatio covers the resend share of the daily cap: a
// fraction in (0, 1] is accepted, anything else falls back to the documented
// 0.30 with a warning (D5).
func TestInitVariablesResendCapRatio(t *testing.T) {
	e := newTestEnv()

	tests := []struct {
		name  string
		value *string
		want  float64
	}{
		{name: "Happy path: an absent variable selects the documented default", value: nil, want: 0.30},
		{name: "Happy path: a fraction inside the range is used", value: strPtr("0.5"), want: 0.5},
		{name: "Edge: the upper bound 1 is accepted", value: strPtr("1"), want: 1},
		{name: "Edge: zero is outside the range and falls back", value: strPtr("0"), want: 0.30},
		{name: "Edge: a negative value falls back", value: strPtr("-0.1"), want: 0.30},
		{name: "Edge: a value above 1 falls back", value: strPtr("1.5"), want: 0.30},
		{name: "Edge: NaN falls back", value: strPtr("NaN"), want: 0.30},
		{name: "Edge: a non-numeric value falls back", value: strPtr("thirty"), want: 0.30},
		{name: "Edge: a blank value falls back", value: strPtr(""), want: 0.30},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetEnv(t, "AUTH_OTP_RESEND_CAP_RATIO")
			if tc.value != nil {
				t.Setenv("AUTH_OTP_RESEND_CAP_RATIO", *tc.value)
			}
			require.Equal(t, tc.want, e.initVariables().AuthOTPResendCapRatio)
		})
	}
}

func strPtr(s string) *string { return &s }

// TestInitVariablesWorkerSettings covers the worker's optional settings: an
// absent, blank or malformed value selects the documented default and the
// required list is unchanged (WKR-14, WKR-16, WKR-19, WKR-20).
func TestInitVariablesWorkerSettings(t *testing.T) {
	e := newTestEnv()

	tests := []struct {
		name         string
		prefetch     string
		maxAttempts  string
		metricsAddr  string
		wantPrefetch int
		wantAttempts int
		wantAddr     string
	}{
		{name: "Happy path: valid values override the defaults", prefetch: "4", maxAttempts: "7", metricsAddr: ":9300", wantPrefetch: 4, wantAttempts: 7, wantAddr: ":9300"},
		{name: "Edge: blank values select the defaults", prefetch: "", maxAttempts: "", metricsAddr: "", wantPrefetch: 10, wantAttempts: 5, wantAddr: ":9101"},
		{name: "Edge: zero falls back to the defaults", prefetch: "0", maxAttempts: "0", wantPrefetch: 10, wantAttempts: 5, wantAddr: ":9101"},
		{name: "Edge: negative falls back to the defaults", prefetch: "-3", maxAttempts: "-1", wantPrefetch: 10, wantAttempts: 5, wantAddr: ":9101"},
		{name: "Edge: non-numeric falls back to the defaults", prefetch: "many", maxAttempts: "lots", wantPrefetch: 10, wantAttempts: 5, wantAddr: ":9101"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EMAIL_WORKER_PREFETCH", tc.prefetch)
			t.Setenv("EMAIL_WORKER_MAX_ATTEMPTS", tc.maxAttempts)
			t.Setenv("EMAIL_WORKER_METRICS_ADDR", tc.metricsAddr)

			vars := e.initVariables()

			require.Equal(t, tc.wantPrefetch, vars.EmailWorkerPrefetch)
			require.Equal(t, tc.wantAttempts, vars.EmailWorkerMaxAttempts)
			require.Equal(t, tc.wantAddr, vars.EmailWorkerMetricsAddr)
		})
	}

	t.Run("Edge: unset variables select the defaults and the required list is unchanged", func(t *testing.T) {
		for _, key := range []string{"EMAIL_WORKER_PREFETCH", "EMAIL_WORKER_MAX_ATTEMPTS", "EMAIL_WORKER_METRICS_ADDR"} {
			unsetEnv(t, key)
		}

		vars := e.initVariables()

		require.Equal(t, 10, vars.EmailWorkerPrefetch)
		require.Equal(t, 5, vars.EmailWorkerMaxAttempts)
		require.Equal(t, ":9101", vars.EmailWorkerMetricsAddr)
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, RequiredKeys())
	})
}

// TestInitVariablesEmailRetryDelays proves the retry tiers are optional: an
// absent, blank, malformed or non-positive value reads as zero, which the
// broker resolves to the tier's default (WKR-14).
func TestInitVariablesEmailRetryDelays(t *testing.T) {
	e := newTestEnv()
	keys := []string{"EMAIL_RETRY_DELAY_FIRST", "EMAIL_RETRY_DELAY_SECOND", "EMAIL_RETRY_DELAY_LATER"}

	tests := []struct {
		name  string
		first string
		later string
		want  [2]time.Duration
	}{
		{name: "Happy path: valid durations override the tiers", first: "10s", later: "1m30s", want: [2]time.Duration{10 * time.Second, 90 * time.Second}},
		{name: "Edge: blank values read as zero", first: "", later: "", want: [2]time.Duration{}},
		{name: "Edge: malformed values read as zero", first: "soon", later: "5", want: [2]time.Duration{}},
		{name: "Edge: non-positive values read as zero", first: "0s", later: "-1m", want: [2]time.Duration{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EMAIL_RETRY_DELAY_FIRST", tc.first)
			t.Setenv("EMAIL_RETRY_DELAY_SECOND", "")
			t.Setenv("EMAIL_RETRY_DELAY_LATER", tc.later)

			vars := e.initVariables()

			require.Equal(t, tc.want[0], vars.EmailRetryDelayFirst)
			require.Zero(t, vars.EmailRetryDelaySecond)
			require.Equal(t, tc.want[1], vars.EmailRetryDelayLater)
		})
	}

	t.Run("Edge: unset variables read as zero and the required list is unchanged", func(t *testing.T) {
		for _, key := range keys {
			unsetEnv(t, key)
		}

		vars := e.initVariables()

		require.Zero(t, vars.EmailRetryDelayFirst)
		require.Zero(t, vars.EmailRetryDelaySecond)
		require.Zero(t, vars.EmailRetryDelayLater)
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, RequiredKeys())
	})
}

func TestDatabasePoolSettings(t *testing.T) {
	tests := []struct {
		name    string
		max     string
		min     string
		wantMax uint64
		wantMin uint64
		wantErr string
	}{
		{name: "Happy path: unset uses safe defaults", wantMax: 20, wantMin: 0},
		{name: "Happy path: valid overrides are applied", max: "50", min: "5", wantMax: 50, wantMin: 5},
		{name: "Edge: min equal to max is accepted", max: "10", min: "10", wantMax: 10, wantMin: 10},
		{name: "Edge: min zero is accepted", max: "10", min: "0", wantMax: 10, wantMin: 0},
		{name: "Edge: max of one is accepted", max: "1", wantMax: 1, wantMin: 0},
		{name: "Error: max above uint32 is rejected", max: "4294967296", wantErr: "DATABASE_MAX_POOL_SIZE"},
		{name: "Error: max zero is rejected", max: "0", wantErr: "DATABASE_MAX_POOL_SIZE"},
		{name: "Error: negative max is rejected", max: "-1", wantErr: "DATABASE_MAX_POOL_SIZE"},
		{name: "Error: non-numeric max is rejected", max: "many", wantErr: "DATABASE_MAX_POOL_SIZE"},
		{name: "Error: negative min is rejected", min: "-2", wantErr: "DATABASE_MIN_POOL_SIZE"},
		{name: "Error: non-numeric min is rejected", min: "x", wantErr: "DATABASE_MIN_POOL_SIZE"},
		{name: "Error: min greater than max is rejected", max: "5", min: "6", wantErr: "must not exceed"},
		{name: "Error: min above the default max is rejected", min: "21", wantErr: "must not exceed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetEnv(t, "DATABASE_MAX_POOL_SIZE")
			unsetEnv(t, "DATABASE_MIN_POOL_SIZE")
			if tc.max != "" {
				t.Setenv("DATABASE_MAX_POOL_SIZE", tc.max)
			}
			if tc.min != "" {
				t.Setenv("DATABASE_MIN_POOL_SIZE", tc.min)
			}

			e := newTestEnv()
			err := e.Validate()

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantMax, e.Variables.DatabaseMaxPoolSize)
			require.Equal(t, tc.wantMin, e.Variables.DatabaseMinPoolSize)
		})
	}
}

func TestRedisURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		set     bool
		want    string
		wantErr string
	}{
		{name: "Edge: unset selects the in-process limiter", want: ""},
		{name: "Edge: empty selects the in-process limiter", set: true, value: "", want: ""},
		{name: "Happy path: redis scheme is accepted", set: true, value: "redis://redis:6379/0", want: "redis://redis:6379/0"},
		{name: "Happy path: rediss scheme is accepted", set: true, value: "rediss://:pw@redis:6380/1", want: "rediss://:pw@redis:6380/1"},
		{name: "Error: an unsupported scheme is rejected", set: true, value: "http://redis:6379", wantErr: "REDIS_URL"},
		{name: "Error: a value without a scheme is rejected", set: true, value: "redis:6379", wantErr: "REDIS_URL"},
		{name: "Error: a scheme without a host is rejected", set: true, value: "redis://", wantErr: "REDIS_URL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetEnv(t, "REDIS_URL")
			if tc.set {
				t.Setenv("REDIS_URL", tc.value)
			}

			e := newTestEnv()
			err := e.Validate()

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, e.Variables.RedisURL)
		})
	}
}

func TestInitVariablesDisposableEmailCheck(t *testing.T) {
	tests := []struct {
		name    string
		appEnv  string
		flag    string
		setFlag bool
		want    bool
	}{
		{name: "Happy path: unset flag defaults to enabled", want: true},
		{name: "Happy path: explicit false disables outside production", flag: "false", setFlag: true, want: false},
		{name: "Happy path: explicit true keeps it enabled", flag: "true", setFlag: true, want: true},
		{name: "Edge: numeric zero disables outside production", flag: "0", setFlag: true, want: false},
		{name: "Edge: empty value falls back to the default", flag: "", setFlag: true, want: true},
		{name: "Edge: unparseable value falls back to the default", flag: "sometimes", setFlag: true, want: true},
		{name: "Edge: false is overridden in production", appEnv: "production", flag: "false", setFlag: true, want: true},
		{name: "Edge: unset flag is enabled in production", appEnv: "production", want: true},
		{name: "Edge: production match is case-sensitive", appEnv: "Production", flag: "false", setFlag: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetEnv(t, "AUTH_DISPOSABLE_EMAIL_CHECK")
			unsetEnv(t, "APP_ENV")
			if tc.setFlag {
				t.Setenv("AUTH_DISPOSABLE_EMAIL_CHECK", tc.flag)
			}
			if tc.appEnv != "" {
				t.Setenv("APP_ENV", tc.appEnv)
			}

			env := newTestEnv()
			vars := env.initVariables()

			require.Equal(t, tc.want, vars.AuthDisposableEmailCheck)
			require.Equal(t, tc.appEnv, vars.AppEnv)
		})
	}
}
