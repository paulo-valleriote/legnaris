// Package events is the domain event contract (ADR 0010, ADR 0013): the
// envelope every consumer decodes, the Publisher port that services call, and
// the drop reasons counted in legnaris_events_dropped_total. The AMQP adapter
// lives in the amqp subpackage.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"legnaris/backend/shared/idgen"
)

// ContractVersion is the envelope version. Additive changes keep it at 1
// (ADR 0010, ADR 0013).
const ContractVersion = 1

// ErrInvalidEvent reports an event that cannot form a valid envelope. It is a
// plain sentinel: check with errors.Is.
var ErrInvalidEvent = errors.New("invalid domain event")

// Drop reasons carried by the legnaris_events_dropped_total reason label.
const (
	ReasonBufferFull        = "buffer_full"
	ReasonBrokerUnavailable = "broker_unavailable"
	ReasonPublishFailed     = "publish_failed"
	ReasonShutdown          = "shutdown"
	ReasonInvalidEvent      = "invalid_event"
)

// typePattern enforces <aggregate>.<past_tense_verb> in lower snake case
// (ADR 0010, Naming).
var typePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

// Event is what a service hands to the Publisher. The envelope id and the
// occurrence time are assigned by the publisher, not by the caller.
type Event struct {
	Type             string
	ActorID          *string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	// Data carries IDs, changed keys and metadata only. Never Sheet values, PII
	// or generation output (ADR 0010, Payload privacy).
	Data any
}

// Envelope is the wire contract. Decoders tolerate unknown fields.
type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurred_at"`
	ActorID    *string         `json:"actor_id"`
	Aggregate  Aggregate       `json:"aggregate"`
	Data       json.RawMessage `json:"data"`
}

// Aggregate identifies the entity an event is about. Version is zero when the
// aggregate carries no version.
type Aggregate struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

// Publisher is the port services use to emit best-effort events. Publish never
// blocks and never fails the caller: delivery problems are counted, not
// returned (ADR 0010).
type Publisher interface {
	Publish(ev Event)
}

// NoOp is a Publisher that discards every event. It is the wiring default when
// no broker is configured.
type NoOp struct{}

// Publish discards ev.
func (NoOp) Publish(Event) {}

// DropRecorder counts events that were dropped instead of published.
type DropRecorder interface {
	EventDropped(eventType, reason string)
}

// Encode validates ev and returns its envelope as JSON, using id as the
// envelope id and at as occurred_at. Callers pass a fresh idgen id.
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

// Decode parses an envelope, ignoring fields it does not know.
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
