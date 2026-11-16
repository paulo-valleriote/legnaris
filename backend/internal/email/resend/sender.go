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

const maxRateRetryWait = 5 * time.Second

var ErrMissingConfiguration = errors.New("resend: missing configuration")

type Sender struct {
	client *resend.Client
	from   string
	quota  *quotaGate
}

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

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
