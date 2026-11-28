package env

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var llmEnvKeys = []string{"LLM_PROVIDER", "LLM_BASE_URL", "LLM_API_KEY", "LLM_MODEL", "LLM_TIMEOUT", "LLM_REASONING_EFFORT", "LLM_BREAKER_THRESHOLD", "LLM_BREAKER_PROBE_INTERVAL"}

func unsetLLMEnv(t *testing.T) {
	t.Helper()
	for _, key := range llmEnvKeys {
		unsetEnv(t, key)
	}
}

func TestInitVariablesLLMDefaults(t *testing.T) {
	unsetLLMEnv(t)
	t.Setenv("DATABASE_URI", "mongodb://localhost")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("EMAIL_CODE_SECRET", "secret")

	vars := initTestVariables()

	assert.Equal(t, "openrouter", vars.LLMProvider)
	assert.Equal(t, "https://openrouter.ai/api/v1", vars.LLMBaseURL)
	assert.Empty(t, vars.LLMAPIKey)
	assert.Equal(t, "deepseek/deepseek-v4.1-flash", vars.LLMModel)
	assert.Empty(t, vars.LLMReasoningEffort)
	assert.Equal(t, 120*time.Second, vars.LLMTimeout)
	assert.Equal(t, 5, vars.LLMBreakerThreshold)
	assert.Equal(t, 60*time.Second, vars.LLMBreakerProbeInterval)
}

func TestInitVariablesLLMOverrides(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openrouter")
	t.Setenv("LLM_BASE_URL", "https://gateway.example/api/v1")
	t.Setenv("LLM_API_KEY", "key-value")
	t.Setenv("LLM_MODEL", "deepseek/deepseek-v4.1-flash-1017")
	t.Setenv("LLM_REASONING_EFFORT", "low")
	t.Setenv("LLM_TIMEOUT", "15s")
	t.Setenv("LLM_BREAKER_THRESHOLD", "3")
	t.Setenv("LLM_BREAKER_PROBE_INTERVAL", "30s")

	vars := initTestVariables()

	assert.Equal(t, "https://gateway.example/api/v1", vars.LLMBaseURL)
	assert.Equal(t, "key-value", vars.LLMAPIKey)
	assert.Equal(t, "deepseek/deepseek-v4.1-flash-1017", vars.LLMModel)
	assert.Equal(t, "low", vars.LLMReasoningEffort)
	assert.Equal(t, 15*time.Second, vars.LLMTimeout)
	assert.Equal(t, 3, vars.LLMBreakerThreshold)
	assert.Equal(t, 30*time.Second, vars.LLMBreakerProbeInterval)
}

func TestInitVariablesLLMInvalidTimeoutUsesDefault(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "Edge: unparseable duration falls back", value: "soon"},
		{name: "Edge: zero duration falls back", value: "0s"},
		{name: "Edge: negative duration falls back", value: "-5s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LLM_TIMEOUT", tc.value)
			assert.Equal(t, 120*time.Second, initTestVariables().LLMTimeout)
		})
	}
}

func TestInitVariablesLLMBreakerInvalidValuesUseDefaults(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "Edge: a non-integer threshold keeps the default", key: "LLM_BREAKER_THRESHOLD", value: "many"},
		{name: "Edge: a zero threshold keeps the default", key: "LLM_BREAKER_THRESHOLD", value: "0"},
		{name: "Edge: a negative threshold keeps the default", key: "LLM_BREAKER_THRESHOLD", value: "-1"},
		{name: "Edge: an unparseable interval keeps the default", key: "LLM_BREAKER_PROBE_INTERVAL", value: "soon"},
		{name: "Edge: a zero interval keeps the default", key: "LLM_BREAKER_PROBE_INTERVAL", value: "0s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			vars := initTestVariables()
			assert.Equal(t, 5, vars.LLMBreakerThreshold)
			assert.Equal(t, 60*time.Second, vars.LLMBreakerProbeInterval)
		})
	}
}

