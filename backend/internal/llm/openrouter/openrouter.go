package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"legnaris/backend/internal/llm"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"
)

const (
	DefaultBaseURL = "https://openrouter.ai/api/v1"
	DefaultModel   = "deepseek/deepseek-v4.1-flash"
)

var (
	ErrMissingAPIKey          = errors.New("openrouter: API key is required")
	ErrMissingModel           = errors.New("openrouter: model is required")
	ErrFreeModel              = errors.New("openrouter: :free model variants are refused")
	ErrInvalidBaseURL         = errors.New("openrouter: base URL must be an absolute http or https URL with a host")
	ErrInvalidTimeout         = errors.New("openrouter: timeout must be positive")
	ErrMissingLogger          = errors.New("openrouter: logger is required")
	ErrInvalidReasoningEffort = errors.New("openrouter: reasoning effort must be one of none, minimal, low, medium, high, xhigh")
)

var reasoningEfforts = []string{"none", "minimal", "low", "medium", "high", "xhigh"}

func ReasoningEfforts() []string {
	return slices.Clone(reasoningEfforts)
}

func isValidReasoningEffort(effort string) bool {
	return effort == "" || slices.Contains(reasoningEfforts, effort)
}

const (
	noEndpointsPhrase = "no endpoints found"
	noProvidersPhrase = "no allowed providers"
)

const (
	finishLength    = "length"
	maxErrorMessage = 300
	maxErrorCode    = 64
)

const (
	reasonMalformedResponse = "malformed_response"
	reasonNoChoices         = "no_choices"
	reasonEmptyContent      = "empty_content"
	reasonInvalidJSON       = "invalid_json"
	reasonTruncated         = "truncated"
)

type Config struct {
	BaseURL         string
	APIKey          string
	Model           string
	ReasoningEffort string
	Timeout         time.Duration
	Logger          logger.StructuredLogger
}

type Client struct {
	endpoint        string
	apiKey          string
	model           string
	reasoningEffort string
	http            *http.Client
	logger          logger.StructuredLogger
}

var _ llm.Client = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	endpoint, err := chatCompletionsURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	switch {
	case cfg.Logger == nil:
		return nil, ErrMissingLogger
	case strings.TrimSpace(cfg.APIKey) == "":
		return nil, ErrMissingAPIKey
	case cfg.Model == "":
		return nil, ErrMissingModel
	case strings.HasSuffix(cfg.Model, ":free"):
		return nil, ErrFreeModel
	case cfg.Timeout <= 0:
		return nil, ErrInvalidTimeout
	case !isValidReasoningEffort(cfg.ReasoningEffort):
		return nil, fmt.Errorf("%w: %q", ErrInvalidReasoningEffort, cfg.ReasoningEffort)
	}
	return &Client{
		endpoint:        endpoint,
		apiKey:          cfg.APIKey,
		model:           cfg.Model,
		reasoningEffort: cfg.ReasoningEffort,
		http:            &http.Client{Timeout: cfg.Timeout},
		logger:          cfg.Logger,
	}, nil
}

func chatCompletionsURL(base string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidBaseURL, base)
	}
	return strings.TrimSuffix(base, "/") + "/chat/completions", nil
}

func (c *Client) Generate(ctx context.Context, req llm.Request) (llm.Response, error) {
	if err := req.Validate(); err != nil {
		return llm.Response{}, err
	}
	payload, err := json.Marshal(c.buildRequest(req))
	if err != nil {
		return llm.Response{}, fmt.Errorf("%w: encode request: %w", llm.ErrPermanent, err)
	}

	started := time.Now()
	status, raw, err := c.post(ctx, payload)
	latency := time.Since(started)
	if err != nil {
		return llm.Response{}, c.failure(ctx, 0, latency, err)
	}
	if status/100 != 2 {
		return llm.Response{}, c.failure(ctx, status, latency, statusError(status, raw), upstreamAttrs(raw)...)
	}
	return c.parse(ctx, raw, latency)
}

