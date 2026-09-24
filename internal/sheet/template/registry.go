package template

import (
	"legnaris/internal/engine"
)

// Registry is an in-memory catalog of code-defined default templates and their
// compiled DAGs. It is built once at boot: every template is validated and its
// formulas compiled a single time. Templates are immutable and served directly
// from memory — there is no MongoDB template collection or seed step.
type Registry struct {
	templates map[string]Template
	dags      map[string]*DAG
	order     []string
}

// BuildRegistry validates and compiles every default template into a DAG.
// It returns an error if any template fails validation or formula compilation.
func BuildRegistry(eng *engine.FormulaEngine) (*Registry, error) {
	registry := &Registry{
		templates: make(map[string]Template, len(DefaultTemplates)),
		dags:      make(map[string]*DAG, len(DefaultTemplates)),
		order:     make([]string, 0, len(DefaultTemplates)),
	}

	for _, tmpl := range DefaultTemplates {
		dag, err := BuildDAG(tmpl.ID, tmpl.Fields, eng)
		if err != nil {
			return nil, err
		}

		registry.templates[tmpl.ID] = tmpl
		registry.dags[tmpl.ID] = dag
		registry.order = append(registry.order, tmpl.ID)
	}

	return registry, nil
}

// List returns all default templates in declaration order.
func (r *Registry) List() []Template {
	result := make([]Template, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, r.templates[id])
	}
	return result
}

// FetchByID returns the template with the given ID and whether it exists.
// The returned template is a shallow copy; callers must not mutate it.
func (r *Registry) FetchByID(id string) (*Template, bool) {
	tmpl, ok := r.templates[id]
	if !ok {
		return nil, false
	}
	return &tmpl, true
}

// DAG returns the compiled DAG for the given template ID and whether it exists.
func (r *Registry) DAG(id string) (*DAG, bool) {
	dag, ok := r.dags[id]
	return dag, ok
}

// DAGs returns the full templateID → DAG map.
func (r *Registry) DAGs() map[string]*DAG {
	return r.dags
}
