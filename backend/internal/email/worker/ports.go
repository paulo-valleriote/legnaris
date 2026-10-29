package worker

import (
	"context"
	"time"

	"legnaris/backend/internal/email"
)

// Delivery is one message handed to the worker: the raw body and the handle
// that acknowledges it. The worker never sees a broker struct (WKR-02).
type Delivery struct {
	Body   []byte
	Handle DeliveryHandle
}

// DeliveryHandle acknowledges one delivery. A delivery that is never
// acknowledged stays with the broker, which redelivers it (WKR-13).
type DeliveryHandle interface {
	Ack() error
}

// DeliverySource hands the worker one delivery at a time. The stream ends when
// the broker cancels the consumer or the channel closes; the worker then
// registers again (WKR-17).
type DeliverySource interface {
	Consume(ctx context.Context) (<-chan Delivery, error)
}

// Republisher publishes to a destination and reports success only once the
// broker confirmed it accepted the message for routing (WKR-09, WKR-13).
type Republisher interface {
	// PublishTo publishes a decoded message.
	PublishTo(ctx context.Context, destination email.Destination, msg email.Message) error
	// PublishBodyTo publishes a body that could not be decoded, unchanged, so an
	// operator can inspect what arrived.
	PublishBodyTo(ctx context.Context, destination email.Destination, body []byte) error
}

// Observer receives the pipeline's outcomes. No argument carries a code or an
// address (WKR-21).
type Observer interface {
	SendResult(result string)
	SendDuration(d time.Duration)
	MessageDead()
	MessageExpired()
}

// Delivery outcomes reported to the Observer.
const (
	ResultSent         = "sent"
	ResultRetried      = "retried"
	ResultDeadLettered = "dead_lettered"
	ResultExpired      = "expired"
)