func (c *Client) post(ctx context.Context, payload []byte) (int, []byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, fmt.Errorf("%w: build request: %w", llm.ErrPermanent, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: gateway request failed: %w", llm.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, sharedjson.MaxBodySize+1))
	if err != nil {
		return 0, nil, fmt.Errorf("%w: read gateway response: %w", llm.ErrTransient, err)
	}
	return resp.StatusCode, raw, nil
}

func statusError(status int, raw []byte) error {
	if isNoEligibleProvider(raw) {
		return fmt.Errorf("%w: status %d", llm.ErrProviderUnavailable, status)
	}
	if status == http.StatusTooManyRequests || status >= http.StatusInternalServerError {
		return fmt.Errorf("%w: status %d", llm.ErrTransient, status)
	}
	return fmt.Errorf("%w: status %d", llm.ErrPermanent, status)
}

func isNoEligibleProvider(raw []byte) bool {
	message, _ := upstreamError(raw)
	message = strings.ToLower(message)
	return strings.Contains(message, noEndpointsPhrase) || strings.Contains(message, noProvidersPhrase)
}

func upstreamAttrs(raw []byte) []logger.Attribute {
	message, code := upstreamError(raw)
	attrs := make([]logger.Attribute, 0, 2)
	if message != "" {
		attrs = append(attrs, logger.Attribute{Key: "error_message", Value: logger.Truncate(message, maxErrorMessage)})
	}
	if code != "" {
		attrs = append(attrs, logger.Attribute{Key: "upstream_code", Value: code})
	}
	return attrs
}

func upstreamError(raw []byte) (string, string) {
	var body struct {
		Error struct {
			Message string          `json:"message"`
			Code    json.RawMessage `json:"code"`
		} `json:"error"`
	}
	if err := sharedjson.DecodeBytes(raw, &body); err != nil {
		return "", ""
	}
	return strings.TrimSpace(body.Error.Message), upstreamCode(body.Error.Code)
}

func upstreamCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := sharedjson.DecodeBytes(raw, &text); err == nil {
		return logger.Truncate(text, maxErrorCode)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" {
		return ""
	}
	return logger.Truncate(trimmed, maxErrorCode)
}

func (c *Client) failure(ctx context.Context, status int, latency time.Duration, err error, extra ...logger.Attribute) error {
	attrs := []logger.Attribute{
		{Key: "model", Value: c.model},
		{Key: "status", Value: strconv.Itoa(status)},
		{Key: "latency_ms", Value: strconv.FormatInt(latency.Milliseconds(), 10)},
		{Key: "reasoning_effort", Value: c.reasoningEffortLabel()},
		{Key: "error_class", Value: errorClass(err)},
	}
	attrs = append(attrs, extra...)
	c.logger.Warn(ctx, "llm generation failed", withGenerationID(ctx, attrs)...)
	return err
}

func (c *Client) reasoningEffortLabel() string {
	if c.reasoningEffort == "" {
		return "default"
	}
	return c.reasoningEffort
}

func withGenerationID(ctx context.Context, attrs []logger.Attribute) []logger.Attribute {
	id := llm.GenerationID(ctx)
	if id == "" {
		return attrs
	}
	out := make([]logger.Attribute, 0, len(attrs)+1)
	out = append(out, logger.Attribute{Key: "generation_id", Value: id})
	return append(out, attrs...)
}

func errorClass(err error) string {
	switch {
	case errors.Is(err, llm.ErrProviderUnavailable):
		return "provider_unavailable"
	case errors.Is(err, llm.ErrTransient):
		return "transient"
	default:
		return "permanent"
	}
}

