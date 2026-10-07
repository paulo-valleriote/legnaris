package sheet

import (
	"context"
	"errors"
	"legnaris/internal/sheet/template"
	"legnaris/shared/apperr"
	"legnaris/shared/idgen"
	"legnaris/shared/maputils"
	"time"
)

type CreateSheetCommand struct {
	TemplateID string `json:"template_id"`
	SheetName  string `json:"sheet_name"`
	OwnerID    string `json:"-"`
}

type PartialUpdateSheetCommand struct {
	SheetID string         `json:"-"`
	OwnerID string         `json:"-"`
	Data    map[string]any `json:"data"`
}

// PreviewTemplateCommand carries the template dry-run input. TemplateID is
// filled from the path param (json:"-" keeps a body template_id out), so the
// body is the same {"data": <delta>} shape the sheet endpoints take.
type PreviewTemplateCommand struct {
	TemplateID string         `json:"-"`
	Data       map[string]any `json:"data"`
}

type Service interface {
	Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error)
	FetchMySheet(ctx context.Context, ownerID string) (*Sheet, error)
	FetchByID(ctx context.Context, id string, ownerID string) (*Sheet, error)
	PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
	Preview(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
	PreviewTemplate(ctx context.Context, cmd PreviewTemplateCommand) (map[string]any, error)
}

// User-facing messages for the errors this service builds itself. They are
// plain strings, never error causes — causes stay apperr-only per AGENTS.md.
// They stay exported so any boundary that needs the wording reads it from
// here, while the checks themselves have a single source in this package:
// controllers forward input to the service instead of repeating the
// validation (LGN-66).
const (
	MsgSheetIDNotProvided = "sheet id not provided"
	MsgSheetIDInvalid     = "sheet id must be a canonical UUID v7"
	MsgFieldDoesNotExist  = "field does not exist: "
	MsgNestedTooDeep      = "sheet data is nested too deeply"
	MsgOwnerNotProvided   = "authenticated user is required"
	MsgSheetLimitReached  = "user already owns a sheet"
)

// requireSheetID is the single source of truth for the sheet-ID checks
// (LGN-86): the missing-ID message stays byte-for-byte for existing callers,
// then the idgen contract rejects anything that is not a canonical UUID v7 —
// so junk dies with a 400 here instead of becoming a 404 repository lookup,
// no matter which handler or caller reached the service.
func requireSheetID(id string) error {
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDNotProvided, nil)
	}
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDInvalid, err)
	}
	return nil
}

// requireOwner rejects a request whose context carried no authenticated user
// id (OWN-19). The JWT middleware guarantees it on protected routes; failing
// closed here means a route mounted without the middleware can never act
// ownerless instead of silently operating on someone else's data.
func requireOwner(ownerID string) error {
	if ownerID == "" {
		return apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}
	return nil
}

// checkOwnership returns the same 404 a missing sheet produces when the stored
// sheet belongs to nobody (legacy row) or to a different user, so a foreign
// sheet is indistinguishable from a nonexistent one and leaks no existence
// (OWN-11, OWN-12, OWN-24).
func checkOwnership(stored *Sheet, ownerID string) error {
	if stored.OwnerID == "" || stored.OwnerID != ownerID {
		return translateRepoErr("fetch", ErrSheetNotFound)
	}
	return nil
}

// DAGProvider resolves the compiled DAG for a template ID. It is satisfied by
// *template.Registry, which is built once at boot and never mutated afterwards:
// the sheet service reads DAGs through that read-only view instead of holding a
// map it could mutate or that could mutate underneath it (LGN-54).
type DAGProvider interface {
	DAG(id string) (*template.DAG, bool)
}

type ServiceImpl struct {
	repository      Repository
	templateService template.Service
	provider        DAGProvider
}

// NewService builds the sheet service over a DAG provider. The provider must
// already be complete when the service starts (boot-time registry): neither the
// service nor its callers mutate it afterwards.
func NewService(repository Repository, templateService template.Service, provider DAGProvider) Service {
	return &ServiceImpl{
		repository:      repository,
		templateService: templateService,
		provider:        provider,
	}
}

