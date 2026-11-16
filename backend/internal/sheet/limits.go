package sheet

import (
	"context"
	"errors"

	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
)

var ErrSheetLimitReached = errors.New("sheet limit reached")

type PlanSource interface {
	PlanOf(ctx context.Context, ownerID string) (plan.Plan, error)
}

type TransactionRunner interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type ServiceOption func(*ServiceImpl)

func WithPlanLimits(plans PlanSource, tx TransactionRunner) ServiceOption {
	return func(s *ServiceImpl) {
		s.plans = plans
		s.tx = tx
	}
}

type LimitStatus struct {
	Kind  plan.SheetKind
	Used  int
	Limit int
}

func (s LimitStatus) Reached() bool {
	return s.Used >= s.Limit
}

type PlanUsage struct {
	PlanID string
	Kinds  []LimitStatus
}

const (
	MsgKindInvalid         = "sheet kind must be one of character, npc, monster"
	MsgTemplateKindRefused = "template does not accept this sheet kind"
	MsgLimitsNotConfigured = "sheet limits are not configured"
)

func requireKind(raw string) (plan.SheetKind, error) {
	kind, ok := plan.ParseSheetKind(raw)
	if !ok {
		return "", apperr.NewError(apperr.CodeInvalidRequest, MsgKindInvalid, nil)
	}
	return kind, nil
}

func sheetLimitError(kind plan.SheetKind, limit int, cause error) *apperr.Error {
	return apperr.NewError(apperr.CodeSheetLimitReached, MsgSheetLimitReached, cause).
		WithDetails(map[string]any{detailKindKey: string(kind), "limit": limit})
}

func (s *ServiceImpl) planLimitsEnabled() bool {
	return s.plans != nil && s.tx != nil
}

func (s *ServiceImpl) CheckSheetLimit(ctx context.Context, ownerID string, rawKind plan.SheetKind) (LimitStatus, error) {
	if err := requireOwner(ownerID); err != nil {
		return LimitStatus{}, err
	}
	kind, err := requireKind(string(rawKind))
	if err != nil {
		return LimitStatus{}, err
	}
	if !s.planLimitsEnabled() {
		return LimitStatus{}, apperr.NewError(apperr.CodeInternal, MsgLimitsNotConfigured, nil)
	}
	return s.limitStatus(ctx, ownerID, kind)
}

func (s *ServiceImpl) PlanUsage(ctx context.Context, ownerID string) (PlanUsage, error) {
	if err := requireOwner(ownerID); err != nil {
		return PlanUsage{}, err
	}
	if !s.planLimitsEnabled() {
		return PlanUsage{}, apperr.NewError(apperr.CodeInternal, MsgLimitsNotConfigured, nil)
	}
	current, err := s.plans.PlanOf(ctx, ownerID)
	if err != nil {
		return PlanUsage{}, apperr.NewError(apperr.CodeInternal, "failed to resolve plan", err)
	}

	usage := PlanUsage{PlanID: current.ID}
	for _, kind := range plan.SheetKinds() {
		used, err := s.repository.CountOf(ctx, ownerID, kind)
		if err != nil {
			return PlanUsage{}, translateRepoErr("count", err)
		}
		usage.Kinds = append(usage.Kinds, LimitStatus{Kind: kind, Used: used, Limit: current.SheetLimit(kind)})
	}
	return usage, nil
}

func (s *ServiceImpl) limitStatus(ctx context.Context, ownerID string, kind plan.SheetKind) (LimitStatus, error) {
	current, err := s.plans.PlanOf(ctx, ownerID)
	if err != nil {
		return LimitStatus{}, apperr.NewError(apperr.CodeInternal, "failed to resolve plan", err)
	}
	used, err := s.repository.CountOf(ctx, ownerID, kind)
	if err != nil {
		return LimitStatus{}, translateRepoErr("count", err)
	}
	return LimitStatus{Kind: kind, Used: used, Limit: current.SheetLimit(kind)}, nil
}
