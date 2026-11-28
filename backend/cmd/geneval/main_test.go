package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/llm/openrouter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResult struct {
	resp llm.Response
	err  error
}

type fakeClient struct {
	results  []fakeResult
	requests []llm.Request
}

func (f *fakeClient) Generate(_ context.Context, req llm.Request) (llm.Response, error) {
	f.requests = append(f.requests, req)
	if len(f.results) == 0 {
		return llm.Response{}, errors.New("unexpected provider call")
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result.resp, result.err
}

func getenv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func runWithClient(t *testing.T, values map[string]string, client llm.Client, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, getenv(values), &stdout, &stderr, func(openrouter.Config) (llm.Client, error) {
		return client, nil
	})
	return code, stdout.String(), stderr.String()
}

func baseEnv(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"LLM_API_KEY":       "test-key",
		"EVAL_COST_LEDGER":  filepath.Join(t.TempDir(), "cost.jsonl"),
		"EVAL_MAX_COST_USD": "1.00",
	}
}

func sampleObject(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	object := make(map[string]any, len(props))
	for name, raw := range props {
		prop, ok := raw.(map[string]any)
		require.True(t, ok)
		object[name] = sampleValue(t, prop)
	}
	return object
}

func sampleValue(t *testing.T, prop map[string]any) any {
	t.Helper()
	switch prop["type"] {
	case "object":
		return sampleObject(t, prop)
	case "array":
		return []any{}
	case "string":
		if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
			return enum[0]
		}
		return "Sample"
	case "number":
		if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
			return enum[0]
		}
		if minimum, ok := prop["minimum"].(float64); ok {
			return minimum
		}
		return float64(0)
	case "boolean":
		return false
	default:
		require.FailNow(t, "unsupported schema type", "%v", prop["type"])
		return nil
	}
}

func validDraftObject(t *testing.T) map[string]any {
	t.Helper()
	tmpl, dag, err := loadDefaultTemplate()
	require.NoError(t, err)
	raw, err := generation.OutputSchema(tmpl, evalKind)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))

	object := sampleObject(t, schema)
	identity, ok := object["identity"].(map[string]any)
	require.True(t, ok)
	identity["name"] = "Probe"
	identity["class"] = "Fighter"
	identity["subclass"] = ""
	identity["species"] = "Human"
	identity["background"] = "Soldier"
	identity["level"] = float64(1)

	abilities, ok := object["abilities"].(map[string]any)
	require.True(t, ok)
	for _, ability := range []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"} {
		group, ok := abilities[ability].(map[string]any)
		require.True(t, ok)
		group["score"] = float64(10)
	}

	saves, ok := object["saves"].(map[string]any)
	require.True(t, ok)
	for _, ability := range []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"} {
		group, ok := saves[ability].(map[string]any)
		require.True(t, ok)
		group["prof"] = float64(0)
	}
	saves["strength"].(map[string]any)["prof"] = float64(1)
	saves["constitution"].(map[string]any)["prof"] = float64(1)

	rawObject, err := json.Marshal(object)
	require.NoError(t, err)
	parsed, err := generation.ParseOutput(rawObject)
	require.NoError(t, err)
	expanded, err := generation.Expand(tmpl, evalKind, parsed)
	require.NoError(t, err)
	_, err = generation.Assemble(tmpl, dag, evalKind, expanded)
	require.NoError(t, err)
	return object
}

func draftResponse(t *testing.T, cost *float64) llm.Response {
	t.Helper()
	object := validDraftObject(t)
	raw, err := json.Marshal(object)
	require.NoError(t, err)
	return llm.Response{Output: raw, Model: "probe/model", Usage: llm.Usage{CostUSD: cost}}
}

func ptrFloat(value float64) *float64 {
	return &value
}

func readLedger(t *testing.T, path string) []costEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	entries := make([]costEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry costEntry
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		entries = append(entries, entry)
	}
	return entries
}

func TestRunInfo(t *testing.T) {
	t.Run("Happy path: info prints the prompt version without a provider call", func(t *testing.T) {
		called := false
		var stdout, stderr bytes.Buffer
		code := run([]string{"-info"}, getenv(map[string]string{}), &stdout, &stderr, func(openrouter.Config) (llm.Client, error) {
			called = true
			return nil, errors.New("should not be called")
		})

		require.Zero(t, code, stderr.String())
		require.False(t, called)
		var info evalInfo
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &info))
		assert.Equal(t, "fifth_character_v1", info.TemplateID)
		assert.Equal(t, "character", info.Kind)
		assert.Equal(t, openrouter.DefaultModel, info.Model)
		assert.True(t, strings.HasPrefix(info.PromptVersion, "v1+"))
		assert.Len(t, info.PromptVersion, len("v1+")+12)
	})
}

