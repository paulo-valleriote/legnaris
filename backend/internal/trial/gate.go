// Package trial decides whether a newly registered account receives the Free
// generation allowance (ADR 0009, LGN-229). The allowance itself is the
// generation_trial row owned by the quota service (LGN-182); this package only
// gates who may be granted it.
package trial

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/apperr"
)

// MaxAccountsPerIP is how many accounts from one client IP may receive the
// allowance inside IPWindow. Accounts over the cap are still created.
const MaxAccountsPerIP = 3

// IPWindow is the sliding window in which MaxAccountsPerIP is counted.
const IPWindow = 24 * time.Hour

// Granter creates the generation_trial row that carries the allowance
// (shape defined in LGN-182). Implementations must be idempotent and must not
// reset an existing row's used count, so a repeated grant is harmless.
type Granter interface {
	Grant(ctx context.Context, userID string) error
}

// DomainChecker reports whether an address belongs to a disposable provider.
// It returns an error only when the check itself could not run; the gate then
// fails closed and grants nothing.
type DomainChecker interface {
	IsDisposable(ctx context.Context, email string) (bool, error)
}

// Decision is the outcome of Gate.Evaluate. Every non-Granted decision means
// the account keeps 0 allowance, and none of them is an error.
type Decision string

const (
	// DecisionGranted means the allowance row was created.
	DecisionGranted Decision = "granted"
	// DecisionNotVerified means the email has not passed the OTP check yet.
	DecisionNotVerified Decision = "not_verified"
	// DecisionInvalidIP means the client IP is missing or unusable as a key.
	DecisionInvalidIP Decision = "invalid_ip"
	// DecisionDisposable means the email domain is disposable.
	DecisionDisposable Decision = "disposable"
	// DecisionIPLimitReached means the IP already received MaxAccountsPerIP
	// allowances inside IPWindow.
	DecisionIPLimitReached Decision = "ip_limit_reached"
)

// Candidate is one freshly registered account being evaluated for the allowance.
type Candidate struct {
	UserID        string
	Email         string
	EmailVerified bool
	ClientIP      string
}

// Gate applies the trial eligibility rules in a fixed order. The cheap checks
// come first, and the per-IP slot is consumed last, so a refused candidate
// never uses up a slot another account needs.
type Gate struct {
	granter Granter
	domains DomainChecker
	ips     ratelimit.Allower
}

// NewGate wires the gate. The ips allower must be configured with
// MaxAccountsPerIP and IPWindow; the gate does not enforce those values itself.
func NewGate(granter Granter, domains DomainChecker, ips ratelimit.Allower) *Gate {
	return &Gate{granter: granter, domains: domains, ips: ips}
}

// Evaluate decides the allowance for c and, when eligible, grants it. It
// returns apperr.CodeInvalidRequest for an empty user id, and a plain wrapped
// error when a port fails. In both cases no allowance is granted.
func (g *Gate) Evaluate(ctx context.Context, c Candidate) (Decision, error) {
	if c.UserID == "" {
		return "", apperr.NewError(apperr.CodeInvalidRequest, "user id is required", nil)
	}
	if !c.EmailVerified {
		return DecisionNotVerified, nil
	}
	if _, err := netip.ParseAddr(c.ClientIP); err != nil {
		return DecisionInvalidIP, nil
	}

	disposable, err := g.domains.IsDisposable(ctx, c.Email)
	if err != nil {
		return "", fmt.Errorf("check disposable domain: %w", err)
	}
	if disposable {
		return DecisionDisposable, nil
	}

	if admitted, _ := g.ips.Allow(c.ClientIP, g.ips.Now()); !admitted {
		return DecisionIPLimitReached, nil
	}

	if err := g.granter.Grant(ctx, c.UserID); err != nil {
		return "", fmt.Errorf("grant generation trial: %w", err)
	}
	return DecisionGranted, nil
}
