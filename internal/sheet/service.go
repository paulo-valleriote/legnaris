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
		return nil, errors.New("template not founded")
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
		return nil, err
	}

	// Apply the flat derived effects back as a nested map (same strategy as
	// PartialUpdate), instead of maps.Copy which would leak flat keys.
	applyNestedUpdates(initialData, derivedEffects)

	newSheet := Sheet{
		ID:         uuid.NewString(),
		TemplateID: cmd.TemplateID,
		Name:       cmd.SheetName,
		Data:       initialData,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repository.Create(ctx, &newSheet); err != nil {
		return nil, err
	}
	return &newSheet, nil
}

func (s *ServiceImpl) FetchByID(ctx context.Context, id string) (*Sheet, error) {
	if id == "" {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, "sheet id not informed", errors.New("sheet id is empty"))
	}

	sheet, err := s.repository.FetchByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return sheet, nil
}

func (s *ServiceImpl) PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	storedSheet, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return nil, err
	}

	dag, exists := s.dagCache[storedSheet.TemplateID]
	if !exists {
		return nil, errors.New("DAG not found for template")
	}

	// The API contract is a nested map ({"data":{"stats":{"dexterity":12}}}).
	// Flatten it to flat path keys because the DAG engine operates
	// exclusively on those.
	updates := maputils.FlattenPathKeys(cmd.Data)

	for key := range updates {
		if !dag.HasNode(key) {
			return nil, errors.New("field not exists: " + key)
		}
		node, _ := dag.GetNode(key)
		if !node.IsEditable {
			return nil, errors.New("field is not editable: " + key)
		}
	}

	cascadingEffects, err := dag.Recalculate(storedSheet.Data, updates)
	if err != nil {
		return nil, err
	}

	// Merge both original updates AND cascading effects
	mergedData := deepCopyNested(storedSheet.Data)
	applyNestedUpdates(mergedData, updates)
	applyNestedUpdates(mergedData, cascadingEffects)

	dbUpdates := map[string]any{
		"data":       mergedData,
		"updated_at": time.Now(),
	}

	if err := s.repository.PartialUpdate(ctx, cmd.SheetID, dbUpdates); err != nil {
		return nil, err
	}
	return nestedCascadingEffects(cascadingEffects), nil
}

// nestedCascadingEffects converts the flat path-key cascading effects
// returned by DAG.Recalculate back into a nested map so the API response
// mirrors the nested request payload.
func nestedCascadingEffects(flat map[string]any) map[string]any {
	nested := make(map[string]any)
	applyNestedUpdates(nested, flat)
	return nested
}

// buildNestedAllFields creates nested map with ALL leaf fields (base + derived)
// using their default values (0 for derived fields without explicit default)
func buildNestedAllFields(group template.FieldGroup) map[string]any {
	result := make(map[string]any)
	for name, node := range group {
		if node.IsLeaf() {
			setNestedValue(result, []string{name}, node.Schema.Default)
		} else if node.IsGroup() {
			nested := buildNestedAllFields(node.Children)
			if len(nested) > 0 {
				result[name] = nested
			}
		}
	}
	return result
}

func setNestedValue(nested map[string]any, path []string, value any) {
	current := nested
	for i, key := range path {
		if i == len(path)-1 {
			current[key] = value
		} else {
			if current[key] == nil {
				current[key] = make(map[string]any)
			}
			nextMap, ok := current[key].(map[string]any)
			if !ok {
				nextMap = make(map[string]any)
				current[key] = nextMap
			}
			current = nextMap
		}
	}
}

func applyNestedUpdates(nested map[string]any, updates map[string]any) {
	for key, value := range updates {
		path := []string{}
		for _, part := range splitPathKey(key) {
			path = append(path, part)
		}
		setNestedValue(nested, path, value)
	}
}

func deepCopyNested(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if nestedMap, ok := v.(map[string]any); ok {
			dst[k] = deepCopyNested(nestedMap)
		} else {
			dst[k] = v
		}
	}
	return dst
}

func splitPathKey(key string) []string {
	var result []string
	current := ""
	for _, c := range key {
		if c == '.' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
