package openrouter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAPIKey = "test-key"
	testModel  = "z-ai/glm-5.3-flash"
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
	return chatCompletionBody(t, content, "", 0)
}

func chatCompletionBody(t *testing.T, content, finish string, reasoningTokens int) string {
	t.Helper()
	quoted, err := json.Marshal(content)
	require.NoError(t, err)
	choice := `{"message":{"role":"assistant","content":` + string(quoted) + `}`
	if finish != "" {
		choice += `,"finish_reason":` + strconv.Quote(finish)
	}
	choice += `}`
	usage := `"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18`
	if reasoningTokens > 0 {
		usage += `,"completion_tokens_details":{"reasoning_tokens":` + strconv.Itoa(reasoningTokens) + `}`
	}
	usage += `}`
	return `{"choices":[` + choice + `],` + usage + `}`
}

func newLoggingClient(t *testing.T, level slog.Level, handler http.HandlerFunc) *openrouter.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := testConfig(srv.URL)
	cfg.Logger = logger.NewLogger(level)
	client, err := openrouter.New(cfg)
	require.NoError(t, err)
	return client
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()
	fn()
	require.NoError(t, writer.Close())
	os.Stdout = original
	return <-done
}

func findLogEntry(t *testing.T, logs, msg string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		if entry["msg"] == msg {
			return entry
		}
	}
	require.FailNow(t, "log entry not found", "msg=%q logs=%s", msg, logs)
	return nil
}

