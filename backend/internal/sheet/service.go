package sheet

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
	"legnaris/backend/shared/maputils"
)

type CreateSheetCommand struct {
	TemplateID string `json:"template_id"`
	Kind       string `json:"kind"`
	SheetName  string `json:"sheet_name"`
	OwnerID    string `json:"-"`
}

type sheetRef struct {
	id      string
	ownerID string
}

func (r sheetRef) validate() error {
	if err := requireOwner(r.ownerID); err != nil {
		return err
	}
	return requireSheetID(r.id)
}

type PartialUpdateSheetCommand struct {
	SheetID   string         `json:"-"`
	OwnerID   string         `json:"-"`
	RequestID string         `json:"-"`
	Data      map[string]any `json:"data"`
	Dice      *Dice          `json:"dice"`
}

func (c PartialUpdateSheetCommand) ref() sheetRef {
	return sheetRef{id: c.SheetID, ownerID: c.OwnerID}
}

func (c PartialUpdateSheetCommand) hasDelta() bool {
	return len(c.Data) > 0
}

type SetLayoutCommand struct {
	SheetID string  `json:"-"`
	OwnerID string  `json:"-"`
	Layout  *Layout `json:"layout"`
}

type PreviewTemplateCommand struct {
	TemplateID string         `json:"-"`
	Data       map[string]any `json:"data"`
}

type DeleteSheetCommand struct {
	SheetID string `json:"-"`
	OwnerID string `json:"-"`
}

type Service interface {
	Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error)
	FetchMySheet(ctx context.Context, ownerID string) (*Sheet, error)
	ListMine(ctx context.Context, ownerID string, kind plan.SheetKind) ([]Sheet, error)
	FetchByID(ctx context.Context, id string, ownerID string) (*Sheet, error)
	CreateWithValues(ctx context.Context, cmd CreateWithValuesCommand, afterInsert AfterInsertHook) (*Sheet, error)
	PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
	Preview(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
	PreviewTemplate(ctx context.Context, cmd PreviewTemplateCommand) (map[string]any, error)
	SetLayout(ctx context.Context, cmd SetLayoutCommand) (*Layout, error)
	Delete(ctx context.Context, cmd DeleteSheetCommand) error
	Duplicate(ctx context.Context, cmd DuplicateSheetCommand) (*Sheet, error)
	CheckSheetLimit(ctx context.Context, ownerID string, kind plan.SheetKind) (LimitStatus, error)
	PlanUsage(ctx context.Context, ownerID string) (PlanUsage, error)
}

const (
	MsgSheetIDNotProvided = "sheet id not provided"
	MsgSheetKindInvalid   = "kind must be character, npc or enemy"
	MsgSheetIDInvalid     = "sheet id must be a canonical UUID v7"
	MsgFieldDoesNotExist  = "field does not exist: "
	MsgNestedTooDeep      = "sheet data is nested too deeply"
	MsgOwnerNotProvided   = "authenticated user is required"
	MsgSheetLimitReached  = "sheet limit reached for this kind"
	MsgSheetConflict      = "sheet was changed by another request, try again"
	MsgNothingToUpdate    = "sheet update carries neither data nor dice"
)

const conflictRetryAfter = time.Second

func requireSheetID(id string) error {
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDNotProvided, nil)
	}
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDInvalid, err)
	}
	return nil
}

func requireOwner(ownerID string) error {
	if ownerID == "" {
		return apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}
	return nil
}

func checkOwnership(stored *Sheet, ownerID string) error {
	if stored.OwnerID == "" || stored.OwnerID != ownerID {
		return translateRepoErr("fetch", sheeterr.ErrSheetNotFound)
	}
	return nil
}

type DAGProvider interface {
	DAG(id string) (*template.DAG, bool)
}

type ServiceImpl struct {
	repository      Repository
	templateService template.Service
	provider        DAGProvider
	plans           PlanSource
	tx              TransactionRunner
	linkCleaner     LinkCleaner
	log             logger.StructuredLogger
	publisher       events.Publisher
}

