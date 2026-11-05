package email

import (
	"context"

	"legnaris/backend/shared/logger"
)

// DevSender is the local-development sender behind EMAIL_PROVIDER=dev. Unlike
// LogSender it writes the message body in clear, so a developer can read the
// one-time code without a mailbox. It makes no provider call. Production must
// never run it: the factory refuses to build it when APP_ENV is production.
type DevSender struct {
	logger logger.StructuredLogger
}

// NewDevSender builds the development sender over a structured logger.
func NewDevSender(log logger.StructuredLogger) *DevSender {
	return &DevSender{logger: log}
}

// Send logs the outbound message, including its text body that carries the
// code, and returns nil.
func (s *DevSender) Send(ctx context.Context, m Outbound) error {
	s.logger.Info(ctx, "development email send (dev sender: body logged in clear)",
		logger.Attribute{Key: "idempotency_key", Value: m.IdempotencyKey},
		logger.Attribute{Key: "from", Value: logSenderFrom},
		logger.Attribute{Key: "to", Value: m.To},
		logger.Attribute{Key: "subject", Value: m.Subject},
		logger.Attribute{Key: "text", Value: m.Text},
	)
	return nil
}
