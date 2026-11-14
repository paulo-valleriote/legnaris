package generation

import (
	"fmt"

	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/maputils"
)

// Assemble builds the Sheet data a generation stores. It seeds the Template exactly
// as PreviewTemplate does (every leaf at its seed value, derived leaves then filled
// by the DAG), applies the expanded values through NormalizeAndRecalculate and
// returns the nested result. Nothing is persisted: the caller stores the map on the
// generation document until the player accepts it.
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

// seedNested returns the nested data of a new Sheet of tmpl: each leaf at its seed
// value, every group nested as in the Template.
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
