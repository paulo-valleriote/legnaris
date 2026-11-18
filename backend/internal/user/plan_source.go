package user

import (
	"context"
	"fmt"

	"legnaris/backend/internal/plan"
)

type PlanDeriver func(ctx context.Context, userID string) (string, error)

type PlanSource struct {
	repository Repository
	deriver    PlanDeriver
}

func NewPlanSource(repository Repository) PlanSource {
	return PlanSource{repository: repository}
}

func (p PlanSource) WithDeriver(deriver PlanDeriver) PlanSource {
	p.deriver = deriver
	return p
}

func (p PlanSource) PlanOf(ctx context.Context, userID string) (plan.Plan, error) {
	stored, err := p.repository.FetchByID(ctx, userID)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("fetch plan owner: %w", err)
	}
	id := stored.Plan
	if p.deriver != nil {
		derived, err := p.deriver(ctx, userID)
		if err != nil {
			return plan.Plan{}, fmt.Errorf("derive plan: %w", err)
		}
		id = derived
	}
	return plan.Lookup(id), nil
}
