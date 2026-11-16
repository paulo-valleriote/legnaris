package generation

import (
	"fmt"

	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/maputils"
)

func Assemble(tmpl *template.Template, dag *template.DAG, values map[string]any) (map[string]any, error) {
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
	updates, err := maputils.FlattenPathKeys(values)
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