func NewService(repository Repository, templateService template.Service, provider DAGProvider, log logger.StructuredLogger, publisher events.Publisher, opts ...ServiceOption) Service {
	if publisher == nil {
		publisher = events.NoOp{}
	}
	svc := &ServiceImpl{
		repository:      repository,
		templateService: templateService,
		provider:        provider,
		log:             log,
		publisher:       publisher,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *ServiceImpl) Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error) {
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
	limit := base.limit

	sheetID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, "failed to generate sheet id", err)
	}

	newSheet := Sheet{
		ID:         sheetID,
		TemplateID: cmd.TemplateID,
		OwnerID:    cmd.OwnerID,
		Kind:       kind,
		Name:       cmd.SheetName,
		Data:       initialData,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.ReserveSlot(txCtx, cmd.OwnerID, kind, limit); err != nil {
			return err
		}
		return s.repository.Create(txCtx, &newSheet)
	})
	if err != nil {
		return nil, translateCreateErr(err, kind, limit)
	}
	s.publishSheetEvent(eventSheetCreated, cmd.OwnerID, sheetID, newSheet.Version, map[string]any{detailTemplateIDKey: cmd.TemplateID})
	return &newSheet, nil
}

func translateCreateErr(err error, kind plan.SheetKind, limit int) error {
	if errors.Is(err, sheeterr.ErrSheetLimitReached) {
		return sheetLimitError(kind, limit, err)
	}
	return translateRepoErr("create", err)
}

func (s *ServiceImpl) ListMine(ctx context.Context, ownerID string, kind plan.SheetKind) ([]Sheet, error) {
	if err := requireOwner(ownerID); err != nil {
		return nil, err
	}

	found, err := s.repository.ListByOwnerKind(ctx, ownerID, kind)
	if err != nil {
		return nil, translateRepoErr("list", err)
	}

	return found, nil
}

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

type preparedSave struct {
	templateID string
	version    int64
	delta      map[string]any
	merged     map[string]any
	cascade    map[string]any
}

func (s *ServiceImpl) recalculate(ctx context.Context, ref sheetRef, data map[string]any) (*preparedSave, error) {
	storedSheet, err := s.fetchOwned(ctx, ref)
	if err != nil {
		return nil, err
	}

	dag, exists := s.provider.DAG(storedSheet.TemplateID)
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "DAG not found for template", nil)
	}

	updates, err := maputils.FlattenPathKeys(data)
	if err != nil {
		return nil, translateMapErr("update", err)
	}

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

	return &preparedSave{
		templateID: storedSheet.TemplateID,
		version:    storedSheet.Version,
		delta:      normalized,
		merged:     mergedData,
		cascade:    cascadingEffects,
	}, nil
}

func (s *ServiceImpl) fetchOwned(ctx context.Context, ref sheetRef) (*Sheet, error) {
	if err := ref.validate(); err != nil {
		return nil, err
	}

	storedSheet, err := s.repository.FetchByID(ctx, ref.id)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}
	if err := checkOwnership(storedSheet, ref.ownerID); err != nil {
		return nil, err
	}
	return storedSheet, nil
}

