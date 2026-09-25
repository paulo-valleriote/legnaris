package template

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"legnaris/internal/engine"
	"legnaris/shared/maputils"

	"github.com/expr-lang/expr/vm"
)

// DAG represents a directed acyclic graph of computed fields for a template.
// Nodes are evaluated in topological order to ensure dependencies are resolved
// before dependents.
//
// The graph is built once per template at boot and never mutates afterwards:
// BuildDAG is the only writer (a single goroutine, running before the graph is
// published to anyone else) and no exported method writes graph state, so
// concurrent readers never race (LGN-54). mu keeps that discipline explicit for
// the read path and serialises any write that ever reaches the graph.
//
// The guard is an unexported mutex on purpose. An exported embedded RWMutex
// would let any caller Lock the DAG (starving every reader) or Unlock it
// (panicking), and taking the read lock twice — as validateUpdateKeys used to
// do through HasNode — deadlocks whenever a writer is queued between the two
// acquisitions, which sync.RWMutex explicitly forbids.
type DAG struct {
	// mu guards nodes, adjacencyList and topologicalOrder. Read them through
	// the accessors (HasNode, GetNode, GetChildren) or while holding mu
	// explicitly in Recalculate — never acquire it twice.
	mu               sync.RWMutex
	templateID       string
	nodes            map[string]*Node
	adjacencyList    map[string][]string
	topologicalOrder []string
	formulaEngine    *engine.FormulaEngine
}

// Node represents a single field in the DAG.
//
// Node is handed out by value from GetNode, and DAG keeps its own private
// instances: a caller that mutates a returned copy changes nothing but its
// own local variable (LGN-54). The compiled program stays unexported so no
// caller outside this package can reach the state every Recalculate reads.
type Node struct {
	Path            Path
	FormulaString   string
	compiledFormula *vm.Program
	IsEditable      bool
}

// IsDerived returns true if this node has a formula (computed field).
func (n Node) IsDerived() bool {
	return n.compiledFormula != nil
}

// MapKey returns the flat path key used in DAG maps (e.g., "attributes.str").
func (n Node) MapKey() string {
	return n.Path.String()
}

// HasNode returns true if a node with the given key exists in the DAG.
// It takes the read lock itself, so it must not be called while the caller
// already holds it — see Recalculate for the single-lock read path.
func (d *DAG) HasNode(key string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.nodes[key]
	return ok
}

// GetNode returns a copy of the node for the given key and whether it was
// found. The Path slice is cloned too, so the returned value shares no
// memory with the DAG and cannot be used to mutate it.
// It takes the read lock itself, so it must not be called while the caller
// already holds it — see Recalculate for the single-lock read path.
func (d *DAG) GetNode(key string) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	node, ok := d.nodes[key]
	if !ok {
		return Node{}, false
	}
	copied := *node
	copied.Path = slices.Clone(node.Path)
	return copied, true
}

// GetChildren returns the list of child keys for the given node key.
// Returns nil if the key has no children.
func (d *DAG) GetChildren(key string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	children, ok := d.adjacencyList[key]
	if !ok {
		return nil
	}
	// Copy while the read lock is still held: the caller must get a fresh
	// allocation, never the internal adjacency slice (LGN-54).
	result := make([]string, len(children))
	copy(result, children)
	return result
}

// BuildDAG constructs a computation DAG from a template's field schemas.
// Returns an error if any formula fails to compile, a dependency cycle is detected,
// or a DependsOn references a non-existent field.
// Construction runs single-threaded at boot, and the graph is immutable from
// the moment BuildDAG returns: nothing writes it afterwards (LGN-54).
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

	// No lock is held here: construction is single-threaded and the graph is
	// immutable from now on, so there is nothing left to protect (LGN-54).
	return dag, nil
}

// newDAG allocates and initializes a DAG with empty collections.
func newDAG(templateID string, eng *engine.FormulaEngine) *DAG {
	return &DAG{
		templateID:    templateID,
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
	d.mu.Lock()
	defer d.mu.Unlock()

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
			node.compiledFormula = program
		}

		d.nodes[key] = node
	}
	return nil
}

// buildAdjacencyList constructs the dependency graph from DependsOn declarations.
// Returns ErrInvalidDependency if any DependsOn references a non-existent field.
func (d *DAG) buildAdjacencyList(schemas []*FieldSchema, paths []Path) error {
	d.mu.Lock()
	defer d.mu.Unlock()

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

	// One read-lock acquisition covers key validation and evaluation.
	// The read lock must never be taken recursively: if a writer queued a Lock
	// between two RLocks, the second acquisition would wait for the writer
	// while the writer waits for the first RLock — a deadlock sync.RWMutex
	// explicitly warns about (LGN-54).
	d.mu.RLock()
	defer d.mu.RUnlock()

	if err := d.validateUpdateKeysLocked(updatedData); err != nil {
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

// validateUpdateKeysLocked checks that all keys in updatedData exist in the DAG.
// A key is valid if it is a DAG node or a parent prefix of a DAG node
// (e.g., "stats" is valid if "stats.str" exists in the DAG).
// It reads the graph directly and must be called with mu already held by the
// caller: it must not take the lock itself, or it would nest a read lock inside
// the one Recalculate already holds.
func (d *DAG) validateUpdateKeysLocked(updatedData map[string]any) error {
	for key := range updatedData {
		if _, ok := d.nodes[key]; ok {
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
// Must be called with mu already held.
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

	newValue, err := d.formulaEngine.Evaluate(node.compiledFormula, engineEnv)
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
