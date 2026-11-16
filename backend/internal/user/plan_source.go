package user

import (
	"context"
	"fmt"

	"legnaris/backend/internal/plan"
)

type PlanSource struct {
	repository Repository
}

func NewPlanSource(repository Repository) PlanSource {
	return PlanSource{repository: repository}
}

func (p PlanSource) PlanOf(ctx context.Context, userID string) (plan.Plan, error) {
	stored, err := p.repository.FetchByID(ctx, userID)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("fetch plan owner: %w", err)
	}
	return plan.Lookup(stored.Plan), nil
}
