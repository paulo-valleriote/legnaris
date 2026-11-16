package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrTransient = errors.New("llm transient failure")

var ErrPermanent = errors.New("llm permanent failure")

var ErrProviderUnavailable = fmt.Errorf("%w: no eligible provider", ErrTransient)

type Request struct {
	System    string
	Prompt    string
	Schema    json.RawMessage
	MaxTokens int
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type Response struct {
	Output json.RawMessage
	Usage  Usage
}

type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

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
