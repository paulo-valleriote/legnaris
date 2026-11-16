package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrMigrationBlocked = errors.New("email canonicalization blocked")

type EmailIdentity struct {
	ID    string
	Email string
}

type EmailChange struct {
	ID   string
	From string
	To   string
}

type EmailCollision struct {
	Canonical string
	IDs       []string
}

type InvalidEmailRow struct {
	ID     string
	Raw    string
	Reason string
}

type EmailMigrationPlan struct {
	Changes    []EmailChange
	Collisions []EmailCollision
	Invalid    []InvalidEmailRow
}

func (p EmailMigrationPlan) Blocked() bool {
	return len(p.Collisions) > 0 || len(p.Invalid) > 0
}

type EmailMigrationStore interface {
	ListEmailIdentities(ctx context.Context) ([]EmailIdentity, error)
	UpdateEmail(ctx context.Context, id string, email string, updatedAt time.Time) error
}

func PlanEmailCanonicalization(rows []EmailIdentity) EmailMigrationPlan {
	plan := EmailMigrationPlan{}
	idsByCanonical := make(map[string][]string, len(rows))
	canonicalOrder := make([]string, 0, len(rows))

	for _, row := range rows {
		canonical, err := CanonicalEmail(row.Email)
		if err != nil {
			plan.Invalid = append(plan.Invalid, InvalidEmailRow{
				ID:     row.ID,
				Raw:    row.Email,
				Reason: invalidReason(row.Email),
			})
			continue
		}

		if canonical != row.Email {
			plan.Changes = append(plan.Changes, EmailChange{ID: row.ID, From: row.Email, To: canonical})
		}

		if _, seen := idsByCanonical[canonical]; !seen {
			canonicalOrder = append(canonicalOrder, canonical)
		}
		idsByCanonical[canonical] = append(idsByCanonical[canonical], row.ID)
	}

	for _, canonical := range canonicalOrder {
		if ids := idsByCanonical[canonical]; len(ids) > 1 {
			plan.Collisions = append(plan.Collisions, EmailCollision{Canonical: canonical, IDs: ids})
		}
	}

	return plan
}

func (p EmailMigrationPlan) Apply(ctx context.Context, store EmailMigrationStore, now time.Time, dryRun bool) (int, error) {
	if p.Blocked() {
		return 0, ErrMigrationBlocked
	}
	if dryRun {
		return 0, nil
	}

	written := 0
	for _, change := range p.Changes {
		if err := store.UpdateEmail(ctx, change.ID, change.To, now); err != nil {
			return written, fmt.Errorf("update email for %s: %w", change.ID, err)
		}
		written++
	}
	return written, nil
}

func invalidReason(raw string) string {
	if len(strings.TrimSpace(raw)) > MaxEmailLength {
		return "address exceeds 254 bytes"
	}
	return "address cannot be parsed"
}