func logString(t *testing.T, entry map[string]any, key string) string {
	t.Helper()
	value, ok := entry[key].(string)
	require.True(t, ok, "attribute %q must be a logged string", key)
	return value
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
		{name: "Edge: :free model fails at construction", mutate: func(c *openrouter.Config) { c.Model = "z-ai/glm-5.3-flash:free" }, wantErr: openrouter.ErrFreeModel},
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

func TestErrFreeModelMessage(t *testing.T) {
	assert.Equal(t, "openrouter: :free model variants are refused", openrouter.ErrFreeModel.Error())
}

func TestNewReasoningEffort(t *testing.T) {
	type testCase struct {
		name    string
		effort  string
		wantErr error
	}

	efforts := openrouter.ReasoningEfforts()
	require.Len(t, efforts, 6)

	tests := make([]testCase, 0, len(efforts)+4)
	tests = append(tests, testCase{name: "Happy path: empty effort keeps the provider default", effort: ""})
	for _, effort := range efforts {
		tests = append(tests, testCase{name: "Happy path: " + effort + " is accepted", effort: effort})
	}
	tests = append(tests,
		testCase{name: "Edge: uppercase is refused", effort: "LOW", wantErr: openrouter.ErrInvalidReasoningEffort},
		testCase{name: "Error: unknown effort is refused", effort: "turbo", wantErr: openrouter.ErrInvalidReasoningEffort},
		testCase{name: "Edge: leading space is refused", effort: " low", wantErr: openrouter.ErrInvalidReasoningEffort},
		testCase{name: "Edge: trailing space is refused", effort: "low ", wantErr: openrouter.ErrInvalidReasoningEffort},
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig("https://openrouter.example")
			cfg.ReasoningEffort = tc.effort

			client, err := openrouter.New(cfg)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, client)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

func TestReasoningEffortsReturnsCopy(t *testing.T) {
	efforts := openrouter.ReasoningEfforts()
	require.NotEmpty(t, efforts)
	first := efforts[0]

	efforts[0] = "mutated"

	assert.Equal(t, first, openrouter.ReasoningEfforts()[0])
}

func TestReasoningEffortsMatchesEnv(t *testing.T) {
	assert.Equal(t, env.ReasoningEfforts(), openrouter.ReasoningEfforts())
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
			if errors.Is(tc.wantErr, llm.ErrPermanent) {
				assert.NotErrorIs(t, err, llm.ErrTransient)
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
			assert.NotErrorIs(t, err, llm.ErrTransient)
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

func TestGenerateUnusableResponseLogsReason(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantErr    error
		wantReason string
		wantFinish string
	}{
		{
			name:       "Edge: empty body is a malformed response",
			body:       "",
			wantErr:    llm.ErrPermanent,
			wantReason: "malformed_response",
		},
		{
			name:       "Edge: body that is not JSON is a malformed response",
			body:       "not json",
			wantErr:    llm.ErrPermanent,
			wantReason: "malformed_response",
		},
		{
			name:       "Edge: missing choices",
			body:       `{"usage":{"total_tokens":0}}`,
			wantErr:    llm.ErrPermanent,
			wantReason: "no_choices",
		},
		{
			name:       "Edge: empty choices",
			body:       `{"choices":[],"usage":{"total_tokens":0}}`,
			wantErr:    llm.ErrPermanent,
			wantReason: "no_choices",
		},
		{
			name:       "Edge: missing content and finish reason",
			body:       `{"choices":[{"message":{}}]}`,
			wantErr:    llm.ErrTransient,
			wantReason: "empty_content",
		},
		{
			name: "Edge: null content with a length finish reason is transient",
			body: `{"choices":[{"message":{"content":null},"finish_reason":"length"}],` +
				`"usage":{"completion_tokens":6000,"completion_tokens_details":{"reasoning_tokens":6000}}}`,
			wantErr:    llm.ErrTransient,
			wantReason: "empty_content",
			wantFinish: "length",
		},
		{
			name:       "Error: non-JSON content",
			body:       chatBody(t, "hello"),
			wantErr:    llm.ErrPermanent,
			wantReason: "invalid_json",
		},
		{
			name:       "Error: truncated non-JSON content is transient",
			body:       chatCompletionBody(t, `{"name":`, "length", 0),
			wantErr:    llm.ErrTransient,
			wantReason: "invalid_json",
			wantFinish: "length",
		},
		{
			name:       "Edge: valid JSON that hit the length limit is retried",
			body:       chatCompletionBody(t, `{}`, "length", 0),
			wantErr:    llm.ErrTransient,
			wantReason: "truncated",
			wantFinish: "length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var generateErr error
			logs := captureStdout(t, func() {
				client := newLoggingClient(t, slog.LevelInfo, func(w http.ResponseWriter, _ *http.Request) {
					_, _ = io.WriteString(w, tc.body)
				})
				_, generateErr = client.Generate(llm.WithGenerationID(context.Background(), "job-1"), testRequest())
			})

			require.ErrorIs(t, generateErr, tc.wantErr)
			if errors.Is(tc.wantErr, llm.ErrPermanent) {
				assert.NotErrorIs(t, generateErr, llm.ErrTransient)
			}

			entry := findLogEntry(t, logs, "llm generation failed")
			assert.Equal(t, tc.wantReason, logString(t, entry, "reason"))
			assert.Equal(t, tc.wantFinish, logString(t, entry, "finish_reason"))
			assert.NotEmpty(t, logString(t, entry, "completion_tokens"))
			assert.NotEmpty(t, logString(t, entry, "reasoning_tokens"))
			assert.Equal(t, "job-1", logString(t, entry, "generation_id"))
			assert.NotContains(t, entry, "error_code")
		})
	}

	t.Run("Edge: reasoning tokens explain a null completion", func(t *testing.T) {
		logs := captureStdout(t, func() {
			client := newLoggingClient(t, slog.LevelInfo, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"choices":[{"message":{"content":null},"finish_reason":"length"}],`+
					`"usage":{"completion_tokens":6000,"completion_tokens_details":{"reasoning_tokens":6000}}}`)
			})
			_, _ = client.Generate(context.Background(), testRequest())
		})

		entry := findLogEntry(t, logs, "llm generation failed")
		assert.Equal(t, "6000", logString(t, entry, "completion_tokens"))
		assert.Equal(t, "6000", logString(t, entry, "reasoning_tokens"))
	})
}

func TestGenerateNon2xxLogsUpstreamFailure(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantMessage string
		wantCode    string
	}{
		{
			name:        "Happy path: message and numeric code are logged",
			status:      http.StatusBadRequest,
			body:        `{"error":{"message":"bad schema","code":400}}`,
			wantMessage: "bad schema",
			wantCode:    "400",
		},
		{
			name:        "Edge: a string code is logged",
			status:      http.StatusUnauthorized,
			body:        `{"error":{"message":"invalid key","code":"invalid_api_key"}}`,
			wantMessage: "invalid key",
			wantCode:    "invalid_api_key",
		},
		{
			name:        "Edge: a null code is omitted",
			status:      http.StatusBadRequest,
			body:        `{"error":{"message":"bad schema","code":null}}`,
			wantMessage: "bad schema",
		},
		{
			name:     "Edge: a missing message omits error_message",
			status:   http.StatusBadRequest,
			body:     `{"error":{"code":400}}`,
			wantCode: "400",
		},
		{
			name:   "Edge: an unparsable body omits both attributes",
			status: http.StatusInternalServerError,
			body:   `<html>bad gateway</html>`,
		},
		{
			name:   "Edge: an empty body omits both attributes",
			status: http.StatusBadGateway,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var generateErr error
			logs := captureStdout(t, func() {
				client := newLoggingClient(t, slog.LevelInfo, func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
				})
				_, generateErr = client.Generate(llm.WithGenerationID(context.Background(), "job-2"), testRequest())
			})

			require.Error(t, generateErr)

			entry := findLogEntry(t, logs, "llm generation failed")
			assert.Equal(t, "job-2", logString(t, entry, "generation_id"))
			assert.Equal(t, strconv.Itoa(tc.status), logString(t, entry, "status"))
			if tc.wantMessage == "" {
				assert.NotContains(t, entry, "error_message")
			} else {
				assert.Equal(t, tc.wantMessage, logString(t, entry, "error_message"))
			}
			if tc.wantCode == "" {
				assert.NotContains(t, entry, "upstream_code")
			} else {
				assert.Equal(t, tc.wantCode, logString(t, entry, "upstream_code"))
			}
			assert.NotContains(t, entry, "error_code")
		})
	}

	t.Run("Edge: a huge message is truncated to 300 bytes on a rune boundary", func(t *testing.T) {
		message := strings.Repeat("a", 299) + strings.Repeat("é", 100)
		logs := captureStdout(t, func() {
			client := newLoggingClient(t, slog.LevelInfo, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"message":`+strconv.Quote(message)+`,"code":400}}`)
			})
			_, _ = client.Generate(context.Background(), testRequest())
		})

		entry := findLogEntry(t, logs, "llm generation failed")
		got := logString(t, entry, "error_message")
		assert.LessOrEqual(t, len(got), 300)
		assert.True(t, utf8.ValidString(got), "a truncated message must not split a rune")
		assert.Equal(t, strings.Repeat("a", 299), got)
	})

	t.Run("Edge: a message of repeated multi-byte runes stops on a boundary", func(t *testing.T) {
		logs := captureStdout(t, func() {
			client := newLoggingClient(t, slog.LevelInfo, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"message":`+strconv.Quote(strings.Repeat("é", 400))+`}}`)
			})
			_, _ = client.Generate(context.Background(), testRequest())
		})

		entry := findLogEntry(t, logs, "llm generation failed")
		got := logString(t, entry, "error_message")
		assert.Len(t, got, 300)
		assert.True(t, utf8.ValidString(got))
		assert.Equal(t, strings.Repeat("é", 150), got)
	})
}

func TestGenerateSuccessLogCarriesFinishReason(t *testing.T) {
	var (
		resp llm.Response
		err  error
		logs string
	)
	logs = captureStdout(t, func() {
		client := newLoggingClient(t, slog.LevelInfo, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, chatCompletionBody(t, `{"name":"Aria"}`, "stop", 42))
		})
		resp, err = client.Generate(llm.WithGenerationID(context.Background(), "job-3"), testRequest())
	})

	require.NoError(t, err)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 42, resp.Usage.ReasoningTokens)

	entry := findLogEntry(t, logs, "llm generation completed")
	assert.Equal(t, "stop", logString(t, entry, "finish_reason"))
	assert.Equal(t, "42", logString(t, entry, "reasoning_tokens"))
	assert.Equal(t, "job-3", logString(t, entry, "generation_id"))
}

func TestGenerateLogsReasoningEffort(t *testing.T) {
	tests := []struct {
		name       string
		effort     string
		status     int
		body       string
		wantMsg    string
		wantEffort string
	}{
		{
			name:       "Happy path: a configured effort is logged on completion",
			effort:     "low",
			status:     http.StatusOK,
			body:       chatBody(t, `{"name":"Aria"}`),
			wantMsg:    "llm generation completed",
			wantEffort: "low",
		},
		{
			name:       "Edge: an empty effort is logged as default on completion",
			status:     http.StatusOK,
			body:       chatBody(t, `{"name":"Aria"}`),
			wantMsg:    "llm generation completed",
			wantEffort: "default",
		},
		{
			name:       "Happy path: a configured effort is logged on failure",
			effort:     "low",
			status:     http.StatusBadRequest,
			body:       `{"error":{"message":"bad schema"}}`,
			wantMsg:    "llm generation failed",
			wantEffort: "low",
		},
		{
			name:       "Edge: an empty effort is logged as default on failure",
			status:     http.StatusBadRequest,
			body:       `{"error":{"message":"bad schema"}}`,
			wantMsg:    "llm generation failed",
			wantEffort: "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var generateErr error
			logs := captureStdout(t, func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
				}))
				t.Cleanup(srv.Close)

				cfg := testConfig(srv.URL)
				cfg.ReasoningEffort = tc.effort
				cfg.Logger = logger.NewLogger(slog.LevelInfo)
				client, err := openrouter.New(cfg)
				require.NoError(t, err)

				_, generateErr = client.Generate(context.Background(), testRequest())
			})

			if tc.status == http.StatusOK {
				require.NoError(t, generateErr)
			} else {
				require.Error(t, generateErr)
			}
			entry := findLogEntry(t, logs, tc.wantMsg)
			assert.Equal(t, tc.wantEffort, logString(t, entry, "reasoning_effort"))
		})
	}
}

func TestGenerateLogsNeverCarryPromptOrContent(t *testing.T) {
	const (
		secretSystem  = "SECRET-SYSTEM-XYZ"
		secretPrompt  = "SECRET-PROMPT-XYZ"
		secretContent = "SECRET-CONTENT-XYZ"
	)
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "Happy path: a completed generation logs no content", status: http.StatusOK, body: chatBody(t, `{"name":"`+secretContent+`"}`)},
		{name: "Error: an invalid JSON output logs no content", status: http.StatusOK, body: chatBody(t, secretContent)},
		{name: "Error: an upstream failure logs no request text", status: http.StatusBadRequest, body: `{"error":{"message":"bad schema","code":400}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureStdout(t, func() {
				client := newLoggingClient(t, slog.LevelInfo, func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
				})
				_, _ = client.Generate(context.Background(), llm.Request{
					System: secretSystem, Prompt: secretPrompt, Schema: testSchema, MaxTokens: 256,
				})
			})

			assert.NotContains(t, logs, secretSystem)
			assert.NotContains(t, logs, secretPrompt)
			assert.NotContains(t, logs, secretContent)
			assert.NotContains(t, logs, testAPIKey)
		})
	}
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
	assert.InDelta(t, float64(256), gotBody["max_tokens"], 1e-9)

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