func TestRunDraft(t *testing.T) {
	t.Run("Happy path: a valid output is printed and the cost is recorded", func(t *testing.T) {
		env := baseEnv(t)
		client := &fakeClient{results: []fakeResult{{resp: draftResponse(t, ptrFloat(0.0012))}}}
		code, stdout, stderr := runWithClient(t, env, client, "a veteran city guard")

		require.Zero(t, code, stderr)
		var values map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &values))
		identity, ok := values["identity"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Probe", identity["name"])

		entries := readLedger(t, env["EVAL_COST_LEDGER"])
		require.Len(t, entries, 1)
		assert.True(t, entries[0].Known)
		require.NotNil(t, entries[0].CostUSD)
		assert.InDelta(t, 0.0012, *entries[0].CostUSD, 1e-9)
		assert.Equal(t, "probe/model", entries[0].Model)
		assert.NotEmpty(t, entries[0].PromptVersion)
		require.Len(t, client.requests, 1)
		require.NotNil(t, client.requests[0].Temperature)
		assert.InDelta(t, 0, *client.requests[0].Temperature, 1e-9)
	})

	t.Run("Edge: an invalid output is retried before the valid one", func(t *testing.T) {
		env := baseEnv(t)
		client := &fakeClient{results: []fakeResult{
			{resp: llm.Response{Output: []byte(`{"identity":{"name":"Only"}}`), Model: "probe/model", Usage: llm.Usage{CostUSD: ptrFloat(0.001)}}},
			{resp: draftResponse(t, ptrFloat(0.002))},
		}}
		code, stdout, stderr := runWithClient(t, env, client, "a ranger")

		require.Zero(t, code, stderr)
		require.NotEmpty(t, stdout)
		require.Len(t, client.requests, 2)
		assert.Contains(t, client.requests[1].Prompt, "previous answer was rejected")
		require.Len(t, readLedger(t, env["EVAL_COST_LEDGER"]), 2)
	})

	t.Run("Error: a rejected output after every retry fails without printing a sheet", func(t *testing.T) {
		env := baseEnv(t)
		results := make([]fakeResult, generation.MaxSchemaRetries+1)
		for i := range results {
			results[i] = fakeResult{resp: llm.Response{Output: []byte(`{"identity":{"name":"Only"}}`), Model: "probe/model", Usage: llm.Usage{CostUSD: ptrFloat(0.001)}}}
		}
		client := &fakeClient{results: results}
		code, stdout, stderr := runWithClient(t, env, client, "a rogue")

		require.NotZero(t, code)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "output rejected")
		require.Len(t, client.requests, generation.MaxSchemaRetries+1)
	})
}

func TestRunCostGuard(t *testing.T) {
	t.Run("Error: a response without cost blocks the eval", func(t *testing.T) {
		env := baseEnv(t)
		client := &fakeClient{results: []fakeResult{{resp: draftResponse(t, nil)}}}
		code, stdout, stderr := runWithClient(t, env, client, "a cleric")

		require.NotZero(t, code)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "cost is unknown")
		_, err := os.Stat(env["EVAL_COST_LEDGER"] + ".blocked")
		require.NoError(t, err)

		second := &fakeClient{results: []fakeResult{{resp: draftResponse(t, ptrFloat(0.01))}}}
		code, _, stderr = runWithClient(t, env, second, "a paladin")
		require.NotZero(t, code)
		assert.Contains(t, stderr, "eval aborted")
		assert.Empty(t, second.requests)
	})

	t.Run("Error: the cap stops the next call before it reaches the provider", func(t *testing.T) {
		env := baseEnv(t)
		require.NoError(t, os.WriteFile(env["EVAL_COST_LEDGER"], []byte(`{"model":"probe/model","known":true,"cost_usd":1.0}`+"\n"), 0o600))
		client := &fakeClient{results: []fakeResult{{resp: draftResponse(t, ptrFloat(0.01))}}}
		code, _, stderr := runWithClient(t, env, client, "a monk")

		require.NotZero(t, code)
		assert.Contains(t, stderr, "cost cap")
		assert.Empty(t, client.requests)
	})

	t.Run("Edge: a call that pushes the total over the cap blocks the eval", func(t *testing.T) {
		env := baseEnv(t)
		env["EVAL_MAX_COST_USD"] = "0.01"
		client := &fakeClient{results: []fakeResult{{resp: draftResponse(t, ptrFloat(0.02))}}}
		code, stdout, stderr := runWithClient(t, env, client, "a bard")

		require.NotZero(t, code)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "cost cap")
		_, err := os.Stat(env["EVAL_COST_LEDGER"] + ".blocked")
		require.NoError(t, err)
	})
}