func (s *ServiceImpl) PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	result, err := s.prepareSave(ctx, cmd)
	if err != nil {
		return nil, err
	}

	dbUpdates := map[string]any{"updated_at": time.Now()}
	if cmd.hasDelta() {
		dbUpdates["data"] = result.merged
	}
	if cmd.Dice != nil {
		block, err := s.validatedDice(ctx, result.templateID, cmd.Dice)
		if err != nil {
			return nil, err
		}
		dbUpdates[diceField] = block
	}

	newVersion, err := s.repository.PartialUpdate(ctx, cmd.SheetID, result.version, dbUpdates)
	if err == nil {
		s.log.Info(ctx, "sheet patched", append(
			patchLogFields(cmd.SheetID, cmd.RequestID, result.templateID, result.version, result.delta, result.cascade),
			logger.Attribute{Key: "dice_replaced", Value: strconv.FormatBool(cmd.Dice != nil)},
			logger.Attribute{Key: "version_after", Value: strconv.FormatInt(newVersion, 10)},
		)...)
		s.publishSheetUpdated(cmd.OwnerID, cmd.SheetID, changedKeysOf(cmd), newVersion)
		return nestedCascadingEffects(result.cascade)
	}
	if !errors.Is(err, sheeterr.ErrVersionConflict) {
		return nil, translateRepoErr("update", err)
	}
	s.log.Warn(ctx, "sheet patch version conflict",
		patchLogFields(cmd.SheetID, cmd.RequestID, result.templateID, result.version, result.delta, result.cascade)...)
	return nil, apperr.NewError(apperr.CodeConflict, MsgSheetConflict, sheeterr.ErrVersionConflict).
		WithRetryAfter(conflictRetryAfter)
}

func (s *ServiceImpl) prepareSave(ctx context.Context, cmd PartialUpdateSheetCommand) (*preparedSave, error) {
	if cmd.hasDelta() {
		return s.recalculate(ctx, cmd.ref(), cmd.Data)
	}
	if err := cmd.ref().validate(); err != nil {
		return nil, err
	}
	if cmd.Dice == nil {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgNothingToUpdate, nil)
	}
	storedSheet, err := s.fetchOwned(ctx, cmd.ref())
	if err != nil {
		return nil, err
	}
	return &preparedSave{
		templateID: storedSheet.TemplateID,
		version:    storedSheet.Version,
		delta:      map[string]any{},
		cascade:    map[string]any{},
	}, nil
}

func patchLogFields(sheetID, requestID, templateID string, versionBefore int64, delta, cascade map[string]any) []logger.Attribute {
	return []logger.Attribute{
		{Key: "sheet_id", Value: sheetID},
		{Key: "template_id", Value: templateID},
		{Key: "request_id", Value: requestID},
		{Key: "version_before", Value: strconv.FormatInt(versionBefore, 10)},
		{Key: "delta_paths", Value: joinSortedPaths(delta)},
		{Key: "delta_values", Value: numericValuesOf(delta)},
		{Key: "cascade_paths", Value: joinSortedPaths(cascade)},
		{Key: "cascade_values", Value: numericValuesOf(cascade)},
	}
}

func joinSortedPaths(flat map[string]any) string {
	paths := make([]string, 0, len(flat))
	for path := range flat {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return strings.Join(paths, ",")
}

func numericValuesOf(flat map[string]any) string {
	pairs := make([]string, 0, len(flat))
	for path, value := range flat {
		if text, ok := numericText(value); ok {
			pairs = append(pairs, path+"="+text)
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

func numericText(value any) (string, bool) {
	switch v := value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return strconv.Itoa(v), true
	}
	return "", false
}

func (s *ServiceImpl) Preview(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	result, err := s.recalculate(ctx, cmd.ref(), cmd.Data)
	if err != nil {
		return nil, err
	}
	return nestedCascadingEffects(result.cascade)
}

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

func (s *ServiceImpl) SetLayout(ctx context.Context, cmd SetLayoutCommand) (*Layout, error) {
	if err := requireOwner(cmd.OwnerID); err != nil {
		return nil, err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return nil, err
	}

	stored, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}
	if err := checkOwnership(stored, cmd.OwnerID); err != nil {
		return nil, err
	}

	var layout *Layout
	if !cmd.Layout.IsEmpty() {
		tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: stored.TemplateID})
		if err != nil {
			return nil, translateTemplateErr(err)
		}
		if err := template.ValidateLayout(*tmpl, cmd.Layout.SectionOrder, cmd.Layout.Pinned); err != nil {
			return nil, translateLayoutErr(err)
		}
		layout = cmd.Layout.Clone()
	}

	if err := s.repository.UpdateLayout(ctx, cmd.SheetID, layout); err != nil {
		return nil, translateRepoErr("update", err)
	}
	s.publishSheetUpdated(cmd.OwnerID, cmd.SheetID, []string{layoutChangedKey}, stored.Version)
	return layout, nil
}