func TestGenerateRequestReasoningEffort(t *testing.T) {
	tests := []struct {
		name   string
		effort string
	}{
		{name: "Happy path: a configured effort is sent", effort: "low"},
		{name: "Edge: an empty effort omits the reasoning object", effort: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
				_, _ = io.WriteString(w, chatBody(t, `{}`))
			}))
			t.Cleanup(srv.Close)

			cfg := testConfig(srv.URL)
			cfg.ReasoningEffort = tc.effort
			client, err := openrouter.New(cfg)
			require.NoError(t, err)

			_, err = client.Generate(context.Background(), testRequest())
			require.NoError(t, err)

			if tc.effort == "" {
				assert.NotContains(t, gotBody, "reasoning")
				return
			}
			assert.Equal(t, map[string]any{"effort": tc.effort}, gotBody["reasoning"])
		})
	}
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

func TestGenerateUsesResponseModel(t *testing.T) {
	t.Run("Happy path: the gateway model is returned", func(t *testing.T) {
		client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"model":"actual/fallback-model",`+chatBody(t, `{"name":"Aria"}`)[1:])
		})
		resp, err := client.Generate(context.Background(), testRequest())
		require.NoError(t, err)
		assert.Equal(t, "actual/fallback-model", resp.Model)
	})

	t.Run("Edge: a missing gateway model falls back to the configured one", func(t *testing.T) {
		client := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, chatBody(t, `{"name":"Aria"}`))
		})
		resp, err := client.Generate(context.Background(), testRequest())
		require.NoError(t, err)
		assert.Equal(t, testModel, resp.Model)
	})
}

func TestDefaultModel(t *testing.T) {
	assert.Equal(t, "z-ai/glm-5.3-flash", openrouter.DefaultModel)
}