func TestRunOpenRouterRoundTrip(t *testing.T) {
	t.Run("Happy path: the CLI reads usage.cost from the provider response", func(t *testing.T) {
		object := validDraftObject(t)
		rawObject, err := json.Marshal(object)
		require.NoError(t, err)
		quoted, err := json.Marshal(string(rawObject))
		require.NoError(t, err)

		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			body := `{"choices":[{"message":{"role":"assistant","content":` + string(quoted) + `},"finish_reason":"stop"}],` +
				`"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18,"cost":0.0012}}`
			_, _ = io.WriteString(w, body)
		}))
		t.Cleanup(srv.Close)

		env := baseEnv(t)
		env["LLM_BASE_URL"] = srv.URL + "/api/v1"
		var stdout, stderr bytes.Buffer
		code := run([]string{"a city guard"}, getenv(env), &stdout, &stderr, func(cfg openrouter.Config) (llm.Client, error) {
			return openrouter.New(cfg)
		})

		require.Zero(t, code, stderr.String())
		var values map[string]any
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &values))
		assert.Contains(t, gotBody, "temperature")
		assert.InDelta(t, 0, gotBody["temperature"], 1e-9)
		entries := readLedger(t, env["EVAL_COST_LEDGER"])
		require.Len(t, entries, 1)
		require.NotNil(t, entries[0].CostUSD)
		assert.InDelta(t, 0.0012, *entries[0].CostUSD, 1e-9)
	})
}

func TestDataset(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "evals", "generation", "dataset.json"))
	require.NoError(t, err)
	var tests []struct {
		Description string            `json:"description"`
		Vars        map[string]string `json:"vars"`
		Metadata    map[string]string `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(data, &tests))

	require.Len(t, tests, 60)
	seen := map[string]bool{}
	categories := map[string]int{}
	for _, test := range tests {
		require.NotEmpty(t, test.Description)
		require.NotEmpty(t, test.Vars["concept"])
		require.NotEmpty(t, test.Metadata["category"])
		require.False(t, seen[test.Description], "duplicate description %q", test.Description)
		seen[test.Description] = true
		categories[test.Metadata["category"]]++
	}
	assert.Equal(t, 60, categories["classic"]+categories["species"]+categories["background"]+categories["level"]+categories["multiclass"]+categories["nonsrd"]+categories["language"]+categories["edge"])
	assert.NotZero(t, categories["classic"])
	assert.NotZero(t, categories["edge"])
	assert.NotZero(t, categories["language"])
	assert.Len(t, categories, 8)
}

func TestHarnessConfig(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	t.Run("Happy path: the config checks JSON and coherence with one serial provider call", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "evals", "generation", "promptfooconfig.yaml"))
		require.NoError(t, err)
		text := string(data)
		assert.Contains(t, text, "exec: ./exec-provider.sh")
		assert.Contains(t, text, "file://assertions/sheet.js")
		assert.Contains(t, text, "maxConcurrency: 1")
		assert.Contains(t, text, "cache: false")
	})

	t.Run("Edge: the runner keeps the 90% gate and the 1.00 USD cap", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "evals", "generation", "run.sh"))
		require.NoError(t, err)
		text := string(data)
		assert.Contains(t, text, "PROMPTFOO_PASS_RATE_THRESHOLD=90")
		assert.Contains(t, text, "1.00")
		assert.Contains(t, text, "OPENROUTER_API_KEY")
	})

	t.Run("Edge: the workflow runs only manually and nightly, never on a pull request", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "prompt-evals.yml"))
		require.NoError(t, err)
		text := string(data)
		assert.Contains(t, text, "workflow_dispatch:")
		assert.Contains(t, text, "schedule:")
		assert.NotContains(t, text, "pull_request")
		assert.Contains(t, text, "OPENROUTER_API_KEY")
	})
}
