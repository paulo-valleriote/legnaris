// Package sheettest provides test doubles for the sheet service layer.
package sheettest

import "legnaris/backend/internal/sheet/template"

// DAGMap adapts a plain map to the sheet.DAGProvider interface, so tests can
// wire hand-built DAGs without constructing a whole template.Registry
// (LGN-54: the service takes a provider, not a mutable map).
type DAGMap map[string]*template.DAG

// DAG resolves the DAG stored under the given template ID.
func (p DAGMap) DAG(id string) (*template.DAG, bool) {
	dag, ok := p[id]
	return dag, ok
}
