package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/env"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"
)

const (
	defaultMaxCostUSD = 1.00
	defaultLedgerName = "legnaris-generation-eval-cost.jsonl"
	evalKind          = plan.KindCharacter
)

type clientFactory func(openrouter.Config) (llm.Client, error)

type evalInfo struct {
	TemplateID    string `json:"template_id"`
	Kind          string `json:"kind"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
}

type costEntry struct {
	Model         string   `json:"model"`
	PromptVersion string   `json:"prompt_version"`
	CostUSD       *float64 `json:"cost_usd,omitempty"`
	Known         bool     `json:"known"`
	At            string   `json:"at"`
}

type discardLogger struct{}

func (discardLogger) Info(context.Context, string, ...logger.Attribute) {}

func (discardLogger) Debug(context.Context, string, ...logger.Attribute) {}

func (discardLogger) Warn(context.Context, string, ...logger.Attribute) {}

func (discardLogger) Error(context.Context, string, ...logger.Attribute) {}

func (discardLogger) Fatal(context.Context, string, ...logger.Attribute) {}

func (l discardLogger) With(string, string) logger.StructuredLogger {
	return l
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, func(cfg openrouter.Config) (llm.Client, error) {
		return openrouter.New(cfg)
	}))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer, newClient clientFactory) int {
	flags := flag.NewFlagSet("geneval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	infoOnly := flags.Bool("info", false, "print the prompt version, model and template without calling the provider")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	tmpl, dag, err := loadDefaultTemplate()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "geneval:", err)
		return 1
	}
	system, err := generation.SystemPrompt(tmpl, evalKind)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "geneval:", err)
		return 1
	}
	version := generation.PromptVersion(system)
	model := modelFromEnvironment(getenv)

	if *infoOnly {
		encoded, err := json.Marshal(evalInfo{
			TemplateID:    tmpl.ID,
			Kind:          string(evalKind),
			Model:         model,
			PromptVersion: version,
		})
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "geneval:", err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(encoded))
		return 0
	}

	concept := strings.TrimSpace(flags.Arg(0))
	if concept == "" {
		_, _ = fmt.Fprintln(stderr, "geneval: a concept is required")
		return 2
	}

	ledgerPath := strings.TrimSpace(getenv("EVAL_COST_LEDGER"))
	if ledgerPath == "" {
		ledgerPath = filepath.Join(os.TempDir(), defaultLedgerName)
	}
	maxCost, err := maxCostFromEnvironment(getenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "geneval:", err)
		return 1
	}
	cfg, err := clientConfig(getenv, model)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "geneval:", err)
		return 1
	}
	client, err := newClient(cfg)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "geneval:", err)
		return 1
	}

	values, err := draft(context.Background(), client, tmpl, dag, concept, ledgerPath, version, model, maxCost)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "geneval:", err)
		return 1
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "geneval: encode sheet:", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, string(encoded))
	return 0
}

func loadDefaultTemplate() (*template.Template, *template.DAG, error) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	if err != nil {
		return nil, nil, fmt.Errorf("build template registry: %w", err)
	}
	defaultID := ""
	for _, tmpl := range template.DefaultTemplates() {
		if tmpl.Default {
			defaultID = tmpl.ID
			break
		}
	}
	if defaultID == "" {
		return nil, nil, errors.New("no default template")
	}
	tmpl, ok := registry.FetchByID(defaultID)
	if !ok {
		return nil, nil, fmt.Errorf("default template %q is not in the registry", defaultID)
	}
	dag, ok := registry.DAG(defaultID)
	if !ok {
		return nil, nil, fmt.Errorf("default template %q has no DAG", defaultID)
	}
	return tmpl, dag, nil
}

func modelFromEnvironment(getenv func(string) string) string {
	if value := strings.TrimSpace(getenv("LLM_MODEL")); value != "" {
		return value
	}
	return openrouter.DefaultModel
}

func clientConfig(getenv func(string) string, model string) (openrouter.Config, error) {
	provider := strings.TrimSpace(getenv("LLM_PROVIDER"))
	if provider != "" && provider != "openrouter" {
		return openrouter.Config{}, fmt.Errorf("LLM_PROVIDER %q is not supported", provider)
	}
	baseURL := strings.TrimSpace(getenv("LLM_BASE_URL"))
	if baseURL == "" {
		baseURL = openrouter.DefaultBaseURL
	}
	timeout := env.DefaultLLMTimeout
	if value := strings.TrimSpace(getenv("LLM_TIMEOUT")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return openrouter.Config{}, fmt.Errorf("LLM_TIMEOUT: %w", err)
		}
		timeout = parsed
	}
	return openrouter.Config{
		BaseURL:         baseURL,
		APIKey:          strings.TrimSpace(getenv("LLM_API_KEY")),
		Model:           model,
		ReasoningEffort: strings.TrimSpace(getenv("LLM_REASONING_EFFORT")),
		Timeout:         timeout,
		Logger:          discardLogger{},
	}, nil
}

func maxCostFromEnvironment(getenv func(string) string) (float64, error) {
	value := strings.TrimSpace(getenv("EVAL_MAX_COST_USD"))
	if value == "" {
		return defaultMaxCostUSD, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("EVAL_MAX_COST_USD %q must be a positive number", value)
	}
	return parsed, nil
}

func draft(ctx context.Context, client llm.Client, tmpl *template.Template, dag *template.DAG, concept, ledgerPath, promptVersion, model string, maxCost float64) (map[string]any, error) {
	system, err := generation.SystemPrompt(tmpl, evalKind)
	if err != nil {
		return nil, err
	}
	schema, err := generation.OutputSchema(tmpl, evalKind)
	if err != nil {
		return nil, err
	}
	temperature := 0.0
	problems := []string(nil)
	for attempt := 0; attempt <= generation.MaxSchemaRetries; attempt++ {
		if err := spendAllowed(ledgerPath, maxCost); err != nil {
			return nil, err
		}
		resp, err := client.Generate(ctx, llm.Request{
			System:      system,
			Prompt:      generation.UserPrompt(evalKind, concept, problems),
			Schema:      schema,
			MaxTokens:   generation.MaxOutputTokens,
			Temperature: &temperature,
		})
		if err != nil {
			return nil, err
		}
		cost, known := resp.Usage.Cost()
		responseModel := resp.Model
		if responseModel == "" {
			responseModel = model
		}
		if err := recordCost(ledgerPath, costEntry{
			Model:         responseModel,
			PromptVersion: promptVersion,
			CostUSD:       costPointer(cost, known),
			Known:         known,
			At:            time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return nil, err
		}
		if !known {
			return nil, block(ledgerPath, fmt.Sprintf("cost is unknown for model %s", responseModel))
		}
		if total, _, err := ledgerTotal(ledgerPath); err != nil {
			return nil, err
		} else if total > maxCost {
			return nil, block(ledgerPath, fmt.Sprintf("cost cap %.4f USD exceeded by total %.4f USD", maxCost, total))
		}
		object, err := generation.ParseOutput(resp.Output)
		if err != nil {
			problems = outputProblems(err)
			continue
		}
		expanded, err := generation.Expand(tmpl, evalKind, object)
		if err != nil {
			problems = outputProblems(err)
			continue
		}
		values, err := generation.Assemble(tmpl, dag, evalKind, expanded)
		if err != nil {
			return nil, err
		}
		return values, nil
	}
	return nil, fmt.Errorf("output rejected after %d attempts: %s", generation.MaxSchemaRetries+1, strings.Join(problems, "; "))
}

func costPointer(cost float64, known bool) *float64 {
	if !known {
		return nil
	}
	return &cost
}

func outputProblems(err error) []string {
	var outputErr *generation.OutputError
	if errors.As(err, &outputErr) && len(outputErr.Problems) > 0 {
		return outputErr.Problems
	}
	return []string{err.Error()}
}

func spendAllowed(ledgerPath string, maxCost float64) error {
	if reason, blocked := readBlocked(ledgerPath); blocked {
		return fmt.Errorf("eval aborted: %s", reason)
	}
	total, unknown, err := ledgerTotal(ledgerPath)
	if err != nil {
		return err
	}
	if unknown {
		return block(ledgerPath, "cost is unknown in the ledger")
	}
	if total >= maxCost {
		return block(ledgerPath, fmt.Sprintf("cost cap reached at %.4f USD", total))
	}
	return nil
}

func ledgerTotal(path string) (float64, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("open cost ledger: %w", err)
	}
	defer func() { _ = file.Close() }()

	total := 0.0
	unknown := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry costEntry
		if err := sharedjson.DecodeBytes([]byte(line), &entry); err != nil {
			return 0, false, fmt.Errorf("read cost ledger: %w", err)
		}
		if !entry.Known || entry.CostUSD == nil {
			unknown = true
			continue
		}
		total += *entry.CostUSD
	}
	if err := scanner.Err(); err != nil {
		return 0, false, fmt.Errorf("read cost ledger: %w", err)
	}
	return total, unknown, nil
}

func recordCost(path string, entry costEntry) error {
	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode cost entry: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open cost ledger: %w", err)
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write cost ledger: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close cost ledger: %w", closeErr)
	}
	return nil
}

func block(ledgerPath, reason string) error {
	if err := os.WriteFile(ledgerPath+".blocked", []byte(reason+"\n"), 0o600); err != nil {
		return fmt.Errorf("write eval block: %w", err)
	}
	return fmt.Errorf("eval aborted: %s", reason)
}

func readBlocked(ledgerPath string) (string, bool) {
	data, err := os.ReadFile(ledgerPath + ".blocked")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}