func TestEnvValidateLLMBreaker(t *testing.T) {
	valid := func() Env {
		return Env{Variables: Variables{
			LLMModel:                "deepseek/deepseek-v4.1-flash",
			LLMBreakerThreshold:     5,
			LLMBreakerProbeInterval: time.Minute,
		}}
	}

	tests := []struct {
		name    string
		change  func(*Variables)
		wantErr string
	}{
		{
			name:   "Happy path: the defaults pass",
			change: func(*Variables) {},
		},
		{
			name:    "Error: a zero threshold fails the boot",
			change:  func(v *Variables) { v.LLMBreakerThreshold = 0 },
			wantErr: "LLM_BREAKER_THRESHOLD must be positive",
		},
		{
			name:    "Error: a negative threshold fails the boot",
			change:  func(v *Variables) { v.LLMBreakerThreshold = -3 },
			wantErr: "LLM_BREAKER_THRESHOLD must be positive",
		},
		{
			name:    "Error: a zero probe interval fails the boot",
			change:  func(v *Variables) { v.LLMBreakerProbeInterval = 0 },
			wantErr: "LLM_BREAKER_PROBE_INTERVAL must be positive",
		},
		{
			name:    "Error: a negative probe interval fails the boot",
			change:  func(v *Variables) { v.LLMBreakerProbeInterval = -time.Second },
			wantErr: "LLM_BREAKER_PROBE_INTERVAL must be positive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.change(&cfg.Variables)
			err := cfg.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestValidateLLMModel(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{name: "Happy path: pinned paid model passes", model: "deepseek/deepseek-v4.1-flash", wantErr: false},
		{name: "Happy path: dated paid variant passes", model: "deepseek/deepseek-v4.1-flash-1017", wantErr: false},
		{name: "Edge: :free suffix is refused", model: "deepseek/deepseek-v4.1-flash:free", wantErr: true},
		{name: "Edge: :free suffix on another vendor is refused", model: "meta/llama:free", wantErr: true},
		{name: "Edge: free word without the suffix passes", model: "free/model", wantErr: false},
		{name: "Edge: empty model passes the boot check and fails at the adapter", model: "", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLLMModel(tc.model)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestEnvValidateRejectsFreeLLMModel(t *testing.T) {
	e := newTestEnv()
	e.Variables.LLMModel = "deepseek/deepseek-v4.1-flash:free"

	err := e.Validate()
	require.Error(t, err, "boot must fail when LLM_MODEL is a :free variant")
	assert.Contains(t, err.Error(), "LLM_MODEL must not be a :free model variant")
	assert.NotContains(t, err.Error(), "ADR")

	e.Variables.LLMModel = "deepseek/deepseek-v4.1-flash"
	require.NoError(t, e.Validate())
}

func TestValidateLLMReasoningEffort(t *testing.T) {
	const wantMessage = "LLM_REASONING_EFFORT must be one of none, minimal, low, medium, high, xhigh"

	tests := []struct {
		name    string
		effort  string
		wantErr bool
	}{
		{name: "Happy path: empty keeps the provider default", effort: "", wantErr: false},
		{name: "Happy path: none is accepted", effort: "none", wantErr: false},
		{name: "Happy path: minimal is accepted", effort: "minimal", wantErr: false},
		{name: "Happy path: low is accepted", effort: "low", wantErr: false},
		{name: "Happy path: medium is accepted", effort: "medium", wantErr: false},
		{name: "Happy path: high is accepted", effort: "high", wantErr: false},
		{name: "Happy path: xhigh is accepted", effort: "xhigh", wantErr: false},
		{name: "Edge: uppercase is refused", effort: "LOW", wantErr: true},
		{name: "Edge: unknown effort is refused", effort: "turbo", wantErr: true},
		{name: "Edge: leading space is refused", effort: " low", wantErr: true},
		{name: "Edge: trailing space is refused", effort: "low ", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLLMReasoningEffort(tc.effort)
			if tc.wantErr {
				require.EqualError(t, err, wantMessage)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestEnvValidateRejectsInvalidReasoningEffort(t *testing.T) {
	e := newTestEnv()
	e.Variables.LLMReasoningEffort = "turbo"

	require.EqualError(t, e.Validate(), "LLM_REASONING_EFFORT must be one of none, minimal, low, medium, high, xhigh")

	e.Variables.LLMReasoningEffort = "low"
	require.NoError(t, e.Validate())
}

func TestReasoningEffortsReturnsCopy(t *testing.T) {
	efforts := ReasoningEfforts()
	require.NotEmpty(t, efforts)
	first := efforts[0]

	efforts[0] = "mutated"

	assert.Equal(t, first, ReasoningEfforts()[0])
}

func initTestVariables() Variables {
	e := newTestEnv()
	return e.initVariables()
}
