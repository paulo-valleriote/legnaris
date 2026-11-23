package template

import (
	"fmt"

	"legnaris/backend/internal/engine"
)

type Registry struct {
	templates map[string]Template
	dags      map[string]*DAG
	order     []string
}

func BuildRegistry(eng *engine.FormulaEngine) (*Registry, error) {
	return buildRegistry(eng, DefaultTemplates())
}

func buildRegistry(eng *engine.FormulaEngine, templates []Template) (*Registry, error) {
	registry := &Registry{
		templates: make(map[string]Template, len(templates)),
		dags:      make(map[string]*DAG, len(templates)),
		order:     make([]string, 0, len(templates)),
	}

	for _, tmpl := range templates {
		stored := cloneTemplate(tmpl)
		if err := validateTemplateGameSystem(stored); err != nil {
			return nil, err
		}
		if err := stored.prepareRowIndex(); err != nil {
			return nil, fmt.Errorf("template %q: %w", stored.ID, err)
		}

		dag, err := BuildDAGWithActions(stored.ID, stored.Fields, stored.Actions, eng)
		if err != nil {
			return nil, err
		}

		if err := ValidateDiceDefaults(stored); err != nil {
			return nil, fmt.Errorf("template %q: %w", stored.ID, err)
		}

		if err := ValidateNotD20Tests(stored); err != nil {
			return nil, fmt.Errorf("template %q: %w", stored.ID, err)
		}

		if err := ValidateGenerationMetadata(stored); err != nil {
			return nil, fmt.Errorf("template %q: %w", stored.ID, err)
		}

		if err := ValidateFlags(stored); err != nil {
			return nil, fmt.Errorf("template %q: %w", stored.ID, err)
		}

		if err := ValidateNameLeaf(stored); err != nil {
			return nil, fmt.Errorf("template %q: %w", stored.ID, err)
		}

		registry.templates[stored.ID] = stored
		registry.dags[stored.ID] = dag
		registry.order = append(registry.order, stored.ID)
	}

	return registry, nil
}

func (r *Registry) List() []Template {
	result := make([]Template, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, cloneTemplate(r.templates[id]))
	}
	return result
}

func (r *Registry) FetchByID(id string) (*Template, bool) {
	tmpl, ok := r.templates[id]
	if !ok {
		return nil, false
	}
	tmpl = cloneTemplate(tmpl)
	return &tmpl, true
}

func (r *Registry) DAG(id string) (*DAG, bool) {
	dag, ok := r.dags[id]
	return dag, ok
}
