package template

import (
	"errors"
	"fmt"

	"legnaris/internal/engine"
	"legnaris/shared/apperr"
	"legnaris/shared/maputils"

	"github.com/expr-lang/expr/vm"
)

// DAG represents a directed acyclic graph of computed fields for a template.
// Nodes are evaluated in TopologicalOrder to ensure dependencies are resolved
// before dependents. The graph is built once per template at startup.
type DAG struct {
	TemplateID       string
	Nodes            map[string]*Node
	AdjacencyList    map[string][]string
	TopologicalOrder []string
	formulaEngine    *engine.FormulaEngine
}

// Node represents a single field in the DAG.
type Node struct {
	Path            Path
	FormulaString   string
	CompiledFormula *vm.Program
	IsEditable      bool
}

// IsDerived returns true if this node has a formula (computed field).
func (n *Node) IsDerived() bool {
	return n.CompiledFormula != nil
}

// MapKey returns the dot-separated key used in DAG maps (e.g., "attributes.str").
func (n *Node) MapKey() string {
	return n.Path.String()
}

// BuildDAG constructs a computation DAG from a template's field schemas.
// Returns an error if any formula fails to compile or a dependency cycle is detected.
func BuildDAG(templateID string, fields FieldGroup, eng *engine.FormulaEngine) (*DAG, error) {
	if eng == nil {
		return nil, errors.New("formula engine is required")
	}

	leafSchemas, leafPaths := collectLeaves(fields)
	if len(leafSchemas) == 0 {
		return newDAG(templateID, eng), nil
	}

	dag := newDAG(templateID, eng)
	compilationEnv := buildCompilationEnv(leafPaths)

	if err := dag.compileNodeFormulas(leafSchemas, leafPaths, compilationEnv); err != nil {
		return nil, err
	}

	dag.buildAdjacencyList(leafSchemas, leafPaths)

	topoOrder, err := topologicalSort(dag.Nodes, dag.AdjacencyList)
	if err != nil {
		return nil, err
	}
	dag.TopologicalOrder = topoOrder

	return dag, nil
}

// newDAG allocates and initializes a DAG with empty collections.
func newDAG(templateID string, eng *engine.FormulaEngine) *DAG {
	return &DAG{
		TemplateID:    templateID,
		Nodes:         make(map[string]*Node),
		AdjacencyList: make(map[string][]string),
		formulaEngine: eng,
	}
}

// collectLeaves traverses the field group tree and returns all leaf schemas with their paths.
func collectLeaves(group FieldGroup) ([]*FieldSchema, []Path) {
	var schemas []*FieldSchema
	var paths []Path
	collectLeavesRecursive(group, Path{}, &schemas, &paths)
	return schemas, paths
}

func collectLeavesRecursive(group FieldGroup, prefix Path, schemas *[]*FieldSchema, paths *[]Path) {
	for name, node := range group {
		currentPath := append(append(Path{}, prefix...), name)
		if node.IsLeaf() {
			*schemas = append(*schemas, node.Schema)
			*paths = append(*paths, currentPath)
		} else if node.IsGroup() {
			collectLeavesRecursive(node.Children, currentPath, schemas, paths)
		}
	}
}

// buildCompilationEnv creates a flat environment with zero values for all leaf paths.
// Keys use underscore separator (e.g., "attributes_str") for expr-lang compilation.
func buildCompilationEnv(paths []Path) map[string]any {
	env := make(map[string]any, len(paths))
	for _, path := range paths {
		env[path.EngineKey()] = 0.0
	}
	return env
}

// compileNodeFormulas compiles formulas for all leaf nodes and populates dag.Nodes.
func (d *DAG) compileNodeFormulas(schemas []*FieldSchema, paths []Path, compilationEnv map[string]any) error {
	for i, schema := range schemas {
		path := paths[i]
		key := path.String()

		node := &Node{
			Path:          path,
			FormulaString: schema.Formula,
			IsEditable:    schema.IsEditable,
		}

		if schema.Formula != "" {
			program, err := d.formulaEngine.Compile(schema.Formula, compilationEnv)
			if err != nil {
				return ErrCompileFormula(schema.Formula, key, err)
			}
			node.CompiledFormula = program
		}

		d.Nodes[key] = node
	}
	return nil
}

// buildAdjacencyList constructs the dependency graph from DependsOn declarations.
func (d *DAG) buildAdjacencyList(schemas []*FieldSchema, paths []Path) {
	for i, schema := range schemas {
		key := paths[i].String()
		for _, depPath := range schema.DependsOn {
			depKey := Path(depPath).String()
			d.AdjacencyList[depKey] = append(d.AdjacencyList[depKey], key)
		}
	}
}