func (s *ServiceImpl) validatedDice(ctx context.Context, templateID string, dice *Dice) (*Dice, error) {
	if dice.IsEmpty() {
		return nil, nil
	}
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: templateID})
	if err != nil {
		return nil, translateTemplateErr(err)
	}
	if err := template.ValidateSheetDice(*tmpl, dice.Section, dice.Row); err != nil {
		return nil, translateDiceErr(err)
	}
	return dice.Clone(), nil
}

func (s *ServiceImpl) Delete(ctx context.Context, cmd DeleteSheetCommand) error {
	if err := requireOwner(cmd.OwnerID); err != nil {
		return err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return err
	}

	stored, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return translateRepoErr("delete", err)
	}
	if err := checkOwnership(stored, cmd.OwnerID); err != nil {
		return err
	}

	var removed []RemovedLink
	remove := func(txCtx context.Context) error {
		removed = nil
		if err := s.repository.DeleteByID(txCtx, cmd.SheetID); err != nil {
			return err
		}
		if s.linkCleaner != nil {
			found, err := s.linkCleaner.DeleteForSheet(txCtx, cmd.SheetID)
			if err != nil {
				return err
			}
			removed = found
		}
		if !s.planLimitsEnabled() {
			return nil
		}
		return s.repository.ReleaseSlot(txCtx, cmd.OwnerID, stored.Kind)
	}

	if s.tx == nil {
		err = remove(ctx)
	} else {
		err = s.tx.WithTransaction(ctx, remove)
	}
	if err != nil {
		return translateRepoErr("delete", err)
	}
	if s.linkCleaner != nil && len(removed) > 0 {
		s.linkCleaner.PublishSheetUnlinked(stored.Kind, removed)
	}
	s.publishSheetEvent(eventSheetDeleted, cmd.OwnerID, cmd.SheetID, stored.Version, map[string]any{})
	return nil
}

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
	if errors.Is(err, template.ErrTemplateNotFound) {
		return apperr.NewError(apperr.CodeNotFound, "template not found", err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to fetch template", err)
}

func translateLayoutErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	var unknownSection *template.UnknownLayoutSectionError
	var unknownRow *template.UnknownLayoutRowError
	var duplicate *template.DuplicateLayoutKeyError
	if errors.As(err, &unknownSection) || errors.As(err, &unknownRow) || errors.As(err, &duplicate) {
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid layout: "+err.Error(), err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to validate layout", err)
}

func translateDiceErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	var unknownSection *template.UnknownDiceSectionError
	var unknownRow *template.UnknownDiceRowError
	var invalid *template.InvalidDiceError
	var tooMany *template.TooManyDiceOverridesError
	if errors.As(err, &unknownSection) || errors.As(err, &unknownRow) || errors.As(err, &invalid) || errors.As(err, &tooMany) {
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid dice: "+err.Error(), err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to validate dice", err)
}

func translateRepoErr(op string, err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, sheeterr.ErrSheetNotFound) {
		return apperr.NewError(apperr.CodeNotFound, "sheet not found", err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to "+op+" sheet", err)
}

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
	var tooLong *template.TextTooLongError
	if errors.As(err, &tooLong) {
		return apperr.NewError(apperr.CodeInvalidRequest, tooLong.Error(), err)
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

func isMaputilsDataErr(err error) bool {
	return errors.Is(err, maputils.ErrEmptyPath) ||
		errors.Is(err, maputils.ErrNilMap) ||
		errors.Is(err, maputils.ErrNotMap)
}

func nestedCascadingEffects(flat map[string]any) (map[string]any, error) {
	nested := make(map[string]any)
	if err := maputils.ApplyUpdates(nested, flat); err != nil {
		return nil, translateMapErr("update", err)
	}
	return nested, nil
}

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
