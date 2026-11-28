package llm_test

import (
	"context"
	"encoding/json"
	"testing"

	"legnaris/backend/internal/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestValidate(t *testing.T) {
	validSchema := json.RawMessage(`{"type":"object"}`)

	tests := []struct {
		name    string
		req     llm.Request
		wantErr bool
	}{
		{
			name:    "Happy path: prompt, schema and max tokens present",
			req:     llm.Request{System: "sys", Prompt: "concept", Schema: validSchema, MaxTokens: 512},
			wantErr: false,
		},
		{
			name:    "Happy path: empty system prompt is allowed",
			req:     llm.Request{Prompt: "concept", Schema: validSchema, MaxTokens: 1},
			wantErr: false,
		},
		{
			name:    "Edge: empty prompt is rejected",
			req:     llm.Request{Schema: validSchema, MaxTokens: 512},
			wantErr: true,
		},
		{
			name:    "Edge: nil schema is rejected",
			req:     llm.Request{Prompt: "concept", MaxTokens: 512},
			wantErr: true,
		},
		{
			name:    "Edge: schema that is not JSON is rejected",
			req:     llm.Request{Prompt: "concept", Schema: json.RawMessage(`{`), MaxTokens: 512},
			wantErr: true,
		},
		{
			name:    "Edge: zero max tokens is rejected",
			req:     llm.Request{Prompt: "concept", Schema: validSchema, MaxTokens: 0},
			wantErr: true,
		},
		{
			name:    "Edge: negative max tokens is rejected",
			req:     llm.Request{Prompt: "concept", Schema: validSchema, MaxTokens: -1},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorIs(t, err, llm.ErrPermanent, "invalid requests are permanent: %v", err)
			assert.NotErrorIs(t, err, llm.ErrTransient)
		})
	}
}

func TestProviderUnavailableIsTransient(t *testing.T) {
	require.ErrorIs(t, llm.ErrProviderUnavailable, llm.ErrTransient, "no eligible provider must be retried")
	assert.NotErrorIs(t, llm.ErrProviderUnavailable, llm.ErrPermanent)
}

func TestGenerationID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "Happy path: an id round-trips through the context", id: "job-1", want: "job-1"},
		{name: "Edge: an empty id is not stored", id: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := llm.WithGenerationID(context.Background(), tc.id)
			assert.Equal(t, tc.want, llm.GenerationID(ctx))
		})
	}

	t.Run("Edge: a context without an id reads empty", func(t *testing.T) {
		assert.Empty(t, llm.GenerationID(context.Background()))
	})
}

func TestResponseCarriesModel(t *testing.T) {
	response := llm.Response{Model: "deepseek/deepseek-v4.1-flash"}
	assert.Equal(t, "deepseek/deepseek-v4.1-flash", response.Model)
}

func TestUsageCost(t *testing.T) {
	known := 0.001234
	zero := 0.0
	negative := -0.01

	tests := []struct {
		name      string
		usage     llm.Usage
		want      float64
		wantKnown bool
	}{
		{name: "Happy path: a provider-reported cost is known", usage: llm.Usage{CostUSD: &known}, want: 0.001234, wantKnown: true},
		{name: "Edge: a zero cost is a known zero", usage: llm.Usage{CostUSD: &zero}, want: 0, wantKnown: true},
		{name: "Edge: a missing cost is unknown", usage: llm.Usage{}, want: 0, wantKnown: false},
		{name: "Edge: a negative cost is unknown", usage: llm.Usage{CostUSD: &negative}, want: 0, wantKnown: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cost, known := tc.usage.Cost()
			assert.Equal(t, tc.wantKnown, known)
			assert.InDelta(t, tc.want, cost, 1e-12)
		})
	}
}
