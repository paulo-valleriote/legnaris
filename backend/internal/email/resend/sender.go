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
	"strconv"
	"strings"
	"time"

	"legnaris/backend/internal/email"

	resend "github.com/resend/resend-go/v4"
)

// maxRateRetryWait bounds how long the adapter waits on a Retry-After before
// retrying once in place. A longer wait is left to the worker's retry tiers
// (LGN-156).
const maxRateRetryWait = 5 * time.Second

// ErrMissingConfiguration reports an empty API key or From address at
// construction; the adapter refuses to build rather than fail on first send
// (EML-12).
var ErrMissingConfiguration = errors.New("resend: missing configuration")

// Sender is the Resend-backed email.Sender.
type Sender struct {
	client *resend.Client
	from   string
	quota  *quotaGate
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

	return &Sender{
		client: resend.NewCustomClient(httpClient, apiKey),
		from:   from,
		quota:  newQuotaGate(time.Now),
	}, nil
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
// (EML-09, EML-12). While the provider quota is known to be spent, Send fails
// without a provider call. A rate limit with a short Retry-After is retried once
// in place, reusing the same idempotency key (LGN-156).
func (s *Sender) Send(ctx context.Context, m email.Outbound) error {
	if until, blocked := s.quota.blocked(); blocked {
		return fmt.Errorf("%w: %w: sending resumes at %s", email.ErrQuotaExhausted, email.ErrTransient, until.Format(time.RFC3339))
	}

	err := s.dispatch(ctx, m)
	if wait, ok := rateRetryWait(err); ok && sleepContext(ctx, wait) == nil {
		err = s.dispatch(ctx, m)
	}
	if err == nil {
		return nil
	}
	return s.classify(err)
}

// dispatch performs one SDK call and returns its raw error.
func (s *Sender) dispatch(ctx context.Context, m email.Outbound) error {
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
	return err
}

// rateRetryWait reports the Retry-After wait of a pure rate-limit response when
// it is short enough to retry in place.
func rateRetryWait(err error) (time.Duration, bool) {
	var status *statusError
	if !errors.As(err, &status) || status.Status != http.StatusTooManyRequests {
		return 0, false
	}
	if name := providerQuotaName(status.Body); name == quotaNameDaily || name == quotaNameMonthly {
		return 0, false
	}
	seconds, parseErr := strconv.Atoi(strings.TrimSpace(status.Header.Get("Retry-After")))
	if parseErr != nil || seconds < 0 {
		return 0, false
	}
	wait := time.Duration(seconds) * time.Second
	return wait, wait <= maxRateRetryWait
}

// sleepContext waits for d or until ctx is done, whichever comes first.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// classify maps a send failure onto ErrTransient or ErrPermanent: a rate limit,
// a server error, a timeout or a network failure may succeed on retry, while a
// client error, a malformed request or a missing field cannot (EML-12). A 429
// is split further: a spent daily or monthly quota closes the local gate until
// it resets, and a rate limit wraps ErrRateLimited. Both wrap ErrTransient
// (LGN-156).
func (s *Sender) classify(err error) error {
	var status *statusError
	if errors.As(err, &status) {
		s.quota.observeDailyHeader(status.Header.Get(dailyQuotaHeader))
		return s.classifyStatus(status)
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

// classifyStatus maps a non-2xx provider response onto the sentinels.
func (s *Sender) classifyStatus(status *statusError) error {
	switch {
	case status.Status == http.StatusTooManyRequests:
		return s.classifyTooManyRequests(status)
	case status.Status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: provider status %d", email.ErrTransient, status.Status)
	default:
		return fmt.Errorf("%w: provider status %d", email.ErrPermanent, status.Status)
	}
}

// classifyTooManyRequests separates a spent quota from a rate limit. A quota
// closes the gate until it resets (midnight UTC for daily, the first of the next
// month for monthly); the gate is what keeps a dead day from costing one
// request per retry tier.
func (s *Sender) classifyTooManyRequests(status *statusError) error {
	now := s.quota.now()
	switch providerQuotaName(status.Body) {
	case quotaNameDaily:
		s.quota.exhaust(nextUTCMidnight(now))
		return fmt.Errorf("%w: %w: provider status %d (%s)", email.ErrQuotaExhausted, email.ErrTransient, status.Status, quotaNameDaily)
	case quotaNameMonthly:
		s.quota.exhaust(nextUTCMonth(now))
		return fmt.Errorf("%w: %w: provider status %d (%s)", email.ErrQuotaExhausted, email.ErrTransient, status.Status, quotaNameMonthly)
	default:
		return fmt.Errorf("%w: %w: provider status %d", email.ErrRateLimited, email.ErrTransient, status.Status)
	}
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
