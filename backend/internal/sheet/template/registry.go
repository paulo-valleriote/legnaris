package template

import (
	"fmt"

	"legnaris/backend/internal/engine"
)

type Registry struct {
	templates     map[string]Template
	dags          map[string]*DAG
	order         []string
	draftsAllowed bool
}

type TemplateStateRef struct {
	ID    string
	State State
}

type RegistryOption func(*Registry)

func WithDraftsAllowed(allowed bool) RegistryOption {
	return func(r *Registry) {
		r.draftsAllowed = allowed
	}
}

func BuildRegistry(eng *engine.FormulaEngine, opts ...RegistryOption) (*Registry, error) {
	return buildRegistry(eng, DefaultTemplates(), opts...)
}

func buildRegistry(eng *engine.FormulaEngine, templates []Template, opts ...RegistryOption) (*Registry, error) {
	registry := &Registry{
		templates: make(map[string]Template, len(templates)),
		dags:      make(map[string]*DAG, len(templates)),
		order:     make([]string, 0, len(templates)),
	}
	for _, opt := range opts {
		opt(registry)
	}

	for _, tmpl := range templates {
		stored := cloneTemplate(tmpl)
		if !stored.State.Valid() {
			return nil, fmt.Errorf("template %q: %w", stored.ID, ErrInvalidTemplateState(stored.ID, stored.State))
		}
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

		if err := ValidatePromptGuidance(stored); err != nil {
			return nil, fmt.Errorf("template %q: %w", stored.ID, err)
		}

		registry.templates[stored.ID] = stored
		registry.dags[stored.ID] = dag
		registry.order = append(registry.order, stored.ID)
	}

	return registry, nil
}

func (r *Registry) DraftsAllowed() bool {
	return r.draftsAllowed
}

func (r *Registry) List() []Template {
	result := make([]Template, 0, len(r.order))
	for _, id := range r.order {
		tmpl := r.templates[id]
		if !tmpl.State.Listed(r.draftsAllowed) {
			continue
		}
		result = append(result, cloneTemplate(tmpl))
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

func (r *Registry) TemplateStatesByGameSystem(gameSystemID string) []TemplateStateRef {
	if gameSystemID == "" {
		return nil
	}
	result := make([]TemplateStateRef, 0, len(r.order))
	for _, id := range r.order {
		tmpl := r.templates[id]
		if tmpl.GameSystemID != gameSystemID {
			continue
		}
		result = append(result, TemplateStateRef{ID: tmpl.ID, State: tmpl.State})
	}
	return result
}

func (r *Registry) DAG(id string) (*DAG, bool) {
	dag, ok := r.dags[id]
	return dag, ok
}
