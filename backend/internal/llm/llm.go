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
	ReasoningTokens  int
	CostUSD          *float64
}

func (u Usage) Cost() (float64, bool) {
	if u.CostUSD == nil || *u.CostUSD < 0 {
		return 0, false
	}
	return *u.CostUSD, true
}

type Response struct {
	Output       json.RawMessage
	Usage        Usage
	FinishReason string
	Model        string
}

type generationIDKey struct{}

func WithGenerationID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, generationIDKey{}, id)
}

func GenerationID(ctx context.Context) string {
	id, _ := ctx.Value(generationIDKey{}).(string)
	return id
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
