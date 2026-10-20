package template

import (
	"legnaris/backend/internal/engine"
)

// Registry is an in-memory catalog of code-defined default templates and their
// compiled DAGs. It is built once at boot: every template is validated and its
// formulas compiled a single time. Templates are immutable and served directly
// from memory — there is no MongoDB template collection or seed step.
//
// The registry is frozen after boot (LGN-54): it is the only writer of its own
// state and no exported method mutates it. It clones DefaultTemplates() on ingest
// and hands out deep copies from List and FetchByID, so neither the exported
// catalog nor a caller can change what it serves. DAGs are shared by pointer
// through DAG(id) — never as a map — and are themselves immutable.
type Registry struct {
	templates map[string]Template
	dags      map[string]*DAG
	order     []string
}

// BuildRegistry validates and compiles every default template into a DAG.
// It returns an error if any template fails validation or formula compilation.
func BuildRegistry(eng *engine.FormulaEngine) (*Registry, error) {
	templates := DefaultTemplates()
	registry := &Registry{
		templates: make(map[string]Template, len(templates)),
		dags:      make(map[string]*DAG, len(templates)),
		order:     make([]string, 0, len(templates)),
	}

	for _, tmpl := range templates {
		// Clone on ingest: the registry must own its templates, not alias the
		// DefaultTemplates() catalog that anyone can mutate (LGN-54).
		stored := cloneTemplate(tmpl)

		dag, err := BuildDAG(stored.ID, stored.Fields, eng)
		if err != nil {
			return nil, err
		}

		registry.templates[stored.ID] = stored
		registry.dags[stored.ID] = dag
		registry.order = append(registry.order, stored.ID)
	}

	return registry, nil
}

// List returns all default templates in declaration order.
// Each template is a deep copy: mutating it cannot reach the registry.
func (r *Registry) List() []Template {
	result := make([]Template, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, cloneTemplate(r.templates[id]))
	}
	return result
}

// FetchByID returns a deep copy of the template with the given ID and whether
// it exists; the copy shares no map, schema or path with the registry.
func (r *Registry) FetchByID(id string) (*Template, bool) {
	tmpl, ok := r.templates[id]
	if !ok {
		return nil, false
	}
	tmpl = cloneTemplate(tmpl)
	return &tmpl, true
}

// DAG returns the compiled DAG for the given template ID and whether it exists.
// Every caller gets the same instance: the graph is immutable after boot, so
// sharing it is safe (LGN-54).
func (r *Registry) DAG(id string) (*DAG, bool) {
	dag, ok := r.dags[id]
	return dag, ok
}
