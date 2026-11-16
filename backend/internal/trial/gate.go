package trial

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/apperr"
)

const MaxAccountsPerIP = 3

const IPWindow = 24 * time.Hour

type Granter interface {
	Grant(ctx context.Context, userID string) error
}

type DomainChecker interface {
	IsDisposable(ctx context.Context, email string) (bool, error)
}

type Decision string

const (
	DecisionGranted        Decision = "granted"
	DecisionNotVerified    Decision = "not_verified"
	DecisionInvalidIP      Decision = "invalid_ip"
	DecisionDisposable     Decision = "disposable"
	DecisionIPLimitReached Decision = "ip_limit_reached"
)

type Candidate struct {
	UserID        string
	Email         string
	EmailVerified bool
	ClientIP      string
}

type Gate struct {
	granter Granter
	domains DomainChecker
	ips     ratelimit.Allower
}

func NewGate(granter Granter, domains DomainChecker, ips ratelimit.Allower) *Gate {
	return &Gate{granter: granter, domains: domains, ips: ips}
}

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