func (s *ServiceImpl) Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error) {
	// Fail closed before any repository access: a route mounted without the JWT
	// middleware must never create an ownerless sheet (OWN-19).
	if cmd.OwnerID == "" {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}

	// FetchTemplate before the DAG lookup: it owns the template-ID checks
	// (missing, malformed slug), so POST /sheets fails exactly like
	// GET /templates/:id instead of collapsing every bad ID into a 404 (LGN-86).
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: cmd.TemplateID})
	if err != nil {
		return nil, translateTemplateErr(err)
	}

	dag, exists := s.provider.DAG(cmd.TemplateID)
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	// Best-effort pre-check for a friendly 409 on the common path. The unique
	// sheets.owner_id index remains the authoritative guard: only the insert
	// closes the concurrent-create race (OWN-03, OWN-04).
	if _, err := s.repository.FetchByOwnerID(ctx, cmd.OwnerID); err == nil {
		return nil, apperr.NewError(apperr.CodeSheetLimitReached, MsgSheetLimitReached, nil)
	} else if !errors.Is(err, ErrSheetNotFound) {
		return nil, translateRepoErr("create", err)
	}

	initialData := buildNestedAllFields(tmpl.Fields)

	// Flatten the initial data into dot-notation keys so the DAG can match
	// leaf keys during recalc (the DAG walks leaf keys, not top-level groups).
	flatInitial, err := maputils.FlattenPathKeys(initialData)
	if err != nil {
		return nil, translateMapErr("create", err)
	}

	derivedEffects, err := dag.Recalculate(map[string]any{}, flatInitial)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	// Apply the flat derived effects back as a nested map (same strategy as
	// PartialUpdate), instead of maps.Copy which would leak flat keys.
	if err := maputils.ApplyUpdates(initialData, derivedEffects); err != nil {
		return nil, translateMapErr("create", err)
	}

	sheetID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, "failed to generate sheet id", err)
	}

	newSheet := Sheet{
		ID:         sheetID,
		TemplateID: cmd.TemplateID,
		OwnerID:    cmd.OwnerID,
		Name:       cmd.SheetName,
		Data:       initialData,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repository.Create(ctx, &newSheet); err != nil {
		return nil, translateRepoErr("create", err)
	}
	return &newSheet, nil
}

// FetchMySheet returns the single sheet owned by ownerID, or 404 NOT_FOUND when
// the owner has none (OWN-07, OWN-08).
func (s *ServiceImpl) FetchMySheet(ctx context.Context, ownerID string) (*Sheet, error) {
	if err := requireOwner(ownerID); err != nil {
		return nil, err
	}

	sheet, err := s.repository.FetchByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}

	return sheet, nil
}

func (s *ServiceImpl) FetchByID(ctx context.Context, id string, ownerID string) (*Sheet, error) {
	if err := requireOwner(ownerID); err != nil {
		return nil, err
	}
	if err := requireSheetID(id); err != nil {
		return nil, err
	}

	sheet, err := s.repository.FetchByID(ctx, id)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}

	if err := checkOwnership(sheet, ownerID); err != nil {
		return nil, err
	}

	return sheet, nil
}

// recalculation is the result of evaluating a delta against a stored sheet
// without persisting it: the merged data a write would store and the flat
// cascading effects the DAG recomputed. Persisting callers use merged; previews
// use cascade only.
type recalculation struct {
	merged  map[string]any
	cascade map[string]any
}

