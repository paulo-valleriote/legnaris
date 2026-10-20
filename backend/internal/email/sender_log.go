package email

import (
	"context"
	"strings"

	"legnaris/backend/shared/logger"
)

// logSenderFrom is a built-in placeholder From for the development sender: the
// real provider requires EMAIL_FROM to be non-empty, but the log path must run
// without it (EML-10).
const logSenderFrom = "no-reply@legnaris.local"

// LogSender is the development sender. It performs no network call and writes
// one structured line identifying the message by its identity and the action
// its code-free subject names, with the recipient masked. It never logs the
// code or the message bodies (EML-10, EML-13).
type LogSender struct {
	logger logger.StructuredLogger
}

// NewLogSender builds the development sender over a structured logger.
func NewLogSender(log logger.StructuredLogger) *LogSender {
	return &LogSender{logger: log}
}

// Send records the outbound message and returns nil; no provider is contacted
// (EML-10).
func (s *LogSender) Send(ctx context.Context, m Outbound) error {
	s.logger.Info(ctx, "development email send",
		logger.Attribute{Key: "idempotency_key", Value: m.IdempotencyKey},
		logger.Attribute{Key: "from", Value: logSenderFrom},
		logger.Attribute{Key: "to", Value: MaskRecipient(m.To)},
		logger.Attribute{Key: "subject", Value: m.Subject},
	)
	return nil
}

// MaskRecipient masks an address for logging as the first local-part character
// plus the domain (for example a***@example.com); an address without a usable
// local part or domain becomes *** (EML-13).
func MaskRecipient(address string) string {
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 {
		return "***"
	}
	return address[:1] + "***" + address[at:]
}
