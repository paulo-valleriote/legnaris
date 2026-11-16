package worker

import (
	"context"
	"time"

	"legnaris/backend/internal/email"
)

type Delivery struct {
	Body   []byte
	Handle DeliveryHandle
}

type DeliveryHandle interface {
	Ack() error
}

type DeliverySource interface {
	Consume(ctx context.Context) (<-chan Delivery, error)
}

type Republisher interface {
	PublishTo(ctx context.Context, destination email.Destination, msg email.Message) error
	PublishBodyTo(ctx context.Context, destination email.Destination, body []byte) error
}

type Observer interface {
	SendResult(result string)
	SendDuration(d time.Duration)
	MessageDead()
	MessageExpired()
	BreakerOpened()
	BreakerClosed()
}

const (
	ResultSent         = "sent"
	ResultRetried      = "retried"
	ResultDeadLettered = "dead_lettered"
	ResultExpired      = "expired"
)