// recalculate is the one recalc path PATCH and both previews share: it
// requires the sheet ID, fetches the sheet, resolves the DAG, flattens the
// delta, normalizes and recalculates it, then deep-merges the result over the
// stored data. It performs no repository write, so a preview can call it
// without side effects.
func (s *ServiceImpl) recalculate(ctx context.Context, sheetID string, ownerID string, data map[string]any) (*recalculation, error) {
	if err := requireOwner(ownerID); err != nil {
		return nil, err
	}
	if err := requireSheetID(sheetID); err != nil {
		return nil, err
	}

	storedSheet, err := s.repository.FetchByID(ctx, sheetID)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}

	if err := checkOwnership(storedSheet, ownerID); err != nil {
		return nil, err
	}

	dag, exists := s.provider.DAG(storedSheet.TemplateID)
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "DAG not found for template", nil)
	}

	// The API contract is a nested map ({"data":{"stats":{"dexterity":12}}}).
	// Flatten it to flat path keys because the DAG engine operates
	// exclusively on those. A payload nested deeper than
	// maputils.MaxNestingDepth is rejected here, before the recursive walk can
	// exhaust the stack.
	updates, err := maputils.FlattenPathKeys(data)
	if err != nil {
		return nil, translateMapErr("update", err)
	}

	// Validate editability, collapse each touched exclusive group to at most
	// one non-zero member, and recompute derived fields under a single DAG
	// read-lock acquisition, so a request never takes the lock more than once.
	normalized, cascadingEffects, err := dag.NormalizeAndRecalculate(storedSheet.Data, updates)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	mergedData := maputils.DeepCopy(storedSheet.Data)
	if err := maputils.ApplyUpdates(mergedData, normalized); err != nil {
		return nil, translateMapErr("update", err)
	}
	if err := maputils.ApplyUpdates(mergedData, cascadingEffects); err != nil {
		return nil, translateMapErr("update", err)
	}

	return &recalculation{merged: mergedData, cascade: cascadingEffects}, nil
}

func (s *ServiceImpl) PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	result, err := s.recalculate(ctx, cmd.SheetID, cmd.OwnerID, cmd.Data)
	if err != nil {
		return nil, err
	}

	dbUpdates := map[string]any{
		"data":       result.merged,
		"updated_at": time.Now(),
	}

	if err := s.repository.PartialUpdate(ctx, cmd.SheetID, dbUpdates); err != nil {
		return nil, translateRepoErr("update", err)
	}
	return nestedCascadingEffects(result.cascade)
}

// Preview runs the same recalculation as PartialUpdate and returns the
// cascading effects, persisting nothing: it shares recalculate, so the cascade
// matches an equivalent PATCH while the stored sheet and updated_at stay
// untouched. Repeated calls against an unchanged sheet return the same map.
func (s *ServiceImpl) Preview(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	result, err := s.recalculate(ctx, cmd.SheetID, cmd.OwnerID, cmd.Data)
	if err != nil {
		return nil, err
	}
	return nestedCascadingEffects(result.cascade)
}

// PreviewTemplate dry-runs a delta against a template with no stored sheet. It
// seeds the template exactly like Create (every leaf at its SeedValue, derived
// leaves then filled by the DAG), applies the delta through the same
// NormalizeAndRecalculate the PATCH path uses and returns the nested cascade.
// Nothing is persisted: there is no sheet to write.
func (s *ServiceImpl) PreviewTemplate(ctx context.Context, cmd PreviewTemplateCommand) (map[string]any, error) {
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: cmd.TemplateID})
	if err != nil {
		return nil, translateTemplateErr(err)
	}

	dag, exists := s.provider.DAG(cmd.TemplateID)
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	seeded := buildNestedAllFields(tmpl.Fields)

	flatSeed, err := maputils.FlattenPathKeys(seeded)
	if err != nil {
		return nil, translateMapErr("create", err)
	}

	derivedEffects, err := dag.Recalculate(map[string]any{}, flatSeed)
	if err != nil {
		return nil, translateDAGErr(err)
	}
	if err := maputils.ApplyUpdates(seeded, derivedEffects); err != nil {
		return nil, translateMapErr("create", err)
	}

	updates, err := maputils.FlattenPathKeys(cmd.Data)
	if err != nil {
		return nil, translateMapErr("update", err)
	}

	_, cascading, err := dag.NormalizeAndRecalculate(seeded, updates)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	return nestedCascadingEffects(cascading)
}

// translateTemplateErr maps template-layer typed errors to apperr at the
// service boundary. The template layer must stay free of apperr imports for
// its own sentinels; the /templates controller runs the same translation
// through templateError.
func translateTemplateErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, template.ErrTemplateIDNotProvided) || errors.Is(err, template.ErrTemplateIDInvalid) {
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to fetch template", err)
}

