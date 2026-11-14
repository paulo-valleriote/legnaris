package sheet

import (
	"context"
	"errors"

	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
)

// ErrSheetLimitReached reports a reservation refused because the owner already
// holds as many Sheets of the kind as the plan allows. The service translates
// it to SHEET_LIMIT_REACHED at the boundary.
var ErrSheetLimitReached = errors.New("sheet limit reached")

// PlanSource resolves the plan an owner is on. It is the only way the sheet
// package learns about plans, so the user store stays behind an interface.
type PlanSource interface {
	PlanOf(ctx context.Context, ownerID string) (plan.Plan, error)
}

// TransactionRunner runs fn inside one database transaction. It is satisfied by
// *db.Transactor; every repository call inside fn must use the context fn
// receives.
type TransactionRunner interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// ServiceOption configures optional collaborators of the sheet service. Limits
// are enforced only when WithPlanLimits is supplied; a service built without it
// refuses every creation rather than create unbounded Sheets.
type ServiceOption func(*ServiceImpl)

// WithPlanLimits wires the plan source that decides the per-kind allowance and
// the transaction runner that makes the counter and the Sheet write atomic.
func WithPlanLimits(plans PlanSource, tx TransactionRunner) ServiceOption {
	return func(s *ServiceImpl) {
		s.plans = plans
		s.tx = tx
	}
}

// LimitStatus is the read-only view of one kind for one owner: how many Sheets
// are held and how many the plan allows.
type LimitStatus struct {
	Kind  plan.SheetKind
	Used  int
	Limit int
}

// Reached reports whether the owner can create no further Sheet of this kind.
func (s LimitStatus) Reached() bool {
	return s.Used >= s.Limit
}

// PlanUsage is an owner's plan and the usage of every kind under it, in the
// stable order plan.SheetKinds returns.
type PlanUsage struct {
	PlanID string
	Kinds  []LimitStatus
}

// Message constants for the errors the limit checks build. They are plain
// strings, reused by controllers if they ever need the wording.
const (
	MsgKindInvalid         = "sheet kind must be one of character, npc, monster"
	MsgTemplateKindRefused = "template does not accept this sheet kind"
	MsgLimitsNotConfigured = "sheet limits are not configured"
)

// requireKind validates the kind a Sheet is created or counted under. A missing
// or unknown kind is a 400, never a silent default.
func requireKind(raw string) (plan.SheetKind, error) {
	kind, ok := plan.ParseSheetKind(raw)
	if !ok {
		return "", apperr.NewError(apperr.CodeInvalidRequest, MsgKindInvalid, nil)
	}
	return kind, nil
}

// sheetLimitError builds the 409 that names the refused kind and its allowance,
// so the client can render "x/y" without a second request.
func sheetLimitError(kind plan.SheetKind, limit int, cause error) *apperr.Error {
	return apperr.NewError(apperr.CodeSheetLimitReached, MsgSheetLimitReached, cause).
		WithDetails(map[string]any{detailKindKey: string(kind), "limit": limit})
}

// planLimitsEnabled reports whether the service was built with WithPlanLimits.
func (s *ServiceImpl) planLimitsEnabled() bool {
	return s.plans != nil && s.tx != nil
}

// CheckSheetLimit reports how many Sheets of kind ownerID holds and what the
// plan allows. It only reads: callers such as the generation API use it to gate
// work before they spend anything, and it never reserves a slot.
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

// PlanUsage reports the owner's plan and the usage of every kind under it. It
// only reads, like CheckSheetLimit.
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

// limitStatus reads the owner's plan and count for one kind.
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
