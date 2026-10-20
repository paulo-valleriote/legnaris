package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrMigrationBlocked reports a plan that must not write: at least one row is
// unparseable or two rows would share one canonical address. The whole set is
// validated before the first write (PRQ-11).
var ErrMigrationBlocked = errors.New("email canonicalization blocked")

// EmailIdentity is one stored row as read from the users collection: only the
// identifier and the stored address, never the credential material.
type EmailIdentity struct {
	ID    string
	Email string
}

// EmailChange names a row whose canonical form differs from its stored value.
type EmailChange struct {
	ID   string
	From string
	To   string
}

// EmailCollision groups the rows that would claim one canonical address. The
// migration never merges accounts, so such a set blocks the run (PRQ-11).
type EmailCollision struct {
	Canonical string
	IDs       []string
}

// InvalidEmailRow names a stored address the canonical rule rejects, with an
// English reason an operator can act on.
type InvalidEmailRow struct {
	ID     string
	Raw    string
	Reason string
}

// EmailMigrationPlan is the validated intent of one run: the rewrites it would
// perform and the findings that forbid writing anything.
type EmailMigrationPlan struct {
	Changes    []EmailChange
	Collisions []EmailCollision
	Invalid    []InvalidEmailRow
}

// Blocked reports whether the plan must refuse to write.
func (p EmailMigrationPlan) Blocked() bool {
	return len(p.Collisions) > 0 || len(p.Invalid) > 0
}

// EmailMigrationStore is the narrow view of the store the operator command
// needs, so the API repository contract does not grow migration methods.
type EmailMigrationStore interface {
	ListEmailIdentities(ctx context.Context) ([]EmailIdentity, error)
	UpdateEmail(ctx context.Context, id string, email string, updatedAt time.Time) error
}

// PlanEmailCanonicalization applies the creation/sign-in rule to every stored
// row and reports what a repair would do. It is pure: the caller reads the
// store once and only Apply writes (PRQ-11, SC-008).
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

// Apply writes the plan's changes, setting only the address and updated_at for
// each row, and returns how many rows it wrote. A blocked plan writes nothing.
// A dry run reports the same intent without touching the store. Each row is an
// independent write, so a mid-run failure leaves a prefix that a re-run
// finishes safely (PRQ-11).
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

// invalidReason distinguishes the two ways the rule rejects an address, so the
// operator report explains the fix instead of repeating the rule's name.
func invalidReason(raw string) string {
	if len(strings.TrimSpace(raw)) > MaxEmailLength {
		return "address exceeds 254 bytes"
	}
	return "address cannot be parsed"
}