// topologicalSort performs Kahn's algorithm to produce a valid evaluation order.
// Returns ErrInfiniteCycle if the graph contains a cycle.
func topologicalSort(nodes map[string]*Node, adj map[string][]string) ([]string, error) {
	inDegree := make(map[string]int, len(nodes))
	for key := range nodes {
		inDegree[key] = 0
	}
	for _, children := range adj {
		for _, child := range children {
			inDegree[child]++
		}
	}

	queue := newQueue[string]()
	for id, deg := range inDegree {
		if deg == 0 {
			queue.append(id)
		}
	}

	var topoOrder []string
	for queue.size() > 0 {
		curr := queue.pop()
		topoOrder = append(topoOrder, curr)

		for _, child := range adj[curr] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue.append(child)
			}
		}
	}

	if len(topoOrder) != len(nodes) {
		return nil, ErrInfiniteCycle
	}
	return topoOrder, nil
}

// Recalculate reapplies updates and recomputes derived fields in topological order.
// Returns a map of only the derived fields that actually changed (cascading effects).
// Returns an error if any formula evaluation fails.
func (d *DAG) Recalculate(originalData, updatedData map[string]any) (map[string]any, error) {
	if d == nil {
		return nil, errors.New("DAG is nil")
	}
	if err := validateRecalculateInputs(originalData, updatedData); err != nil {
		return nil, err
	}

	sheetData, engineEnv, err := d.prepareData(originalData, updatedData)
	if err != nil {
		return nil, err
	}

	needsRecalc := initialRecalcSet(updatedData)
	changedDerived, err := d.walkTopological(sheetData, engineEnv, needsRecalc)
	if err != nil {
		return nil, err
	}

	return changedDerived, nil
}

// prepareData deep-copies original data and applies updates, returning the sheet data and engine environment.
func (d *DAG) prepareData(originalData, updatedData map[string]any) (map[string]any, map[string]any, error) {
	sheetData := maputils.DeepCopy(originalData)
	maputils.ApplyUpdates(sheetData, updatedData)
	engineEnv := maputils.ToEngineEnv(sheetData)
	return sheetData, engineEnv, nil
}

// walkTopological evaluates nodes in topological order and returns changed derived fields.
func (d *DAG) walkTopological(sheetData, engineEnv map[string]any, needsRecalc map[string]bool) (map[string]any, error) {
	changedDerived := make(map[string]any)

	for _, nodeKey := range d.TopologicalOrder {
		if !needsRecalc[nodeKey] {
			continue
		}

		node := d.Nodes[nodeKey]
		propagated, err := d.processNode(node, sheetData, engineEnv, changedDerived)
		if err != nil {
			return nil, err
		}

		if propagated {
			d.propagateChanges(nodeKey, d.AdjacencyList, needsRecalc)
		}
	}

	return changedDerived, nil
}

// propagateChanges marks descendants for recalculation when a node changes.
func (d *DAG) propagateChanges(nodeKey string, adj map[string][]string, needsRecalc map[string]bool) {
	for _, child := range adj[nodeKey] {
		needsRecalc[child] = true
	}
}

func validateRecalculateInputs(originalData, updatedData map[string]any) error {
	if originalData == nil {
		return errors.New("original data is undefined and template dag cannot be recalculated")
	}
	if updatedData == nil {
		return errors.New("updated data is undefined and template dag cannot be recalculated")
	}
	return nil
}

func initialRecalcSet(updatedData map[string]any) map[string]bool {
	set := make(map[string]bool, len(updatedData))
	for k := range updatedData {
		set[k] = true
	}
	return set
}

// processNode evaluates a derived node if needed, updates both nested and flat envs,
// and returns whether the change should propagate to dependents.
// Returns an error if formula evaluation fails, wrapped with field context.
func (d *DAG) processNode(node *Node, sheetData, engineEnv map[string]any, changedDerived map[string]any) (bool, error) {
	if node == nil {
		return false, errors.New("node is nil")
	}

	// Editable nodes always propagate (they are the source of changes)
	if !node.IsDerived() {
		return true, nil
	}

	newValue, err := d.formulaEngine.Evaluate(node.CompiledFormula, engineEnv)
	if err != nil {
		return false, apperr.NewError(
			apperr.CodeInvalidRequest,
			fmt.Sprintf("formula evaluation failed for field '%s'", node.MapKey()),
			fmt.Errorf("formula: %s: %w", node.FormulaString, err),
		)
	}

	oldValue, _ := node.Path.Get(sheetData)
	if !maputils.Equal(oldValue, newValue) {
		node.Path.Set(sheetData, newValue)
		engineEnv[node.Path.EngineKey()] = newValue
		changedDerived[node.MapKey()] = newValue
		return true, nil
	}

	return false, nil
}

// --- Queue helper for topological sort ---

type queue[T any] struct {
	data []T
}

func newQueue[T any]() *queue[T] {
	return &queue[T]{data: make([]T, 0)}
}

func (q *queue[T]) append(v T) {
	q.data = append(q.data, v)
}

func (q *queue[T]) pop() T {
	v := q.data[0]
	q.data = q.data[1:]
	return v
}

func (q *queue[T]) size() int {
	return len(q.data)
}
