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

const detailTemplateIDKey = "template_id"

type CreateWithValuesCommand struct {
	TemplateID string         `json:"template_id"`
	Kind       string         `json:"kind"`
	SheetName  string         `json:"sheet_name"`
	OwnerID    string         `json:"-"`
	Values     map[string]any `json:"values"`
}

type AfterInsertHook func(txCtx context.Context, created *Sheet) error

type creationBase struct {
	kind  plan.SheetKind
	tmpl  *template.Template
	dag   *template.DAG
	limit int
}

type templateGate func(tmpl *template.Template, draftsAllowed bool) bool

func (s *ServiceImpl) resolveNewSheet(ctx context.Context, ownerID string, kind plan.SheetKind, templateID, campaignID string, allowed templateGate, startFallback bool) (creationBase, error) {
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: templateID})
	if err != nil {
		return creationBase{}, translateTemplateErr(err)
	}
	if startFallback && campaignID == "" && tmpl.State == template.StateRetired {
		fallback, err := s.catalogDefault(ctx, kind)
		if err != nil {
			return creationBase{}, err
		}
		if fallback != nil {
			tmpl = fallback
		}
	}
	if err := s.checkTemplateState(ctx, ownerID, tmpl, campaignID, allowed); err != nil {
		return creationBase{}, err
	}
	if !tmpl.AcceptsKind(kind) {
		return creationBase{}, apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateKindRefused, nil)
	}

	dag, exists := s.provider.DAG(tmpl.ID)
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

func (s *ServiceImpl) catalogDefault(ctx context.Context, kind plan.SheetKind) (*template.Template, error) {
	listed, err := s.templateService.List(ctx)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgCatalogDefaultUnavailable, err)
	}
	for i := range listed {
		if listed[i].Default && listed[i].AcceptsKind(kind) {
			return &listed[i], nil
		}
	}
	return nil, nil
}

func (s *ServiceImpl) checkTemplateState(ctx context.Context, ownerID string, tmpl *template.Template, campaignID string, allowed templateGate) error {
	if allowed(tmpl, s.draftsAllowed) {
		return nil
	}
	if tmpl.State == template.StateRetired {
		if campaignID == "" {
			return apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateRetiredRefused, nil)
		}
		return s.requireCampaignScope(ctx, ownerID, campaignID, tmpl.ID)
	}
	return apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateStateRefused, nil)
}

func (s *ServiceImpl) requireCampaignScope(ctx context.Context, ownerID, campaignID, templateID string) error {
	if s.campaignScope == nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgCampaignScopeRefused, nil)
	}
	if err := idgen.Validate(campaignID); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgCampaignScopeRefused, err)
	}
	allowed, err := s.campaignScope.AllowsTemplate(ctx, ownerID, campaignID, templateID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgCampaignScopeUnavailable, err)
	}
	if !allowed {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgCampaignScopeRefused, nil)
	}
	return nil
}

func (s *ServiceImpl) seedInitialData(base creationBase) (map[string]any, error) {
	initialData := buildNestedAllFields(base.tmpl.Fields)

	flatInitial, err := maputils.FlattenPathKeys(buildNestedDAGFields(base.tmpl.Fields))
	if err != nil {
		return nil, translateMapErr("create", err)
	}

	derivedEffects, err := base.dag.Recalculate(map[string]any{}, flatInitial)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	if err := maputils.ApplyUpdates(initialData, derivedEffects); err != nil {
		return nil, translateMapErr("create", err)
	}
	return initialData, nil
}

func (s *ServiceImpl) CreateWithValues(ctx context.Context, cmd CreateWithValuesCommand, afterInsert AfterInsertHook) (*Sheet, error) {
	if cmd.OwnerID == "" {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}
	kind, err := requireKind(cmd.Kind)
	if err != nil {
		return nil, err
	}

	name, err := ValidateSheetName(cmd.SheetName)
	if err != nil {
		return nil, err
	}

	base, err := s.resolveNewSheet(ctx, cmd.OwnerID, kind, cmd.TemplateID, "", (*template.Template).Available, false)
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
		TemplateID: base.tmpl.ID,
		OwnerID:    cmd.OwnerID,
		Kind:       kind,
		Name:       name,
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
	s.publishSheetEvent(eventSheetCreated, cmd.OwnerID, sheetID, newSheet.Version, map[string]any{detailTemplateIDKey: base.tmpl.ID})
	return &newSheet, nil
}

func (s *ServiceImpl) applyValues(dag *template.DAG, seed map[string]any, values map[string]any) (map[string]any, error) {
	flat, err := maputils.FlattenPathKeys(values)
	if err != nil {
		return nil, translateMapErr("create", err)
	}
	flat = dag.DropDerivedRecordValues(flat)
	staticUpdates, recordUpdates, err := dag.SplitRecordUpdates(flat)
	if err != nil {
		return nil, translateDAGErr(err)
	}
	updates := dropDerived(dag, staticUpdates)
	calculation, err := dag.NormalizeAndRecalculateWithRecords(seed, updates, recordUpdates)
	if err != nil {
		return nil, translateDAGErr(err)
	}
	return calculation.Merged, nil
}

func dropDerived(dag *template.DAG, values map[string]any) map[string]any {
	kept := make(map[string]any, len(values))
	for key, value := range values {
		if node, ok := dag.GetNode(key); ok && !node.IsEditable {
			continue
		}
		kept[key] = value
	}
	return kept
}
