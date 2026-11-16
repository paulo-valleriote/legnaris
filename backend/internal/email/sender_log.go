package email

import (
	"context"
	"strings"

	"legnaris/backend/shared/logger"
)

const logSenderFrom = "no-reply@legnaris.local"

type LogSender struct {
	logger logger.StructuredLogger
}

func NewLogSender(log logger.StructuredLogger) *LogSender {
	return &LogSender{logger: log}
}

func (s *LogSender) Send(ctx context.Context, m Outbound) error {
	s.logger.Info(ctx, "development email send",
		logger.Attribute{Key: "idempotency_key", Value: m.IdempotencyKey},
		logger.Attribute{Key: "from", Value: logSenderFrom},
		logger.Attribute{Key: "to", Value: MaskRecipient(m.To)},
		logger.Attribute{Key: "subject", Value: m.Subject},
	)
	return nil
}

func MaskRecipient(address string) string {
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 {
		return "***"
	}
	return address[:1] + "***" + address[at:]
}
