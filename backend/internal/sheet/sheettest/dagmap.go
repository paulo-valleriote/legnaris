package sheettest

import "legnaris/backend/internal/sheet/template"

type DAGMap map[string]*template.DAG

func (p DAGMap) DAG(id string) (*template.DAG, bool) {
	dag, ok := p[id]
	return dag, ok
}
