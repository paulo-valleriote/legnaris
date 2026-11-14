package openrouter_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatewayRealCompletion sends one prompt through the live OpenRouter
// gateway with the pinned ZDR routing. It self-skips when LLM_API_KEY is unset,
// so CI and unit runs never spend credits or need network access.
func TestGatewayRealCompletion(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY is unset; skipping the real OpenRouter gateway test")
	}

	client, err := openrouter.New(openrouter.Config{
		BaseURL: openrouter.DefaultBaseURL,
		APIKey:  apiKey,
		Model:   openrouter.DefaultModel,
		Timeout: 60 * time.Second,
		Logger:  logger.NewLogger(slog.LevelError),
	})
	require.NoError(t, err)

	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false}`)
	resp, err := client.Generate(context.Background(), llm.Request{
		System:    "Reply with a JSON object that matches the schema.",
		Prompt:    "Invent a short fantasy character name.",
		Schema:    schema,
		MaxTokens: 128,
	})
	require.NoError(t, err)

	var out struct {
		Name string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(resp.Output, &out))
	assert.NotEmpty(t, out.Name)
	assert.Positive(t, resp.Usage.TotalTokens)
}
