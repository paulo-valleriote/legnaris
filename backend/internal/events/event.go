package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"legnaris/backend/shared/idgen"
)

const ContractVersion = 1

var ErrInvalidEvent = errors.New("invalid domain event")

const (
	ReasonBufferFull        = "buffer_full"
	ReasonBrokerUnavailable = "broker_unavailable"
	ReasonPublishFailed     = "publish_failed"
	ReasonShutdown          = "shutdown"
	ReasonInvalidEvent      = "invalid_event"
)

var typePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

type Event struct {
	Type             string
	ActorID          *string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	Data             any
}

type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurred_at"`
	ActorID    *string         `json:"actor_id"`
	Aggregate  Aggregate       `json:"aggregate"`
	Data       json.RawMessage `json:"data"`
}

type Aggregate struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

type Publisher interface {
	Publish(ev Event)
}

type NoOp struct{}

func (NoOp) Publish(Event) {}

type DropRecorder interface {
	EventDropped(eventType, reason string)
}

func Encode(ev Event, id string, at time.Time) ([]byte, error) {
	if err := validate(ev, id); err != nil {
		return nil, err
	}

	data, err := json.Marshal(ev.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: encode data: %v", ErrInvalidEvent, err)
	}

	env := Envelope{
		ID:         id,
		Type:       ev.Type,
		Version:    ContractVersion,
		OccurredAt: at.UTC(),
		ActorID:    ev.ActorID,
		Aggregate: Aggregate{
			Type:    ev.AggregateType,
			ID:      ev.AggregateID,
			Version: ev.AggregateVersion,
		},
		Data: data,
	}

	body, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("%w: encode envelope: %v", ErrInvalidEvent, err)
	}
	return body, nil
}

func Decode(body []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	return env, nil
}

func validate(ev Event, id string) error {
	switch {
	case idgen.Validate(id) != nil:
		return fmt.Errorf("%w: envelope id is not a UUID v7", ErrInvalidEvent)
	case !typePattern.MatchString(ev.Type):
		return fmt.Errorf("%w: type %q", ErrInvalidEvent, ev.Type)
	case ev.AggregateType == "":
		return fmt.Errorf("%w: empty aggregate type", ErrInvalidEvent)
	case idgen.Validate(ev.AggregateID) != nil:
		return fmt.Errorf("%w: aggregate id is not a UUID v7", ErrInvalidEvent)
	case ev.ActorID != nil && idgen.Validate(*ev.ActorID) != nil:
		return fmt.Errorf("%w: actor id is not a UUID v7", ErrInvalidEvent)
	case ev.AggregateVersion < 0:
		return fmt.Errorf("%w: negative aggregate version", ErrInvalidEvent)
	}
	return nil
}
