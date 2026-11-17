package generation

import (
	"fmt"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/maputils"
)

func Assemble(tmpl *template.Template, dag *template.DAG, kind plan.SheetKind, values map[string]any) (map[string]any, error) {
	if tmpl == nil || dag == nil {
		return nil, ErrNoTemplate
	}
	seeded := seedNested(tmpl.Fields)

	flatSeed, err := maputils.FlattenPathKeys(seeded)
	if err != nil {
		return nil, fmt.Errorf("flatten seed: %w", err)
	}
	derived, err := dag.Recalculate(map[string]any{}, flatSeed)
	if err != nil {
		return nil, fmt.Errorf("recalculate seed: %w", err)
	}
	if err := maputils.ApplyUpdates(seeded, derived); err != nil {
		return nil, fmt.Errorf("apply seed derivations: %w", err)
	}

	original, err := maputils.FlattenPathKeys(seeded)
	if err != nil {
		return nil, fmt.Errorf("flatten original: %w", err)
	}
	updates, err := maputils.FlattenPathKeys(filterGenerated(tmpl.Fields, kind, values))
	if err != nil {
		return nil, fmt.Errorf("flatten values: %w", err)
	}
	normalized, cascade, err := dag.NormalizeAndRecalculate(original, updates)
	if err != nil {
		return nil, fmt.Errorf("apply generated values: %w", err)
	}
	if err := maputils.ApplyUpdates(seeded, normalized); err != nil {
		return nil, fmt.Errorf("apply normalized values: %w", err)
	}
	if err := maputils.ApplyUpdates(seeded, cascade); err != nil {
		return nil, fmt.Errorf("apply cascade: %w", err)
	}
	return seeded, nil
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
	return out
}

func seedNested(group template.FieldGroup) map[string]any {
	out := make(map[string]any, len(group))
	for name, node := range group {
		switch {
		case node.IsLeaf():
			out[name] = node.Schema.SeedValue()
		case node.IsGroup():
			out[name] = seedNested(node.Children)
		}
	}
	return out
}
