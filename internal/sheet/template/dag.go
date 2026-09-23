package template

import (
	"legnaris/internal/engine"
	"strings"

	"github.com/expr-lang/expr/vm"
)

type Node struct {
	Path            []string
	FormulaString   string
	CompiledFormula *vm.Program
	IsEditable      bool
}

func (n *Node) IsDerived() bool {
	return n.CompiledFormula != nil
}

func (n *Node) MapKey() string {
	return strings.Join(n.Path, ".")
}

type DAG struct {
	TemplateID       string
	Nodes            map[string]*Node
	AdjacencyList    map[string][]string
	TopologicalOrder []string
	formulaEngine    *engine.FormulaEngine
}

func BuildDAG(templateID string, fields FieldGroup, eng *engine.FormulaEngine) (*DAG, error) {
	dag := &DAG{
		TemplateID:    templateID,
		Nodes:         make(map[string]*Node),
		AdjacencyList: make(map[string][]string),
		formulaEngine: eng,
	}

	var leafNodes []*FieldSchema
	var leafPaths [][]string
	collectLeaves(fields, []string{}, &leafNodes, &leafPaths)

	mockEnv := make(map[string]any)
	inDegree := make(map[string]int)

	for _, path := range leafPaths {
		key := strings.Join(path, ".")
		safeKey := strings.Join(path, "_")
		mockEnv[safeKey] = 0.0
		inDegree[key] = 0
	}

	for i, schema := range leafNodes {
		path := leafPaths[i]
		key := strings.Join(path, ".")

		node := &Node{
			Path:            path,
			FormulaString:   schema.Formula,
			IsEditable:      schema.IsEditable,
		}

		if schema.Formula != "" {
			program, err := eng.Compile(schema.Formula, mockEnv)
			if err != nil {
				return nil, ErrCompileFormula(schema.Formula, key, err)
			}
			node.CompiledFormula = program
		}

		dag.Nodes[key] = node

		for _, depPath := range schema.DependsOn {
			depKey := strings.Join(depPath, ".")
			dag.AdjacencyList[depKey] = append(dag.AdjacencyList[depKey], key)
			inDegree[key]++
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

	if len(topoOrder) != len(leafNodes) {
		return nil, ErrInfiniteCycle
	}
	dag.TopologicalOrder = topoOrder

	return dag, nil
}

func collectLeaves(group FieldGroup, prefix []string, schemas *[]*FieldSchema, paths *[][]string) {
	for name, node := range group {
		currentPath := append(append([]string{}, prefix...), name)
		if node.IsLeaf() {
			*schemas = append(*schemas, node.Schema)
			*paths = append(*paths, currentPath)
		} else if node.IsGroup() {
			collectLeaves(node.Children, currentPath, schemas, paths)
		}
	}
}

func (d *DAG) Recalculate(originalData map[string]any, updatedData map[string]any) (map[string]any, error) {
	copiedSheetData := deepCopyNested(originalData)
	applyNestedUpdates(copiedSheetData, updatedData)

	needsRecalc := make(map[string]bool)
	for k := range updatedData {
		needsRecalc[k] = true
	}

	safeSheetData := toFlatForEngine(copiedSheetData)

	cascadingEffects := make(map[string]any)
	for _, nodeKey := range d.TopologicalOrder {
		if !needsRecalc[nodeKey] {
			continue
		}

		node := d.Nodes[nodeKey]
		shouldPropagate := !node.IsDerived()

		if node.IsDerived() {
			newValue, err := d.formulaEngine.Evaluate(node.CompiledFormula, safeSheetData)
			if err != nil {
				return nil, err
			}

			oldValue, _ := getNestedValue(copiedSheetData, node.Path)
			if oldValue != newValue {
				setNestedValue(copiedSheetData, node.Path, newValue)
				safeSheetData[strings.Join(node.Path, "_")] = newValue
				cascadingEffects[nodeKey] = newValue
				shouldPropagate = true
			}
		}

		if shouldPropagate {
			for _, childKey := range d.AdjacencyList[nodeKey] {
				needsRecalc[childKey] = true
			}
		}
	}

	return cascadingEffects, nil
}

func getNestedValue(nested map[string]any, path []string) (any, bool) {
	current := nested
	for i, key := range path {
		if i == len(path)-1 {
			val, ok := current[key]
			return val, ok
		}
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

func setNestedValue(nested map[string]any, path []string, value any) {
	current := nested
	for i, key := range path {
		if i == len(path)-1 {
			current[key] = value
		} else {
			if current[key] == nil {
				current[key] = make(map[string]any)
			}
			nextMap, ok := current[key].(map[string]any)
			if !ok {
				nextMap = make(map[string]any)
				current[key] = nextMap
			}
			current = nextMap
		}
	}
}

func applyNestedUpdates(nested map[string]any, updates map[string]any) {
	for key, value := range updates {
		path := strings.Split(key, ".")
		setNestedValue(nested, path, value)
	}
}

func toFlatForEngine(nested map[string]any) map[string]any {
	flat := make(map[string]any)
	flattenRecursive([]string{}, nested, flat)
	return flat
}

func flattenRecursive(prefix []string, nested map[string]any, flat map[string]any) {
	for key, value := range nested {
		newPrefix := append(append([]string{}, prefix...), key)
		if nestedMap, ok := value.(map[string]any); ok && len(nestedMap) > 0 {
			flattenRecursive(newPrefix, nestedMap, flat)
		} else {
			flat[strings.Join(newPrefix, "_")] = value
		}
	}
}

func deepCopyNested(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if nestedMap, ok := v.(map[string]any); ok {
			dst[k] = deepCopyNested(nestedMap)
		} else {
			dst[k] = v
		}
	}
	return dst
}
