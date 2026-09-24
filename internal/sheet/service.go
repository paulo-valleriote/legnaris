package sheet

import (
	"context"
	"errors"
	"legnaris/internal/sheet/template"
	"legnaris/shared/apperr"
	"legnaris/shared/maputils"
	"time"

	"github.com/google/uuid"
)

type CreateSheetCommand struct {
	TemplateID string `json:"template_id"`
	SheetName  string `json:"sheet_name"`
}

type PartialUpdateSheetCommand struct {
	SheetID string         `json:"-"`
	Data    map[string]any `json:"data"`
}

type Service interface {
	Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error)
	FetchByID(ctx context.Context, id string) (*Sheet, error)
	PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
}

// User-facing error messages and causes shared by the service and its HTTP
// controllers (internal/app imports them to keep wording consistent).
const (
	MsgSheetIDNotProvided = "sheet id not provided"
	MsgFieldDoesNotExist  = "field does not exist: "
)

// ErrSheetIDEmpty is the underlying cause attached when the sheet ID is empty.
var ErrSheetIDEmpty = errors.New("sheet id is empty")

type ServiceImpl struct {
	repository      Repository
	templateService template.Service
	dagCache        map[string]*template.DAG
}

func NewService(repository Repository, templateService template.Service, dags map[string]*template.DAG) Service {
	return &ServiceImpl{
		repository:      repository,
		templateService: templateService,
		dagCache:        dags,
	}
}

func (s *ServiceImpl) Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error) {
	dag, exists := s.dagCache[cmd.TemplateID]
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: cmd.TemplateID})
	if err != nil {
		return nil, err
	}

	// Build initial data with ALL leaf fields (base + derived) with their defaults
	initialData := buildNestedAllFields(tmpl.Fields)

	// Flatten the initial data into dot-notation keys so the DAG can match
	// leaf keys during recalc (the DAG walks leaf keys, not top-level groups).
	flatInitial := maputils.FlattenPathKeys(initialData)

	// Recalculate derived fields from base fields
	derivedEffects, err := dag.Recalculate(map[string]any{}, flatInitial)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	// Apply the flat derived effects back as a nested map (same strategy as
	// PartialUpdate), instead of maps.Copy which would leak flat keys.
	if err := maputils.ApplyUpdates(initialData, derivedEffects); err != nil {
		return nil, translateMapErr("create", err)
	}

	newSheet := Sheet{
		ID:         uuid.NewString(),
		TemplateID: cmd.TemplateID,
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

func (s *ServiceImpl) FetchByID(ctx context.Context, id string) (*Sheet, error) {
	if id == "" {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDNotProvided, ErrSheetIDEmpty)
	}

	sheet, err := s.repository.FetchByID(ctx, id)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}

	return sheet, nil
}

func (s *ServiceImpl) PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	storedSheet, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}

	dag, exists := s.dagCache[storedSheet.TemplateID]
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "DAG not found for template", nil)
	}

	// The API contract is a nested map ({"data":{"stats":{"dexterity":12}}}).
	// Flatten it to flat path keys because the DAG engine operates
	// exclusively on those.
	updates := maputils.FlattenPathKeys(cmd.Data)

	for key := range updates {
		if !dag.HasNode(key) {
			return nil, apperr.NewError(apperr.CodeUnknownField, MsgFieldDoesNotExist+key, nil)
		}
		node, _ := dag.GetNode(key)
		if !node.IsEditable {
			return nil, apperr.NewError(apperr.CodeInvalidRequest, "field is not editable: "+key, nil)
		}
	}

	cascadingEffects, err := dag.Recalculate(storedSheet.Data, updates)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	// Merge both original updates AND cascading effects
	mergedData := maputils.DeepCopy(storedSheet.Data)
	if err := maputils.ApplyUpdates(mergedData, updates); err != nil {
		return nil, translateMapErr("update", err)
	}
	if err := maputils.ApplyUpdates(mergedData, cascadingEffects); err != nil {
		return nil, translateMapErr("update", err)
	}

	dbUpdates := map[string]any{
		"data":       mergedData,
		"updated_at": time.Now(),
	}

	if err := s.repository.PartialUpdate(ctx, cmd.SheetID, dbUpdates); err != nil {
		return nil, translateRepoErr("update", err)
	}
	return nestedCascadingEffects(cascadingEffects)
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
		return apperr.NewError(apperr.CodeUnknownField, unknownField.Error(), nil)
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
// using their default values (0 for derived fields without explicit default)
func buildNestedAllFields(group template.FieldGroup) map[string]any {
	result := make(map[string]any)
	for name, node := range group {
		if node.IsLeaf() {
			result[name] = node.Schema.Default
		} else if node.IsGroup() {
			nested := buildNestedAllFields(node.Children)
			if len(nested) > 0 {
				result[name] = nested
			}
		}
	}
	return result
}
