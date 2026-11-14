package worker

import (
	"errors"
	"fmt"
	"testing"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/platform/breaker"

	"github.com/stretchr/testify/assert"
)

func TestClassifyOutcome(t *testing.T) {
	transient := fmt.Errorf("%w: 503", email.ErrTransient)
	tests := []struct {
		name string
		err  error
		want breaker.Outcome
	}{
		{name: "Happy path: a delivered send is a success", err: nil, want: breaker.OutcomeSuccess},
		{name: "Happy path: a transient 5xx or timeout is an outage", err: transient, want: breaker.OutcomeOutage},
		{
			name: "Edge: a daily or monthly quota is never an outage",
			err:  fmt.Errorf("%w: %w", email.ErrQuotaExhausted, email.ErrTransient),
			want: breaker.OutcomeNeutral,
		},
		{
			name: "Edge: a per-second rate limit is never an outage",
			err:  fmt.Errorf("%w: %w", email.ErrRateLimited, email.ErrTransient),
			want: breaker.OutcomeNeutral,
		},
		{
			name: "Edge: a permanent validation failure is neutral",
			err:  fmt.Errorf("%w: 422", email.ErrPermanent),
			want: breaker.OutcomeNeutral,
		},
		{
			name: "Edge: an unclassified error is neutral, not an outage",
			err:  errors.New("unclassified"),
			want: breaker.OutcomeNeutral,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyOutcome(tc.err))
		})
	}
}
