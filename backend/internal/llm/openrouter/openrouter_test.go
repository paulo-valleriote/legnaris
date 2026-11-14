package openrouter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAPIKey = "test-key"
	testModel  = "deepseek/deepseek-v4.1-flash"
	testPath   = "/api/v1/chat/completions"
)

var testSchema = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)

func testRequest() llm.Request {
	return llm.Request{System: "sys", Prompt: "concept", Schema: testSchema, MaxTokens: 256}
}

func testConfig(baseURL string) openrouter.Config {
	return openrouter.Config{
		BaseURL: baseURL + "/api/v1",
		APIKey:  testAPIKey,
		Model:   testModel,
		Timeout: 5 * time.Second,
		Logger:  logger.NewLogger(slog.LevelError),
	}
}

func newClient(t *testing.T, handler http.HandlerFunc) *openrouter.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := openrouter.New(testConfig(srv.URL))
	require.NoError(t, err)
	return client
}

func chatBody(t *testing.T, content string) string {
	t.Helper()
	quoted, err := json.Marshal(content)
	require.NoError(t, err)
	return `{"choices":[{"message":{"role":"assistant","content":` + string(quoted) + `}}],` +
		`"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`
}

func TestNewConfigValidation(t *testing.T) {
	valid := testConfig("https://openrouter.example")

	tests := []struct {
		name    string
		mutate  func(c *openrouter.Config)
		wantErr error
	}{
		{name: "Happy path: valid config builds the client", mutate: func(*openrouter.Config) {}, wantErr: nil},
		{name: "Edge: missing API key fails at construction", mutate: func(c *openrouter.Config) { c.APIKey = "" }, wantErr: openrouter.ErrMissingAPIKey},
		{name: "Edge: blank API key fails at construction", mutate: func(c *openrouter.Config) { c.APIKey = "   " }, wantErr: openrouter.ErrMissingAPIKey},
		{name: "Edge: empty model fails at construction", mutate: func(c *openrouter.Config) { c.Model = "" }, wantErr: openrouter.ErrMissingModel},
		{name: "Edge: :free model fails at construction", mutate: func(c *openrouter.Config) { c.Model = "deepseek/deepseek-v4.1-flash:free" }, wantErr: openrouter.ErrFreeModel},
		{name: "Edge: relative base URL fails", mutate: func(c *openrouter.Config) { c.BaseURL = "/api/v1" }, wantErr: openrouter.ErrInvalidBaseURL},
		{name: "Edge: base URL without host fails", mutate: func(c *openrouter.Config) { c.BaseURL = "https://" }, wantErr: openrouter.ErrInvalidBaseURL},
		{name: "Edge: zero timeout fails", mutate: func(c *openrouter.Config) { c.Timeout = 0 }, wantErr: openrouter.ErrInvalidTimeout},
		{name: "Edge: negative timeout fails", mutate: func(c *openrouter.Config) { c.Timeout = -time.Second }, wantErr: openrouter.ErrInvalidTimeout},
		{name: "Edge: nil logger fails", mutate: func(c *openrouter.Config) { c.Logger = nil }, wantErr: openrouter.ErrMissingLogger},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			client, err := openrouter.New(cfg)
			if tc.wantErr == nil {
				require.NoError(t, err)
				assert.NotNil(t, client)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
			assert.Nil(t, client)
		})
	}
}

func TestGenerateRejectsInvalidRequestWithoutCallingGateway(t *testing.T) {
	var calls atomic.Int32
	client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{}`)
	})

	_, err := client.Generate(context.Background(), llm.Request{Schema: testSchema, MaxTokens: 1})

	require.ErrorIs(t, err, llm.ErrPermanent)
	assert.Equal(t, int32(0), calls.Load(), "an invalid request never reaches the gateway")
}

func TestGenerateStatusClassification(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{name: "Error: 429 is transient", status: http.StatusTooManyRequests, body: `{"error":{"message":"rate limited"}}`, wantErr: llm.ErrTransient},
		{name: "Error: 500 is transient", status: http.StatusInternalServerError, body: `{}`, wantErr: llm.ErrTransient},
		{name: "Error: 503 is transient", status: http.StatusServiceUnavailable, body: ``, wantErr: llm.ErrTransient},
		{name: "Error: 400 is permanent", status: http.StatusBadRequest, body: `{"error":{"message":"bad schema"}}`, wantErr: llm.ErrPermanent},
		{name: "Error: 401 is permanent", status: http.StatusUnauthorized, body: `{}`, wantErr: llm.ErrPermanent},
		{name: "Error: 422 is permanent", status: http.StatusUnprocessableEntity, body: `{}`, wantErr: llm.ErrPermanent},
		{
			name:    "Error: 404 with no eligible endpoint maps to provider unavailable",
			status:  http.StatusNotFound,
			body:    `{"error":{"message":"No endpoints found for the requested model."}}`,
			wantErr: llm.ErrProviderUnavailable,
		},
		{
			name:    "Error: 404 with no allowed providers maps to provider unavailable",
			status:  http.StatusNotFound,
			body:    `{"error":{"message":"No allowed providers are available for the selected model."}}`,
			wantErr: llm.ErrProviderUnavailable,
		},
		{name: "Error: 404 without the routing message is permanent", status: http.StatusNotFound, body: `{"error":{"message":"not found"}}`, wantErr: llm.ErrPermanent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})

			_, err := client.Generate(context.Background(), testRequest())

			require.ErrorIs(t, err, tc.wantErr)
			if tc.wantErr == llm.ErrPermanent {
				assert.False(t, errors.Is(err, llm.ErrTransient))
			}
		})
	}
}

func TestGenerateProviderUnavailableIsAlsoTransient(t *testing.T) {
	client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"message":"No endpoints found matching your data policy"}}`)
	})

	_, err := client.Generate(context.Background(), testRequest())

	require.ErrorIs(t, err, llm.ErrProviderUnavailable)
	assert.ErrorIs(t, err, llm.ErrTransient, "the job must be retried, not failed permanently")
}

func TestGenerateTimeoutIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig(srv.URL)
	cfg.Timeout = 50 * time.Millisecond
	client, err := openrouter.New(cfg)
	require.NoError(t, err)

	_, err = client.Generate(context.Background(), testRequest())

	require.ErrorIs(t, err, llm.ErrTransient)
}

func TestGenerateTransportFailureIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	client, err := openrouter.New(testConfig(url))
	require.NoError(t, err)

	_, err = client.Generate(context.Background(), testRequest())

	require.ErrorIs(t, err, llm.ErrTransient)
}

func TestGenerateMalformedResponseIsPermanent(t *testing.T) {
	tests := []struct {
		name string
		body func(t *testing.T) string
	}{
		{name: "Edge: empty body", body: func(*testing.T) string { return `` }},
		{name: "Edge: body is not JSON", body: func(*testing.T) string { return `not json` }},
		{name: "Edge: trailing data after the JSON object", body: func(t *testing.T) string { return chatBody(t, `{}`) + ` trailing` }},
		{name: "Edge: empty choices", body: func(*testing.T) string { return `{"choices":[],"usage":{"total_tokens":0}}` }},
		{name: "Edge: missing choices", body: func(*testing.T) string { return `{"usage":{"total_tokens":0}}` }},
		{name: "Edge: empty message content", body: func(*testing.T) string { return `{"choices":[{"message":{"content":""}}]}` }},
		{name: "Edge: content is not valid JSON", body: func(t *testing.T) string { return chatBody(t, `{"name":`) }},
		{name: "Edge: content is plain text", body: func(t *testing.T) string { return chatBody(t, `hello`) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body(t)
			client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, body)
			})

			_, err := client.Generate(context.Background(), testRequest())

			require.ErrorIs(t, err, llm.ErrPermanent)
			assert.False(t, errors.Is(err, llm.ErrTransient))
		})
	}
}

func TestGenerateOversizedResponseIsPermanent(t *testing.T) {
	padding := strings.Repeat(" ", 1<<20)
	client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[]}`+padding)
	})

	_, err := client.Generate(context.Background(), testRequest())

	require.ErrorIs(t, err, llm.ErrPermanent)
}

func TestGenerateErrorsNeverEchoGatewayBody(t *testing.T) {
	const secret = "PROMPT-ECHO-SECRET-1234"
	statuses := []int{http.StatusBadRequest, http.StatusInternalServerError, http.StatusNotFound}

	for _, status := range statuses {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":{"message":"`+secret+` No endpoints found"}}`)
			})

			_, err := client.Generate(context.Background(), testRequest())

			require.Error(t, err)
			assert.NotContains(t, err.Error(), secret)
		})
	}
}

func TestGenerateHappyPathRequestShape(t *testing.T) {
	var (
		gotPath   string
		gotAuth   string
		gotCTType string
		gotBody   map[string]any
	)
	client := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCTType = r.Header.Get("Content-Type")
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		_, _ = io.WriteString(w, chatBody(t, `{"name":"Aria"}`))
	})

	_, err := client.Generate(context.Background(), testRequest())
	require.NoError(t, err)

	assert.Equal(t, testPath, gotPath)
	assert.Equal(t, "Bearer "+testAPIKey, gotAuth)
	assert.Equal(t, "application/json", gotCTType)
	assert.Equal(t, testModel, gotBody["model"])
	assert.Equal(t, float64(256), gotBody["max_tokens"])

	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
	assert.Equal(t, map[string]any{"role": "system", "content": "sys"}, messages[0])
	assert.Equal(t, map[string]any{"role": "user", "content": "concept"}, messages[1])

	format, ok := gotBody["response_format"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "json_schema", format["type"])
	schema, ok := format["json_schema"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, schema["strict"])
	assert.Equal(t, map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required":   []any{"name"},
	}, schema["schema"])
}

func TestGenerateHappyPathSendsDefaultProviderRouting(t *testing.T) {
	var gotBody map[string]any
	client := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		_, _ = io.WriteString(w, chatBody(t, `{}`))
	})

	_, err := client.Generate(context.Background(), testRequest())
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"require_parameters": true,
		"allow_fallbacks":    true,
	}, gotBody["provider"])
}

func TestGenerateHappyPathParsesOutputAndUsage(t *testing.T) {
	client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, chatBody(t, `{"name":"Aria"}`))
	})

	resp, err := client.Generate(context.Background(), testRequest())

	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Aria"}`, string(resp.Output))
	assert.Equal(t, llm.Usage{PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18}, resp.Usage)
}

func TestGenerateEdgeBaseURLTrailingSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, chatBody(t, `{}`))
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig(srv.URL)
	cfg.BaseURL += "/"
	client, err := openrouter.New(cfg)
	require.NoError(t, err)

	_, err = client.Generate(context.Background(), testRequest())

	require.NoError(t, err)
	assert.Equal(t, testPath, gotPath)
}

func TestGenerateEdgeConcurrentCallsAreSafe(t *testing.T) {
	client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, chatBody(t, `{"name":"Aria"}`))
	})

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			_, err := client.Generate(context.Background(), testRequest())
			errs <- err
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
}
