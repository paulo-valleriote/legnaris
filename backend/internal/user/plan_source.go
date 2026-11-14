package user

import (
	"context"
	"fmt"

	"legnaris/backend/internal/plan"
)

// PlanSource resolves the plan of a stored user. It satisfies the sheet
// package's plan lookup without that package reading the user collection.
type PlanSource struct {
	repository Repository
}

// NewPlanSource builds the plan lookup over the user repository.
func NewPlanSource(repository Repository) PlanSource {
	return PlanSource{repository: repository}
}

// PlanOf returns the plan stored for userID. A missing stored plan resolves to
// Free. A lookup failure is returned wrapped, so the caller can tell a missing
// user (ErrUserNotFound) apart from a database error.
func (p PlanSource) PlanOf(ctx context.Context, userID string) (plan.Plan, error) {
	stored, err := p.repository.FetchByID(ctx, userID)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("fetch plan owner: %w", err)
	}
	return plan.Lookup(stored.Plan), nil
}
