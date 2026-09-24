package template

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"legnaris/internal/engine"
	"legnaris/shared/maputils"

	"github.com/expr-lang/expr/vm"
)

// DAG represents a directed acyclic graph of computed fields for a template.
// Nodes are evaluated in TopologicalOrder to ensure dependencies are resolved
// before dependents. The graph is built once per template at startup and is
// immutable afterwards: fields are read-only for the lifetime of the DAG.
// Concurrent read access via Recalculate is safe; mutation after construction
// is prevented by unexported fields and a sync.RWMutex guard.
type DAG struct {
	sync.RWMutex
	TemplateID       string
	nodes            map[string]*Node
	adjacencyList    map[string][]string
	topologicalOrder []string
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

// MapKey returns the flat path key used in DAG maps (e.g., "attributes.str").
func (n *Node) MapKey() string {
	return n.Path.String()
}

// HasNode returns true if a node with the given key exists in the DAG.
func (d *DAG) HasNode(key string) bool {
	d.RLock()
	defer d.RUnlock()
	_, ok := d.nodes[key]
	return ok
}

// GetNode returns the node for the given key and whether it was found.
func (d *DAG) GetNode(key string) (*Node, bool) {
	d.RLock()
	defer d.RUnlock()
	node, ok := d.nodes[key]
	return node, ok
}

// GetChildren returns the list of child keys for the given node key.
// Returns nil if the key has no children.
func (d *DAG) GetChildren(key string) []string {
	d.RLock()
	defer d.RUnlock()
	children, ok := d.adjacencyList[key]
	if !ok {
		return nil
	}
	// Return a copy to prevent external mutation.
	result := make([]string, len(children))
	copy(result, children)
	return result
}

// TopologicalOrder returns a copy of the evaluation order of node keys.
func (d *DAG) TopologicalOrder() []string {
	d.RLock()
	defer d.RUnlock()
	order := make([]string, len(d.topologicalOrder))
	copy(order, d.topologicalOrder)
	return order
}

// BuildDAG constructs a computation DAG from a template's field schemas.
// Returns an error if any formula fails to compile, a dependency cycle is detected,
// or a DependsOn references a non-existent field.
// The DAG is write-locked during construction to ensure thread-safe initialization.
func BuildDAG(templateID string, fields FieldGroup, eng *engine.FormulaEngine) (*DAG, error) {
	if eng == nil {
		return nil, ErrNilFormulaEngine
	}

	// Reject invalid field names before anything else so a bad template
	// fails fast at boot instead of producing colliding keys (LGN-52).
	if err := validateFieldNames(fields, Path{}); err != nil {
		return nil, err
	}

	leafSchemas, leafPaths := collectLeaves(fields)
	if len(leafSchemas) == 0 {
		return newDAG(templateID, eng), nil
	}

	dag := newDAG(templateID, eng)
	compilationEnv := buildCompilationEnv(leafPaths)

	// Validate DependsOn paths first (before compilation) so we catch missing deps early
	if err := dag.buildAdjacencyList(leafSchemas, leafPaths); err != nil {
		return nil, err
	}

	if err := dag.compileNodeFormulas(leafSchemas, leafPaths, compilationEnv); err != nil {
		return nil, err
	}

	topoOrder, err := topologicalSort(dag.nodes, dag.adjacencyList)
	if err != nil {
		return nil, err
	}
	dag.topologicalOrder = topoOrder

	// Lock the DAG after construction to prevent any further mutation.
	dag.Lock()
	defer dag.Unlock()

	return dag, nil
}

// newDAG allocates and initializes a DAG with empty collections.
func newDAG(templateID string, eng *engine.FormulaEngine) *DAG {
	return &DAG{
		TemplateID:    templateID,
		nodes:         make(map[string]*Node),
		adjacencyList: make(map[string][]string),
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

// fieldNamePattern is the allowed shape for a single field-name segment.
// Restricting segments to ASCII letters and digits (starting with a letter)
// guarantees that both key encodings are injective: Path.String() joins with
// '.' and Path.EngineKey() joins with '_', and neither separator can appear
// inside a segment, so two distinct paths can never produce the same key.
// It also keeps every engine key a valid expr-lang identifier.
const fieldNamePattern = `^[A-Za-z][A-Za-z0-9]*$`

// fieldNameRe is the compiled form of fieldNamePattern.
var fieldNameRe = regexp.MustCompile(fieldNamePattern)

// fieldNameRule is the human-readable counterpart of fieldNamePattern used
// in FieldNameError messages.
const fieldNameRule = "must start with a letter and contain only ASCII letters " +
	"and digits ('.' and '_' are not allowed: they make DAG map keys collide with engine keys)"

// validateFieldNames walks every group and leaf name in the tree and returns
// a FieldNameError for the first name that violates fieldNamePattern.
// prefix carries the ancestors so the error reports the full path.
func validateFieldNames(group FieldGroup, prefix Path) error {
	for name, node := range group {
		currentPath := append(append(Path{}, prefix...), name)
		if !fieldNameRe.MatchString(name) {
			return ErrInvalidFieldName(currentPath.String(), fieldNameRule)
		}
		if node.IsGroup() {
			if err := validateFieldNames(node.Children, currentPath); err != nil {
				return err
			}
		}
	}
	return nil
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

// compileNodeFormulas compiles formulas for all leaf nodes and populates dag.nodes.
func (d *DAG) compileNodeFormulas(schemas []*FieldSchema, paths []Path, compilationEnv map[string]any) error {
	d.Lock()
	defer d.Unlock()

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

		d.nodes[key] = node
	}
	return nil
}

// buildAdjacencyList constructs the dependency graph from DependsOn declarations.
// Returns ErrInvalidDependency if any DependsOn references a non-existent field.
func (d *DAG) buildAdjacencyList(schemas []*FieldSchema, paths []Path) error {
	d.Lock()
	defer d.Unlock()

	// Build set of all valid node keys for validation
	validKeys := make(map[string]bool, len(paths))
	for _, path := range paths {
		validKeys[path.String()] = true
	}

	for i, schema := range schemas {
		key := paths[i].String()
		for _, depPath := range schema.DependsOn {
			depKey := Path(depPath).String()
			if !validKeys[depKey] {
				return ErrInvalidDependency(depPath, key)
			}
			d.adjacencyList[depKey] = append(d.adjacencyList[depKey], key)
		}
	}
	return nil
}

// topologicalSort performs cycle detection using DFS to identify cycle nodes.
// Returns topological order or ErrCycleDetected with the cycle path.
func topologicalSort(nodes map[string]*Node, adj map[string][]string) ([]string, error) {
	// Three-state DFS: 0=unvisited, 1=visiting, 2=visited
	state := make(map[string]int, len(nodes))
	var cyclePath []string

	var dfs func(string) bool
	dfs = func(node string) bool {
		state[node] = 1 // visiting
		cyclePath = append(cyclePath, node)

		for _, child := range adj[node] {
			if state[child] == 0 {
				if dfs(child) {
					return true
				}
			} else if state[child] == 1 {
				// Cycle found - extract cycle from cyclePath
				cycleStart := -1
				for i, n := range cyclePath {
					if n == child {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cyclePath = cyclePath[cycleStart:]
					return true
				}
			}
		}

		state[node] = 2 // visited
		cyclePath = cyclePath[:len(cyclePath)-1]
		return false
	}

	// Run DFS from all unvisited nodes
	for node := range nodes {
		if state[node] == 0 {
			if dfs(node) {
				return nil, ErrCycleDetected(cyclePath)
			}
		}
	}

	// No cycle - now run Kahn's for topological order
	inDegree := make(map[string]int, len(nodes))
	for key := range nodes {
		inDegree[key] = 0
	}
	for _, children := range adj {
		for _, child := range children {
			inDegree[child]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var topoOrder []string
	for head := 0; head < len(queue); head++ {
		curr := queue[head]
		topoOrder = append(topoOrder, curr)

		for _, child := range adj[curr] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if len(topoOrder) != len(nodes) {
		// Should not happen since DFS already checked, but fallback
		return nil, ErrInfiniteCycle
	}
	return topoOrder, nil
}

// Recalculate reapplies updates and recomputes derived fields in topological order.
// Returns a map of only the derived fields that actually changed (cascading effects).
// Returns an error if any formula evaluation fails, inputs are invalid, or update keys are unknown.
func (d *DAG) Recalculate(originalData, updatedData map[string]any) (map[string]any, error) {
	if d == nil {
		return nil, ErrNilDAG
	}
	if err := validateRecalculateInputs(originalData, updatedData); err != nil {
		return nil, err
	}

	// Validate all updatedData keys exist in DAG
	if err := d.validateUpdateKeys(updatedData); err != nil {
		return nil, err
	}

	d.RLock()
	defer d.RUnlock()

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

// validateUpdateKeys checks that all keys in updatedData exist in the DAG.
// A key is valid if it is a DAG node or a parent prefix of a DAG node
// (e.g., "stats" is valid if "stats.str" exists in the DAG).
func (d *DAG) validateUpdateKeys(updatedData map[string]any) error {
	d.RLock()
	defer d.RUnlock()

	for key := range updatedData {
		if d.HasNode(key) {
			continue // Direct DAG node
		}
		// Check if key is a prefix of any DAG node (e.g., "stats" for "stats.str")
		if !d.hasNodePrefix(key) {
			return ErrUnknownField(key)
		}
	}
	return nil
}

// hasNodePrefix returns true if any DAG node starts with the given key + ".".
func (d *DAG) hasNodePrefix(key string) bool {
	prefix := key + "."
	for nodeKey := range d.nodes {
		if strings.HasPrefix(nodeKey, prefix) {
			return true
		}
	}
	return false
}

// prepareData deep-copies original data and applies updates, returning the sheet data and engine environment.
func (d *DAG) prepareData(originalData, updatedData map[string]any) (map[string]any, map[string]any, error) {
	sheetData := maputils.DeepCopy(originalData)
	if err := maputils.ApplyUpdates(sheetData, updatedData); err != nil {
		return nil, nil, err
	}
	engineEnv := maputils.ToEngineEnv(sheetData)
	return sheetData, engineEnv, nil
}

// walkTopological evaluates nodes in topological order and returns changed derived fields.
// Must be called with the DAG's read lock already held.
func (d *DAG) walkTopological(sheetData, engineEnv map[string]any, needsRecalc map[string]bool) (map[string]any, error) {
	changedDerived := make(map[string]any)

	for _, nodeKey := range d.topologicalOrder {
		if !needsRecalc[nodeKey] {
			continue
		}

		node := d.nodes[nodeKey]
		propagated, err := d.processNode(node, sheetData, engineEnv, changedDerived)
		if err != nil {
			return nil, err
		}

		if propagated {
			d.propagateChanges(nodeKey, d.adjacencyList, needsRecalc)
		}
	}

	return changedDerived, nil
}

// propagateChanges marks descendants for recalculation when a node changes.
// Must be called with the DAG's read lock already held.
func (d *DAG) propagateChanges(nodeKey string, adj map[string][]string, needsRecalc map[string]bool) {
	for _, child := range adj[nodeKey] {
		needsRecalc[child] = true
	}
}

func validateRecalculateInputs(originalData, updatedData map[string]any) error {
	if originalData == nil {
		return ErrNilOriginalData
	}
	if updatedData == nil {
		return ErrNilUpdatedData
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
		return false, ErrNilNode
	}

	// Editable nodes always propagate (they are the source of changes)
	if !node.IsDerived() {
		return true, nil
	}

	newValue, err := d.formulaEngine.Evaluate(node.CompiledFormula, engineEnv)
	if err != nil {
		return false, fmt.Errorf(
			"%w for field '%s' (formula: %s): %w",
			ErrFormulaEval, node.MapKey(), node.FormulaString, err,
		)
	}

	oldValue, _ := node.Path.Get(sheetData)
	if !maputils.Equal(oldValue, newValue) {
		if err := node.Path.Set(sheetData, newValue); err != nil {
			return false, err
		}
		engineEnv[node.Path.EngineKey()] = newValue
		changedDerived[node.MapKey()] = newValue
		return true, nil
	}

	return false, nil
}
