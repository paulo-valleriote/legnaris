package llm_test

import (
	"encoding/json"
	"errors"
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
			assert.True(t, errors.Is(err, llm.ErrPermanent), "invalid requests are permanent: %v", err)
			assert.False(t, errors.Is(err, llm.ErrTransient))
		})
	}
}

func TestProviderUnavailableIsTransient(t *testing.T) {
	assert.True(t, errors.Is(llm.ErrProviderUnavailable, llm.ErrTransient), "no eligible provider must be retried")
	assert.False(t, errors.Is(llm.ErrProviderUnavailable, llm.ErrPermanent))
}
