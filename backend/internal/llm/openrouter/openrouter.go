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
	ErrMissingAPIKey  = errors.New("openrouter: API key is required")
	ErrMissingModel   = errors.New("openrouter: model is required")
	ErrFreeModel      = errors.New("openrouter: :free model variants are refused (ADR 0008)")
	ErrInvalidBaseURL = errors.New("openrouter: base URL must be an absolute http or https URL with a host")
	ErrInvalidTimeout = errors.New("openrouter: timeout must be positive")
	ErrMissingLogger  = errors.New("openrouter: logger is required")
)

const (
	noEndpointsPhrase = "no endpoints found"
	noProvidersPhrase = "no allowed providers"
)

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
	Logger  logger.StructuredLogger
}

type Client struct {
	endpoint string
	apiKey   string
	model    string
	http     *http.Client
	logger   logger.StructuredLogger
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
	}
	return &Client{
		endpoint: endpoint,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		http:     &http.Client{Timeout: cfg.Timeout},
		logger:   cfg.Logger,
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
		return llm.Response{}, c.failure(ctx, status, latency, statusError(status, raw))
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
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := sharedjson.DecodeBytes(raw, &body); err != nil {
		return false
	}
	message := strings.ToLower(body.Error.Message)
	return strings.Contains(message, noEndpointsPhrase) || strings.Contains(message, noProvidersPhrase)
}

func (c *Client) failure(ctx context.Context, status int, latency time.Duration, err error) error {
	c.logger.Warn(ctx, "llm generation failed",
		logger.Attribute{Key: "model", Value: c.model},
		logger.Attribute{Key: "status", Value: strconv.Itoa(status)},
		logger.Attribute{Key: "latency_ms", Value: strconv.FormatInt(latency.Milliseconds(), 10)},
		logger.Attribute{Key: "error_class", Value: errorClass(err)},
	)
	return err
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
			fmt.Errorf("%w: malformed gateway response: %w", llm.ErrPermanent, err))
	}
	if len(body.Choices) == 0 {
		return llm.Response{}, c.failure(ctx, http.StatusOK, latency,
			fmt.Errorf("%w: no completion choices", llm.ErrPermanent))
	}
	content := body.Choices[0].Message.Content
	if content == "" || !json.Valid([]byte(content)) {
		return llm.Response{}, c.failure(ctx, http.StatusOK, latency,
			fmt.Errorf("%w: output is not valid JSON", llm.ErrPermanent))
	}

	usage := llm.Usage{
		PromptTokens:     body.Usage.PromptTokens,
		CompletionTokens: body.Usage.CompletionTokens,
		TotalTokens:      body.Usage.TotalTokens,
	}
	c.logger.Info(ctx, "llm generation completed",
		logger.Attribute{Key: "model", Value: c.model},
		logger.Attribute{Key: "status", Value: strconv.Itoa(http.StatusOK)},
		logger.Attribute{Key: "latency_ms", Value: strconv.FormatInt(latency.Milliseconds(), 10)},
		logger.Attribute{Key: "prompt_tokens", Value: strconv.Itoa(usage.PromptTokens)},
		logger.Attribute{Key: "completion_tokens", Value: strconv.Itoa(usage.CompletionTokens)},
		logger.Attribute{Key: "total_tokens", Value: strconv.Itoa(usage.TotalTokens)},
	)
	return llm.Response{Output: json.RawMessage(content), Usage: usage}, nil
}

func (c *Client) buildRequest(req llm.Request) chatRequest {
	messages := make([]message, 0, 2)
	if req.System != "" {
		messages = append(messages, message{Role: "system", Content: req.System})
	}
	messages = append(messages, message{Role: "user", Content: req.Prompt})

	return chatRequest{
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
}

type chatRequest struct {
	Model          string          `json:"model"`
	MaxTokens      int             `json:"max_tokens"`
	Messages       []message       `json:"messages"`
	ResponseFormat responseFormat  `json:"response_format"`
	Provider       providerRouting `json:"provider"`
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
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
