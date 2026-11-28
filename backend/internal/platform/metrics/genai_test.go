package metrics

import (
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llm"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
)

const (
	testGenAIModel   = "deepseek/deepseek-v4.1-flash"
	testPrompt       = "v1+0123456789ab"
	testGenAILatency = 1500 * time.Millisecond
)

func costPtr(cost float64) *float64 {
	return &cost
}

func TestGenerationAttemptMetrics(t *testing.T) {
	t.Run("Happy path: latency, tokens and the provider-reported cost are exported", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationAttempt(testGenAIModel, testPrompt, testGenAILatency, llm.Usage{
			PromptTokens:     11,
			CompletionTokens: 7,
			TotalTokens:      18,
			CostUSD:          costPtr(0.001234),
		}, generation.AttemptResultSuccess)

		body := scrapeMetrics(t, e)

		for _, want := range []string{
			`legnaris_gen_ai_client_operation_duration_seconds_count{gen_ai_response_model="deepseek/deepseek-v4.1-flash",prompt_version="v1+0123456789ab",result="success"} 1`,
			`legnaris_gen_ai_client_token_usage_count{gen_ai_response_model="deepseek/deepseek-v4.1-flash",gen_ai_token_type="input",prompt_version="v1+0123456789ab"} 1`,
			`legnaris_gen_ai_client_token_usage_sum{gen_ai_response_model="deepseek/deepseek-v4.1-flash",gen_ai_token_type="input",prompt_version="v1+0123456789ab"} 11`,
			`legnaris_gen_ai_client_token_usage_sum{gen_ai_response_model="deepseek/deepseek-v4.1-flash",gen_ai_token_type="output",prompt_version="v1+0123456789ab"} 7`,
			`legnaris_gen_ai_cost_usd_total{gen_ai_response_model="deepseek/deepseek-v4.1-flash",prompt_version="v1+0123456789ab"} 0.001234`,
		} {
			assert.Containsf(t, body, want, "expected %q in /metrics output, got:\n%s", want, body)
		}
		assert.NotContains(t, body, "legnaris_gen_ai_cost_unknown_total")
	})

	t.Run("Edge: a response without cost counts as unknown, never as zero", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationAttempt(testGenAIModel, testPrompt, testGenAILatency, llm.Usage{
			PromptTokens:     11,
			CompletionTokens: 7,
		}, generation.AttemptResultSuccess)

		body := scrapeMetrics(t, e)

		assert.Contains(t, body, `legnaris_gen_ai_cost_unknown_total{gen_ai_response_model="deepseek/deepseek-v4.1-flash",prompt_version="v1+0123456789ab"} 1`)
		assert.NotContains(t, body, "legnaris_gen_ai_cost_usd_total")
	})

	t.Run("Edge: a negative provider cost counts as unknown", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationAttempt(testGenAIModel, testPrompt, testGenAILatency, llm.Usage{
			CostUSD: costPtr(-0.01),
		}, generation.AttemptResultSuccess)

		body := scrapeMetrics(t, e)

		assert.Contains(t, body, `legnaris_gen_ai_cost_unknown_total{gen_ai_response_model="deepseek/deepseek-v4.1-flash",prompt_version="v1+0123456789ab"} 1`)
		assert.NotContains(t, body, "legnaris_gen_ai_cost_usd_total")
	})

	t.Run("Edge: a failed call exports latency but no tokens or cost", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationAttempt("", testPrompt, testGenAILatency, llm.Usage{}, generation.AttemptResultError)

		body := scrapeMetrics(t, e)

		assert.Contains(t, body, `legnaris_gen_ai_client_operation_duration_seconds_count{gen_ai_response_model="unknown",prompt_version="v1+0123456789ab",result="error"} 1`)
		assert.NotContains(t, body, "legnaris_gen_ai_client_token_usage")
		assert.NotContains(t, body, "legnaris_gen_ai_cost_usd_total")
	})

	t.Run("Edge: an unknown result is normalized to error", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationAttempt(testGenAIModel, testPrompt, testGenAILatency, llm.Usage{}, "surprise")

		body := scrapeMetrics(t, e)

		assert.Contains(t, body, `legnaris_gen_ai_client_operation_duration_seconds_count{gen_ai_response_model="deepseek/deepseek-v4.1-flash",prompt_version="v1+0123456789ab",result="error"} 1`)
	})
}

func TestGenerationOutputMetrics(t *testing.T) {
	t.Run("Happy path: invalid outputs and schema retries are exported by model and prompt version", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationInvalidOutput(testGenAIModel, testPrompt)
		m.GenerationInvalidOutput(testGenAIModel, testPrompt)
		m.GenerationSchemaRetry(testGenAIModel, testPrompt)

		body := scrapeMetrics(t, e)

		assert.Contains(t, body, `legnaris_gen_ai_invalid_output_total{gen_ai_response_model="deepseek/deepseek-v4.1-flash",prompt_version="v1+0123456789ab"} 2`)
		assert.Contains(t, body, `legnaris_gen_ai_schema_retry_total{gen_ai_response_model="deepseek/deepseek-v4.1-flash",prompt_version="v1+0123456789ab"} 1`)
	})

	t.Run("Edge: a missing model is reported as unknown", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationInvalidOutput("", testPrompt)
		m.GenerationSchemaRetry("", testPrompt)

		body := scrapeMetrics(t, e)

		assert.Contains(t, body, `legnaris_gen_ai_invalid_output_total{gen_ai_response_model="unknown",prompt_version="v1+0123456789ab"} 1`)
		assert.Contains(t, body, `legnaris_gen_ai_schema_retry_total{gen_ai_response_model="unknown",prompt_version="v1+0123456789ab"} 1`)
	})
}

func TestGenerationBreakerMetrics(t *testing.T) {
	t.Run("Happy path: the breaker gauge follows its state and openings are counted", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		assert.Contains(t, scrapeMetrics(t, e), "legnaris_gen_ai_breaker_open 0")

		m.GenerationBreakerState(true)
		m.GenerationBreakerOpened()

		body := scrapeMetrics(t, e)
		assert.Contains(t, body, "legnaris_gen_ai_breaker_open 1")
		assert.Contains(t, body, "legnaris_gen_ai_breaker_opened_total 1")
	})

	t.Run("Edge: closing the breaker zeroes the gauge without touching the opening count", func(t *testing.T) {
		e := echo.New()
		m := New()
		m.Register(e)
		m.GenerationBreakerState(true)
		m.GenerationBreakerOpened()
		m.GenerationBreakerState(false)

		body := scrapeMetrics(t, e)
		assert.Contains(t, body, "legnaris_gen_ai_breaker_open 0")
		assert.Contains(t, body, "legnaris_gen_ai_breaker_opened_total 1")
	})
}