// translateRepoErr maps repository errors to typed apperr at the service
// boundary. The repository layer must stay free of apperr imports.
func translateRepoErr(op string, err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, ErrSheetNotFound) {
		return apperr.NewError(apperr.CodeNotFound, "sheet not found", err)
	}
	if errors.Is(err, ErrSheetOwnerExists) {
		return apperr.NewError(apperr.CodeSheetLimitReached, MsgSheetLimitReached, err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to "+op+" sheet", err)
}

// translateDAGErr maps template DAG errors to typed apperr at the service
// boundary. The template/DAG layer must stay free of apperr imports.
func translateDAGErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	var unknownField *template.UnknownFieldError
	if errors.As(err, &unknownField) {
		return apperr.NewError(apperr.CodeUnknownField, MsgFieldDoesNotExist+unknownField.Key, nil)
	}
	var notEditable *template.NotEditableError
	if errors.As(err, &notEditable) {
		return apperr.NewError(apperr.CodeInvalidRequest, notEditable.Error(), nil)
	}
	var conflict *template.ExclusiveGroupConflictError
	if errors.As(err, &conflict) {
		return apperr.NewError(
			apperr.CodeInvalidRequest,
			"conflicting exclusive options: "+conflict.Group,
			err,
		)
	}
	var invalidExclusive *template.ExclusiveValueTypeError
	if errors.As(err, &invalidExclusive) {
		return apperr.NewError(
			apperr.CodeInvalidRequest,
			"exclusive option must be numeric: "+invalidExclusive.Key,
			err,
		)
	}
	if errors.Is(err, maputils.ErrMaxDepthExceeded) {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgNestedTooDeep, err)
	}
	switch {
	case isMaputilsDataErr(err):
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid sheet data", err)
	case errors.Is(err, template.ErrFormulaEval):
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	case errors.Is(err, template.ErrNilOriginalData), errors.Is(err, template.ErrNilUpdatedData):
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	case errors.Is(err, template.ErrInfiniteCycle):
		return apperr.NewError(apperr.CodeCycleDetected, err.Error(), nil)
	}
	return apperr.NewError(apperr.CodeInternal, "dag recalculation failed", err)
}

// translateMapErr maps shared/maputils errors to typed apperr at the service
// boundary. The maputils layer must stay free of apperr imports.
func translateMapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, maputils.ErrMaxDepthExceeded) {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgNestedTooDeep, err)
	}
	if isMaputilsDataErr(err) {
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid sheet data", err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to "+op+" sheet", err)
}

// isMaputilsDataErr reports whether err is a typed maputils nested-data error
// (empty path, nil map or non-map intermediate value).
func isMaputilsDataErr(err error) bool {
	return errors.Is(err, maputils.ErrEmptyPath) ||
		errors.Is(err, maputils.ErrNilMap) ||
		errors.Is(err, maputils.ErrNotMap)
}

// nestedCascadingEffects converts the flat path-key cascading effects
// returned by DAG.Recalculate back into a nested map so the API response
// mirrors the nested request payload.
func nestedCascadingEffects(flat map[string]any) (map[string]any, error) {
	nested := make(map[string]any)
	if err := maputils.ApplyUpdates(nested, flat); err != nil {
		return nil, translateMapErr("update", err)
	}
	return nested, nil
}

// buildNestedAllFields creates nested map with ALL leaf fields (base + derived)
// seeded through FieldSchema.SeedValue: numbers start at their default, text
// starts empty; derived leaves are overwritten by the Recalculate call that
// follows.
func buildNestedAllFields(group template.FieldGroup) map[string]any {
	result := make(map[string]any)
	for name, node := range group {
		if node.IsLeaf() {
			result[name] = node.Schema.SeedValue()
		} else if node.IsGroup() {
			nested := buildNestedAllFields(node.Children)
			if len(nested) > 0 {
				result[name] = nested
			}
		}
	}
	return result
}
