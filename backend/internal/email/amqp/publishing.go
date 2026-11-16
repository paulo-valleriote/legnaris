package amqp

import (
	"legnaris/backend/internal/email"

	amqp "github.com/rabbitmq/amqp091-go"
)

func provision(msg email.Message) amqp.Publishing {
	return amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    msg.ID,
	}
}
