package amqp

import (
	"time"

	"legnaris/backend/internal/email"

	amqp "github.com/rabbitmq/amqp091-go"
)

// provision derives the broker properties that must accompany the encoded
// message: JSON content type, persistent delivery and the intention identity as
// the broker-level MessageId (EML-07). The publication time is stamped because
// the broker reports the head message's age only from that property, and the
// oldest-message alert reads it (OBS-03). The body is set by the publisher; the
// code is deliberately absent from every derived property (EML-13). Deriving
// the properties here keeps backend/internal/email free of the broker library.
func provision(msg email.Message) amqp.Publishing {
	return amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    msg.ID,
		Timestamp:    time.Now(),
	}
}
