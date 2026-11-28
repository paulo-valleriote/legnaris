package env

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var generationEnvKeys = []string{
	"GENERATION_ENABLED",
	"GENERATION_DAILY_CAP",
	"GENERATION_CREATE_RATE_LIMIT_ATTEMPTS",
	"GENERATION_CREATE_RATE_LIMIT_WINDOW",
	"GENERATION_READ_RATE_LIMIT_ATTEMPTS",
	"GENERATION_READ_RATE_LIMIT_WINDOW",
}

func unsetGenerationEnv(t *testing.T) {
	t.Helper()
	for _, key := range generationEnvKeys {
		unsetEnv(t, key)
	}
}

func TestInitVariablesGenerationDefaults(t *testing.T) {
	unsetGenerationEnv(t)
	t.Setenv("DATABASE_URI", "mongodb://localhost")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("EMAIL_CODE_SECRET", "secret")

	vars := initTestVariables()

	assert.False(t, vars.GenerationEnabled, "the kill-switch is off by default")
	assert.Equal(t, 200, vars.GenerationDailyCap)
	assert.Equal(t, 5, vars.GenerationCreateRateLimitAttempts)
	assert.Equal(t, time.Minute, vars.GenerationCreateRateLimitWindow)
	assert.Equal(t, 30, vars.GenerationReadRateLimitAttempts)
	assert.Equal(t, time.Minute, vars.GenerationReadRateLimitWindow)
}

func TestInitVariablesGenerationOverrides(t *testing.T) {
	t.Setenv("GENERATION_ENABLED", "true")
	t.Setenv("GENERATION_DAILY_CAP", "50")
	t.Setenv("GENERATION_CREATE_RATE_LIMIT_ATTEMPTS", "3")
	t.Setenv("GENERATION_CREATE_RATE_LIMIT_WINDOW", "2m")
	t.Setenv("GENERATION_READ_RATE_LIMIT_ATTEMPTS", "60")
	t.Setenv("GENERATION_READ_RATE_LIMIT_WINDOW", "30s")

	vars := initTestVariables()

	assert.True(t, vars.GenerationEnabled)
	assert.Equal(t, 50, vars.GenerationDailyCap)
	assert.Equal(t, 3, vars.GenerationCreateRateLimitAttempts)
	assert.Equal(t, 2*time.Minute, vars.GenerationCreateRateLimitWindow)
	assert.Equal(t, 60, vars.GenerationReadRateLimitAttempts)
	assert.Equal(t, 30*time.Second, vars.GenerationReadRateLimitWindow)
}

func TestInitVariablesGenerationInvalidValuesUseDefaults(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "Edge: a non-boolean kill-switch keeps the off default", key: "GENERATION_ENABLED", value: "maybe"},
		{name: "Edge: a non-integer cap keeps the default", key: "GENERATION_DAILY_CAP", value: "many"},
		{name: "Edge: an unparseable window keeps the default", key: "GENERATION_CREATE_RATE_LIMIT_WINDOW", value: "soon"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetGenerationEnv(t)
			t.Setenv(tc.key, tc.value)
			vars := initTestVariables()
			switch tc.key {
			case "GENERATION_ENABLED":
				assert.False(t, vars.GenerationEnabled)
			case "GENERATION_DAILY_CAP":
				assert.Equal(t, 200, vars.GenerationDailyCap)
			default:
				assert.Equal(t, time.Minute, vars.GenerationCreateRateLimitWindow)
			}
		})
	}
}

func TestValidateGenerationDailyCap(t *testing.T) {
	tests := []struct {
		name    string
		cap     int
		wantErr bool
	}{
		{name: "Edge: a zero cap is valid and refuses every job", cap: 0},
		{name: "Happy path: a positive cap is valid", cap: 200},
		{name: "Error: a negative cap fails the boot", cap: -1, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Env{Variables: Variables{GenerationDailyCap: tc.cap, LLMModel: "z-ai/glm-5.3-flash"}}
			err := cfg.Validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
