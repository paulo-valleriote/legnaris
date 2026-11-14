package sheet

import (
	"context"
	"time"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/maputils"
)

// detailTemplateIDKey is the event payload key that names the Template of a
// created Sheet.
const detailTemplateIDKey = "template_id"

// CreateWithValuesCommand creates a Sheet from values the caller supplies, such
// as an accepted generation preview. Values are nested like the PATCH data and
// are validated through the Template's DAG before anything is written.
type CreateWithValuesCommand struct {
	TemplateID string         `json:"template_id"`
	Kind       string         `json:"kind"`
	SheetName  string         `json:"sheet_name"`
	OwnerID    string         `json:"-"`
	Values     map[string]any `json:"values"`
}

// AfterInsertHook runs inside the transaction that inserts the Sheet, after the
// insert and before commit. An error it returns aborts the Sheet, the slot
// reservation and the hook's own writes together.
type AfterInsertHook func(txCtx context.Context, created *Sheet) error

// creationBase is what a new Sheet needs before any slot is reserved: the kind
// it counts under, the Template, its compiled DAG and the owner's allowance.
type creationBase struct {
	kind  plan.SheetKind
	tmpl  *template.Template
	dag   *template.DAG
	limit int
}

// resolveNewSheet runs the checks every creation shares, in the order the
// creation paths report them: the owner, the kind, the Template and its kind,
// the DAG, the plan limits and the owner's allowance for the kind. It reads only.
func (s *ServiceImpl) resolveNewSheet(ctx context.Context, ownerID string, kind plan.SheetKind, templateID string) (creationBase, error) {
	// FetchTemplate before the DAG lookup: it owns the template-ID checks
	// (missing, malformed slug), so POST /sheets fails exactly like
	// GET /templates/:id instead of collapsing every bad ID into a 404 (LGN-86).
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: templateID})
	if err != nil {
		return creationBase{}, translateTemplateErr(err)
	}
	if !tmpl.AcceptsKind(kind) {
		return creationBase{}, apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateKindRefused, nil)
	}

	dag, exists := s.provider.DAG(templateID)
	if !exists {
		return creationBase{}, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	if !s.planLimitsEnabled() {
		return creationBase{}, apperr.NewError(apperr.CodeInternal, MsgLimitsNotConfigured, nil)
	}
	current, err := s.plans.PlanOf(ctx, ownerID)
	if err != nil {
		return creationBase{}, apperr.NewError(apperr.CodeInternal, "failed to resolve plan", err)
	}
	return creationBase{kind: kind, tmpl: tmpl, dag: dag, limit: current.SheetLimit(kind)}, nil
}

// seedInitialData builds the nested default data of a new Sheet and applies the
// derived defaults the DAG computes from it.
func (s *ServiceImpl) seedInitialData(base creationBase) (map[string]any, error) {
	initialData := buildNestedAllFields(base.tmpl.Fields)

	// Flatten the initial data into dot-notation keys so the DAG can match
	// leaf keys during recalc (the DAG walks leaf keys, not top-level groups).
	flatInitial, err := maputils.FlattenPathKeys(initialData)
	if err != nil {
		return nil, translateMapErr("create", err)
	}

	derivedEffects, err := base.dag.Recalculate(map[string]any{}, flatInitial)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	// Apply the flat derived effects back as a nested map (same strategy as
	// PartialUpdate), instead of maps.Copy which would leak flat keys.
	if err := maputils.ApplyUpdates(initialData, derivedEffects); err != nil {
		return nil, translateMapErr("create", err)
	}
	return initialData, nil
}

// CreateWithValues creates a Sheet whose data is the seeded defaults overlaid
// with cmd.Values after the same normalization and recalculation a PATCH runs.
// The slot reservation, the insert and afterInsert share one transaction, so a
// refused slot writes nothing and a failing hook leaves no Sheet behind.
func (s *ServiceImpl) CreateWithValues(ctx context.Context, cmd CreateWithValuesCommand, afterInsert AfterInsertHook) (*Sheet, error) {
	if cmd.OwnerID == "" {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}
	kind, err := requireKind(cmd.Kind)
	if err != nil {
		return nil, err
	}

	base, err := s.resolveNewSheet(ctx, cmd.OwnerID, kind, cmd.TemplateID)
	if err != nil {
		return nil, err
	}
	initialData, err := s.seedInitialData(base)
	if err != nil {
		return nil, err
	}

	data, err := s.applyValues(base.dag, initialData, cmd.Values)
	if err != nil {
		return nil, err
	}

	sheetID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, "failed to generate sheet id", err)
	}
	now := time.Now()
	newSheet := Sheet{
		ID:         sheetID,
		TemplateID: cmd.TemplateID,
		OwnerID:    cmd.OwnerID,
		Kind:       kind,
		Name:       cmd.SheetName,
		Data:       data,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.ReserveSlot(txCtx, cmd.OwnerID, kind, base.limit); err != nil {
			return err
		}
		if err := s.repository.Create(txCtx, &newSheet); err != nil {
			return err
		}
		if afterInsert == nil {
			return nil
		}
		return afterInsert(txCtx, &newSheet)
	})
	if err != nil {
		return nil, translateCreateErr(err, kind, base.limit)
	}
	s.publishSheetEvent(eventSheetCreated, cmd.OwnerID, sheetID, newSheet.Version, map[string]any{detailTemplateIDKey: cmd.TemplateID})
	return &newSheet, nil
}

// applyValues validates values against the seeded data through the same
// normalization a PATCH uses, then merges the normalized values and the cascade
// they trigger over a copy of the seed. An empty value set keeps the defaults.
func (s *ServiceImpl) applyValues(dag *template.DAG, seed map[string]any, values map[string]any) (map[string]any, error) {
	updates, err := maputils.FlattenPathKeys(values)
	if err != nil {
		return nil, translateMapErr("create", err)
	}
	normalized, cascade, err := dag.NormalizeAndRecalculate(seed, updates)
	if err != nil {
		return nil, translateDAGErr(err)
	}
	merged := maputils.DeepCopy(seed)
	if err := maputils.ApplyUpdates(merged, normalized); err != nil {
		return nil, translateMapErr("create", err)
	}
	if err := maputils.ApplyUpdates(merged, cascade); err != nil {
		return nil, translateMapErr("create", err)
	}
	return merged, nil
}
