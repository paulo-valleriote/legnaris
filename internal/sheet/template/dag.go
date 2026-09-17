package template

import (
	"fmt"
	"legnaris/internal/engine"
	"maps"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type RawSchema struct {
	Type       string
	Label      string
	Formula    string
	DependsOn  []string
	IsEditable bool
}

type Node struct {
	ID              string
	FormulaString   string
	CompiledFormula *vm.Program
	IsEditable      bool
}

// DAG - Directed Acyclic Graph
type DAG struct {
	TemplateID    string
	Nodes         map[string]*Node
	AdjacencyList map[string][]string
	formulaEngine *engine.FormulaEngine
}

func BuildDAG(templateID string, rawNodes map[string]RawSchema, engine *engine.FormulaEngine) (*DAG, error) {
	dag := &DAG{
		TemplateID:    templateID,
		Nodes:         make(map[string]*Node),
		AdjacencyList: make(map[string][]string),
		formulaEngine: engine,
	}

	mockEnv := make(map[string]interface{})
	for id := range rawNodes {
		safeID := strings.ReplaceAll(id, ".", "_")
		mockEnv[safeID] = 0.0
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
		}
	}

	return dag, nil
}

func (d *DAG) Recalculate(originalData map[string]any, updatedData map[string]any) (map[string]any, error) {
	// Apply updates
	copiedSheetData := make(map[string]any, len(originalData)+len(updatedData))
	maps.Copy(copiedSheetData, originalData)
	maps.Copy(copiedSheetData, updatedData)

	safeSheetData := make(map[string]interface{})
	for k, v := range copiedSheetData {
		safeSheetData[strings.ReplaceAll(k, ".", "_")] = v
	}

	// Process dependencies
	queue := NewQueue[string]()
	for k := range updatedData {
		queue.Append(k)
	}

	// Keep changes on Track
	cascadingEffects := make(map[string]any)

	// BFS
	for queue.Size() > 0 {
		current := queue.Pop()

		dependents := d.AdjacencyList[current]
		for _, depID := range dependents {
			node := d.Nodes[depID]
			if node.CompiledFormula == nil {
				continue
			}

			newValue, err := d.formulaEngine.Evaluate(node.CompiledFormula, safeSheetData)
			if err != nil {
				return nil, fmt.Errorf("erro calculando '%s': %w", depID, err)
			}

			if copiedSheetData[depID] != newValue {
				copiedSheetData[depID] = newValue
				cascadingEffects[depID] = newValue
				queue.Append(depID)
			}
		}
	}

	return cascadingEffects, nil
}

func evaluateFormula(compiledProgram *vm.Program, data map[string]any) (any, error) {
	result, err := expr.Run(compiledProgram, data)
	if err != nil {
		return nil, ErrFormulaEval(err)
	}
	return result, nil
}
