// Package resend is the real provider adapter: it sends over HTTPS through the
// official SDK with a stable idempotency key and a bounded timeout, and
// classifies every failure as retryable or permanent (EML-09, EML-12).
package resend

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"legnaris/internal/email"

	resend "github.com/resend/resend-go/v4"
)

// ErrMissingConfiguration reports an empty API key or From address at
// construction; the adapter refuses to build rather than fail on first send
// (EML-12).
var ErrMissingConfiguration = errors.New("resend: missing configuration")

// Sender is the Resend-backed email.Sender.
type Sender struct {
	client *resend.Client
	from   string
}

// New builds the adapter. It refuses an empty apiKey or from, so a real
// deployment must set RESEND_API_KEY and EMAIL_FROM (EML-12).
func New(apiKey, from string, timeout time.Duration) (*Sender, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(from) == "" {
		return nil, fmt.Errorf("%w: RESEND_API_KEY and EMAIL_FROM must be set", ErrMissingConfiguration)
	}

	httpClient := &http.Client{
		Transport: statusTransport{base: http.DefaultTransport},
		Timeout:   timeout,
	}

	return &Sender{client: resend.NewCustomClient(httpClient, apiKey), from: from}, nil
}

// newWithBaseURL builds a Sender pointed at a test server; it exists so the
// suite can drive the SDK over httptest with no real key or network (EML-10).
func newWithBaseURL(apiKey, from, baseURL string, timeout time.Duration) (*Sender, error) {
	sender, err := New(apiKey, from, timeout)
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	sender.client.BaseURL = parsed
	return sender, nil
}

// Send posts the outbound message through the SDK with the intention identity
// as the idempotency key and maps the outcome to the classification sentinels
// (EML-09, EML-12).
func (s *Sender) Send(ctx context.Context, m email.Outbound) error {
	_, err := s.client.Emails.SendWithOptions(ctx,
		&resend.SendEmailRequest{
			From:    s.from,
			To:      []string{m.To},
			Subject: m.Subject,
			Html:    m.HTML,
			Text:    m.Text,
		},
		&resend.SendEmailOptions{IdempotencyKey: m.IdempotencyKey},
	)
	if err == nil {
		return nil
	}
	return classify(err)
}

// classify maps a send failure onto ErrTransient or ErrPermanent: a rate limit,
// a server error, a timeout or a network failure may succeed on retry, while a
// client error, a malformed request or a missing field cannot (EML-12).
func classify(err error) error {
	var status *statusError
	if errors.As(err, &status) {
		if status.Status == http.StatusTooManyRequests || status.Status >= http.StatusInternalServerError {
			return fmt.Errorf("%w: provider status %d", email.ErrTransient, status.Status)
		}
		return fmt.Errorf("%w: provider status %d", email.ErrPermanent, status.Status)
	}

	var missingFields *resend.MissingRequiredFieldsError
	if errors.As(err, &missingFields) {
		return fmt.Errorf("%w: %v", email.ErrPermanent, err)
	}
	if errors.Is(err, resend.ErrFailedToCreateEmailsSendRequest) {
		return fmt.Errorf("%w: %v", email.ErrPermanent, err)
	}
	if errors.Is(err, resend.ErrRateLimit) || isTimeout(err) {
		return fmt.Errorf("%w: %v", email.ErrTransient, err)
	}
	return fmt.Errorf("%w: %v", email.ErrTransient, err)
}

// isTimeout reports whether err is a network or context timeout, which may
// succeed on retry (EML-12).
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
