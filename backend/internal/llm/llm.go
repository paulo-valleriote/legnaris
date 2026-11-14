// Package llm is the port the application uses to call a language model. An
// adapter (for example openrouter) implements Client, and callers depend only on
// this package. Errors classify failures so callers retry or give up with
// errors.Is, never by inspecting provider responses.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrTransient marks a failure worth retrying: network errors, timeouts, 429 and
// 5xx responses.
var ErrTransient = errors.New("llm transient failure")

// ErrPermanent marks a failure that retrying cannot fix: invalid requests, 4xx
// responses other than 429, and malformed gateway responses.
var ErrPermanent = errors.New("llm permanent failure")

// ErrProviderUnavailable reports that the gateway has no endpoint that can serve
// the request. It wraps ErrTransient, because the condition can
// clear, and it is kept distinct so the job can fail with provider_unavailable.
var ErrProviderUnavailable = fmt.Errorf("%w: no eligible provider", ErrTransient)

// Request is one structured-output completion. Schema is the JSON Schema the
// output must satisfy and MaxTokens bounds the completion length.
type Request struct {
	System    string
	Prompt    string
	Schema    json.RawMessage
	MaxTokens int
}

// Usage reports the token counts the gateway billed for one completion.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Response is the structured output, which is untrusted data until the caller
// validates it, and the usage reported for the call.
type Response struct {
	Output json.RawMessage
	Usage  Usage
}

// Client generates a structured completion for a Request.
type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

// Validate reports a request the gateway must never see. Each failure wraps
// ErrPermanent, because resending the same request cannot succeed.
func (r Request) Validate() error {
	switch {
	case r.Prompt == "":
		return fmt.Errorf("%w: prompt is empty", ErrPermanent)
	case len(r.Schema) == 0 || !json.Valid(r.Schema):
		return fmt.Errorf("%w: schema is not valid JSON", ErrPermanent)
	case r.MaxTokens <= 0:
		return fmt.Errorf("%w: max tokens must be positive", ErrPermanent)
	}
	return nil
}