func (c *Client) parse(ctx context.Context, raw []byte, latency time.Duration) (llm.Response, error) {
	var body chatResponse
	if err := sharedjson.DecodeBytes(raw, &body); err != nil {
		return llm.Response{}, c.failure(ctx, http.StatusOK, latency,
			fmt.Errorf("%w: malformed gateway response: %w", llm.ErrPermanent, err),
			completionAttrs(reasonMalformedResponse, body)...)
	}
	if len(body.Choices) == 0 {
		return llm.Response{}, c.failure(ctx, http.StatusOK, latency,
			fmt.Errorf("%w: no completion choices", llm.ErrPermanent),
			completionAttrs(reasonNoChoices, body)...)
	}
	content := body.Choices[0].Message.Content
	reason, err := classifyCompletion(body.finishReason(), content)
	if err != nil {
		return llm.Response{}, c.failure(ctx, http.StatusOK, latency, err, completionAttrs(reason, body)...)
	}

	usage := llm.Usage{
		PromptTokens:     body.Usage.PromptTokens,
		CompletionTokens: body.Usage.CompletionTokens,
		TotalTokens:      body.Usage.TotalTokens,
		ReasoningTokens:  body.Usage.CompletionTokensDetails.ReasoningTokens,
		CostUSD:          body.Usage.Cost,
	}
	model := body.Model
	if model == "" {
		model = c.model
	}
	c.logger.Info(ctx, "llm generation completed", withGenerationID(ctx, []logger.Attribute{
		{Key: "model", Value: model},
		{Key: "status", Value: strconv.Itoa(http.StatusOK)},
		{Key: "latency_ms", Value: strconv.FormatInt(latency.Milliseconds(), 10)},
		{Key: "reasoning_effort", Value: c.reasoningEffortLabel()},
		{Key: "prompt_tokens", Value: strconv.Itoa(usage.PromptTokens)},
		{Key: "completion_tokens", Value: strconv.Itoa(usage.CompletionTokens)},
		{Key: "total_tokens", Value: strconv.Itoa(usage.TotalTokens)},
		{Key: "reasoning_tokens", Value: strconv.Itoa(usage.ReasoningTokens)},
		{Key: "finish_reason", Value: body.finishReason()},
	})...)
	return llm.Response{Output: json.RawMessage(content), Usage: usage, FinishReason: body.finishReason(), Model: model}, nil
}

func classifyCompletion(finish, content string) (string, error) {
	switch {
	case content == "":
		return reasonEmptyContent, fmt.Errorf("%w: completion content is empty", llm.ErrTransient)
	case !json.Valid([]byte(content)):
		if finish == finishLength {
			return reasonInvalidJSON, fmt.Errorf("%w: truncated output is not valid JSON", llm.ErrTransient)
		}
		return reasonInvalidJSON, fmt.Errorf("%w: output is not valid JSON", llm.ErrPermanent)
	case finish == finishLength:
		return reasonTruncated, fmt.Errorf("%w: output was truncated", llm.ErrTransient)
	default:
		return "", nil
	}
}

func completionAttrs(reason string, body chatResponse) []logger.Attribute {
	return []logger.Attribute{
		{Key: "reason", Value: reason},
		{Key: "finish_reason", Value: body.finishReason()},
		{Key: "completion_tokens", Value: strconv.Itoa(body.Usage.CompletionTokens)},
		{Key: "reasoning_tokens", Value: strconv.Itoa(body.Usage.CompletionTokensDetails.ReasoningTokens)},
	}
}

func (c *Client) buildRequest(req llm.Request) chatRequest {
	messages := make([]message, 0, 2)
	if req.System != "" {
		messages = append(messages, message{Role: "system", Content: req.System})
	}
	messages = append(messages, message{Role: "user", Content: req.Prompt})

	request := chatRequest{
		Model:     c.model,
		MaxTokens: req.MaxTokens,
		Messages:  messages,
		ResponseFormat: responseFormat{
			Type: "json_schema",
			JSONSchema: jsonSchema{
				Name:   "output",
				Strict: true,
				Schema: req.Schema,
			},
		},
		Provider: providerRouting{
			RequireParameters: true,
			AllowFallbacks:    true,
		},
	}
	if c.reasoningEffort != "" {
		request.Reasoning = &reasoningConfig{Effort: c.reasoningEffort}
	}
	return request
}

type chatRequest struct {
	Model          string           `json:"model"`
	MaxTokens      int              `json:"max_tokens"`
	Messages       []message        `json:"messages"`
	ResponseFormat responseFormat   `json:"response_format"`
	Provider       providerRouting  `json:"provider"`
	Reasoning      *reasoningConfig `json:"reasoning,omitempty"`
}

type reasoningConfig struct {
	Effort string `json:"effort"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type providerRouting struct {
	RequireParameters bool `json:"require_parameters"`
	AllowFallbacks    bool `json:"allow_fallbacks"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens            int      `json:"prompt_tokens"`
		CompletionTokens        int      `json:"completion_tokens"`
		TotalTokens             int      `json:"total_tokens"`
		Cost                    *float64 `json:"cost"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

func (r chatResponse) finishReason() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].FinishReason
}
