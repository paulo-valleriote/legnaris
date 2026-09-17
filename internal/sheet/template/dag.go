package template

import (
	"legnaris/internal/engine"
	"maps"
	"strings"

	"github.com/expr-lang/expr/vm"
)

type FieldSchema struct {
	Type       string
	Label      string
	Formula    string
	DependsOn  []string
	IsEditable bool
	Default    float64
}

type Node struct {
	ID              string
	FormulaString   string
	CompiledFormula *vm.Program
	IsEditable      bool
}

func (n *Node) IsDerived() bool {
	return n.CompiledFormula != nil
}

// DAG - Directed Acyclic Graph
type DAG struct {
	TemplateID       string
	Nodes            map[string]*Node
	AdjacencyList    map[string][]string
	TopologicalOrder []string
	formulaEngine    *engine.FormulaEngine
}

func BuildDAG(templateID string, rawNodes map[string]FieldSchema, engine *engine.FormulaEngine) (*DAG, error) {
	dag := &DAG{
		TemplateID:    templateID,
		Nodes:         make(map[string]*Node),
		AdjacencyList: make(map[string][]string),
		formulaEngine: engine,
	}

	mockEnv := make(map[string]any)
	inDegree := make(map[string]int)

	for id := range rawNodes {
		safeID := strings.ReplaceAll(id, ".", "_")
		mockEnv[safeID] = 0.0
		inDegree[id] = 0
	}

	for id, schema := range rawNodes {
		node := &Node{
			ID:            id,
			FormulaString: schema.Formula,
			IsEditable:    schema.IsEditable,
		}

		if schema.Formula != "" {
			safeFormula := strings.ReplaceAll(schema.Formula, ".", "_")

			program, err := engine.Compile(safeFormula, mockEnv)
			if err != nil {
				return nil, ErrCompileFormula(schema.Formula, node.ID, err)
			}
			node.CompiledFormula = program
		}

		dag.Nodes[id] = node

		// Directed Edges Build-Up (A -> B)
		for _, dep := range schema.DependsOn {
			dag.AdjacencyList[dep] = append(dag.AdjacencyList[dep], id)
			inDegree[id]++
		}

	}

	queue := NewQueue[string]()
	for id, deg := range inDegree {
		if deg == 0 {
			queue.Append(id)
		}
	}

	var topoOrder []string
	for queue.Size() > 0 {
		curr := queue.Pop()
		topoOrder = append(topoOrder, curr)

		for _, child := range dag.AdjacencyList[curr] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue.Append(child)
			}
		}
	}

	if len(topoOrder) != len(rawNodes) {
		return nil, ErrInfiniteCycle
	}
	dag.TopologicalOrder = topoOrder

	return dag, nil
}

func (d *DAG) Recalculate(originalData map[string]any, updatedData map[string]any) (map[string]any, error) {
	// Apply updates
	copiedSheetData := make(map[string]any, len(originalData)+len(updatedData))
	maps.Copy(copiedSheetData, originalData)
	maps.Copy(copiedSheetData, updatedData)

	needsRecalc := make(map[string]bool)
	for k := range updatedData {
		needsRecalc[k] = true
	}

	safeSheetData := make(map[string]interface{})
	for k, v := range copiedSheetData {
		safeSheetData[strings.ReplaceAll(k, ".", "_")] = v
	}

	// Keep changes on Track
	cascadingEffects := make(map[string]any)
	for _, nodeID := range d.TopologicalOrder {
		if !needsRecalc[nodeID] {
			continue
		}

		node := d.Nodes[nodeID]
		// Base nodes always start the loop as true
		// Derived nodes start the loop as false and if oldValue != newValue (Value has changed) they become true
		shouldPropagate := !node.IsDerived()

		if node.IsDerived() {
			newValue, err := d.formulaEngine.Evaluate(node.CompiledFormula, safeSheetData)
			if err != nil {
				return nil, err
			}

			if copiedSheetData[nodeID] != newValue {
				copiedSheetData[nodeID] = newValue
				safeSheetData[strings.ReplaceAll(nodeID, ".", "_")] = newValue
				cascadingEffects[nodeID] = newValue

				// Value has changed so it need propagtes
				shouldPropagate = true
			}
		}

		if shouldPropagate {
			for _, childID := range d.AdjacencyList[nodeID] {
				needsRecalc[childID] = true
			}
		}
	}

	return cascadingEffects, nil
}
