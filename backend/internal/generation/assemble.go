package generation

import (
	"errors"
	"fmt"
	"strings"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/maputils"
)

var ErrAssembleFailed = errors.New("generation: template could not assemble the generated values")

func Assemble(tmpl *template.Template, dag *template.DAG, kind plan.SheetKind, values map[string]any) (map[string]any, error) {
	if tmpl == nil || dag == nil {
		return nil, ErrNoTemplate
	}
	seeded := seedNested(tmpl.Fields, true)

	flatSeed, err := maputils.FlattenPathKeys(seedNested(tmpl.Fields, false))
	if err != nil {
		return nil, assembleFailure("flatten seed", err)
	}
	derived, err := dag.Recalculate(map[string]any{}, flatSeed)
	if err != nil {
		return nil, assembleFailure("recalculate seed", err)
	}
	if err := maputils.ApplyUpdates(seeded, derived); err != nil {
		return nil, assembleFailure("apply seed derivations", err)
	}

	updates, err := maputils.FlattenPathKeys(filterGenerated(tmpl.Fields, kind, values))
	if err != nil {
		return nil, assembleFailure("flatten values", err)
	}
	result, err := dag.CalculateSave(seeded, updates)
	if err != nil {
		return nil, assembleGeneratedError(err)
	}
	return result.Merged, nil
}

func assembleFailure(stage string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrAssembleFailed, stage, err)
}

func assembleGeneratedError(err error) error {
	var conflict *template.ExclusiveGroupConflictError
	if errors.As(err, &conflict) {
		return &OutputError{Problems: []string{
			conflict.Group + ": choose exactly one option, got " + strings.Join(conflict.Keys, ", "),
		}}
	}
	var valueType *template.ExclusiveValueTypeError
	if errors.As(err, &valueType) {
		return &OutputError{Problems: []string{valueType.Group + ": must be one of the listed options"}}
	}
	return assembleFailure("apply generated values", err)
}

func filterGenerated(group template.FieldGroup, kind plan.SheetKind, values map[string]any) map[string]any {
	lv := levelOf(group, kind)
	out := make(map[string]any)
	for name := range lv.leaves {
		if value, ok := values[name]; ok {
			out[name] = value
		}
	}
	for name, children := range lv.groups {
		child, ok := values[name].(map[string]any)
		if !ok {
			continue
		}
		if filtered := filterGenerated(children, kind, child); len(filtered) > 0 {
			out[name] = filtered
		}
	}
	for _, options := range lv.exclusive {
		for _, member := range options {
			if value, ok := values[member]; ok {
				out[member] = value
			}
		}
	}
	for name := range lv.lists {
		items, ok := values[name].([]any)
		if !ok {
			continue
		}
		out[name] = generatedRecords(items)
	}
	return out
}

func generatedRecords(items []any) map[string]any {
	records := make(map[string]any, len(items))
	for index, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		record := make(map[string]any, len(fields)+1)
		for name, value := range fields {
			record[name] = value
		}
		record[template.ListPositionField] = float64(index)
		records[generatedRecordID(index)] = record
	}
	return records
}

func generatedRecordID(index int) string {
	return fmt.Sprintf("r%032x", index)
}

func seedNested(group template.FieldGroup, includeLists bool) map[string]any {
	out := make(map[string]any, len(group))
	for name, node := range group {
		switch {
		case node.IsLeaf():
			out[name] = node.Schema.SeedValue()
		case node.IsGroup():
			nested := seedNested(node.Children, includeLists)
			if len(nested) > 0 {
				out[name] = nested
			}
		case node.IsList() && includeLists:
			out[name] = map[string]any{}
		}
	}
	return out
}
