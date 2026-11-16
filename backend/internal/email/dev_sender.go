package email

import (
	"context"

	"legnaris/backend/shared/logger"
)

type DevSender struct {
	logger logger.StructuredLogger
}

func NewDevSender(log logger.StructuredLogger) *DevSender {
	return &DevSender{logger: log}
}

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
