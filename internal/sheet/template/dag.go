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
	// mu guards nodes, adjacencyList, topologicalOrder and exclusiveGroups.
	// Read them through the accessors (HasNode, GetNode, GetChildren) or while
	// holding mu explicitly in Recalculate — never acquire it twice.
	mu               sync.RWMutex
	templateID       string
	nodes            map[string]*Node
	adjacencyList    map[string][]string
	topologicalOrder []string
	exclusiveGroups  map[string][]string
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

	dag.exclusiveGroups = buildExclusiveGroups(leafSchemas, leafPaths)

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
		templateID:      templateID,
		nodes:           make(map[string]*Node),
		adjacencyList:   make(map[string][]string),
		exclusiveGroups: make(map[string][]string),
		formulaEngine:   eng,
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

// buildExclusiveGroups indexes editable leaves by their exclusive group id.
// Only editable leaves join a group, and a group with fewer than two members is
// dropped: a single editable member has nothing to exclude (EXC-08), and a
// derived leaf is computed, never chosen. Members are sorted so conflict errors
// and tests are deterministic.
func buildExclusiveGroups(schemas []*FieldSchema, paths []Path) map[string][]string {
	groups := make(map[string][]string)
	for i, schema := range schemas {
		if !schema.IsEditable || schema.ExclusiveGroup == "" {
			continue
		}
		groups[schema.ExclusiveGroup] = append(groups[schema.ExclusiveGroup], paths[i].String())
	}
	for id, members := range groups {
		if len(members) < 2 {
			delete(groups, id)
			continue
		}
		slices.Sort(members)
	}
	return groups
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
				return NewFormulaCompileError(schema.Formula, key, err)
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

const (
	nodeUnvisited = iota
	nodeVisiting
	nodeVisited
)

// topologicalSort detects cycles and, when none are found, returns a
// topological ordering of the nodes.
func topologicalSort(nodes map[string]*Node, adj map[string][]string) ([]string, error) {
	if cycle := detectCycle(nodes, adj); cycle != nil {
		return nil, ErrCycleDetected(cycle)
	}
	return kahnTopoSort(nodes, adj)
}

// detectCycle performs a DFS over every unvisited root and returns the
// first cycle it encounters, or nil when the graph is acyclic.
func detectCycle(nodes map[string]*Node, adj map[string][]string) []string {
	state := make(map[string]int, len(nodes))
	for node := range nodes {
		if state[node] != nodeUnvisited {
			continue
		}
		if cycle := findCycle(node, adj, state, nil); cycle != nil {
			return cycle
		}
	}
	return nil
}

// findCycle is the recursive DFS helper. `path` is local to this
// traversal: it grows as we descend and carries no state from sibling
// branches, so cycle extraction is never affected by a different root's
// walk.
func findCycle(node string, adj map[string][]string, state map[string]int, path []string) []string {
	state[node] = nodeVisiting
	path = append(path, node)

	for _, child := range adj[node] {
		switch state[child] {
		case nodeUnvisited:
			if cycle := findCycle(child, adj, state, path); cycle != nil {
				return cycle
			}
		case nodeVisiting:
			return cycleFromPath(path, child)
		}
	}

	state[node] = nodeVisited
	return nil
}

// cycleFromPath returns the slice of `path` starting at the first
// occurrence of `start`, cloned so that recursive appends can never
// mutate the reported cycle.
func cycleFromPath(path []string, start string) []string {
	if i := slices.Index(path, start); i >= 0 {
		return slices.Clone(path[i:])
	}
	return slices.Clone(path)
}

// kahnTopoSort returns a topological ordering using Kahn's algorithm.
// The graph must be acyclic; a cycle triggers ErrCycleDetected as a
// defensive fallback with the nodes that could not be ordered.
func kahnTopoSort(nodes map[string]*Node, adj map[string][]string) ([]string, error) {
	inDegree := initInDegree(nodes, adj)

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	topoOrder := make([]string, 0, len(nodes))
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
		inTopo := make(map[string]bool, len(topoOrder))
		for _, n := range topoOrder {
			inTopo[n] = true
		}
		var missing []string
		for n := range nodes {
			if !inTopo[n] {
				missing = append(missing, n)
			}
		}
		return nil, ErrCycleDetected(missing)
	}
	return topoOrder, nil
}

// initInDegree computes the in-degree of every node.
func initInDegree(nodes map[string]*Node, adj map[string][]string) map[string]int {
	inDegree := make(map[string]int, len(nodes))
	for key := range nodes {
		inDegree[key] = 0
	}
	for _, children := range adj {
		for _, child := range children {
			inDegree[child]++
		}
	}
	return inDegree
}

// Recalculate reapplies updates and recomputes derived fields in topological order.
// Returns a map of only the derived fields that actually changed by more than epsilon
// (cascading effects). "Changed" means the new value differs from the old by more
// than the epsilon tolerance used in maputils.Equal (1e-9 relative, scalar values only).
// updatedData must carry flat path keys (e.g. "stats.str"): a group value is out of
// contract because prepareData replaces the whole group instead of merging its leaves,
// dropping the siblings the payload left out — the sheet service flattens payloads
// before calling.
// Returns an error if any formula evaluation fails, inputs are invalid, or a flat
// update key is neither a DAG node nor a prefix of one. Group payloads skip that
// check (their leaves are never validated), which is another reason they stay out
// of contract. Editability is deliberately not enforced here (Create seeds derived
// fields too); callers that accept client payloads validate them with
// ValidateUpdateKeys first.
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

	return d.recalculateLocked(originalData, updatedData)
}

// recalculateLocked is the lock-free core of Recalculate. It must be called
// with mu already held.
func (d *DAG) recalculateLocked(originalData, updatedData map[string]any) (map[string]any, error) {
	if err := d.validateUpdateKeysLocked(updatedData); err != nil {
		return nil, err
	}

	sheetData, engineEnv, err := d.prepareData(originalData, updatedData)
	if err != nil {
		return nil, err
	}

	// Updates that do not really change the original data never reach the
	// recalculation set, so floating-point rounding or type coercion
	// (e.g., int vs float64) cannot start a spurious cascade.
	needsRecalc := changedUpdates(originalData, updatedData)

	changedDerived, err := d.walkTopological(sheetData, engineEnv, needsRecalc)
	if err != nil {
		return nil, err
	}

	return changedDerived, nil
}

// changedUpdates returns the update keys that really change originalData, as the
// initial set of nodes to recalculate: entries the change check drops must not
// start a cascade at all.
// Keys are leaf path keys; group-level entries (a nested map value, e.g.
// {"stats": {"str": 16}}) are expanded to their leaves before comparing, so a
// stray group payload still reaches DAG node keys instead of being silently
// dropped by the change check — the supported input stays flat (see Recalculate).
func changedUpdates(originalData, updatedData map[string]any) map[string]bool {
	changed := make(map[string]bool, len(updatedData))
	for key, newValue := range updatedData {
		if group, ok := newValue.(map[string]any); ok {
			for leafKey, leafValue := range maputils.FlattenPathKeys(group) {
				fullKey := key + "." + leafKey
				if hasChanged(originalData, fullKey, leafValue) {
					changed[fullKey] = true
				}
			}
			continue
		}
		if hasChanged(originalData, key, newValue) {
			changed[key] = true
		}
	}
	return changed
}

// hasChanged reports whether the value stored under key differs from newValue
// by more than the maputils.Equal tolerance.
// A key that is absent from originalData counts as a change.
// It is the single gate deciding whether an update entry may start a cascade.
func hasChanged(originalData map[string]any, key string, newValue any) bool {
	oldValue, found := maputils.GetFlat(originalData, key)
	return !found || !maputils.Equal(oldValue, newValue)
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
			continue
		}
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

// ValidateUpdateKeys checks that every key in updates is a leaf node the caller
// is allowed to write, returning UnknownFieldError for a key the graph does not
// know and NotEditableError for a field the DAG computes itself. It is the
// update contract the sheet service enforces before calling Recalculate, and
// the single place where editability is decided (LGN-66).
//
// Editability cannot be enforced inside Recalculate: Create seeds a new sheet
// with every field of the template, derived ones included, and that payload
// must pass validation. Recalculate's own key check therefore stays structural
// (a key must exist, leaf or group prefix), while this one is strict: keys are
// matched against leaf nodes only, so a group prefix such as "stats" is
// rejected as unknown instead of being applied — maputils.Set would replace the
// whole group with the scalar and drop its siblings.
//
// It takes the read lock itself, exactly once, so it must not be called while
// the caller already holds it (a queued writer turns a second RLock into a
// deadlock, see Recalculate).
func (d *DAG) ValidateUpdateKeys(updates map[string]any) error {
	if d == nil {
		return ErrNilDAG
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.validateEditableLocked(updates)
}

// validateEditableLocked is the lock-free core of ValidateUpdateKeys. It must be
// called with mu already held.
func (d *DAG) validateEditableLocked(updates map[string]any) error {
	for key := range updates {
		node, ok := d.nodes[key]
		if !ok {
			return ErrUnknownField(key)
		}
		if !node.IsEditable {
			return ErrNotEditable(key)
		}
	}
	return nil
}

// NormalizeAndRecalculate validates editability, collapses each touched
// exclusive group to at most one non-zero member, and recomputes derived fields
// — all under a single read-lock acquisition. It is the PATCH path's entry
// point: ValidateUpdateKeys + NormalizeExclusiveUpdates + Recalculate would
// otherwise take the read lock three times per request, and the codebase's
// discipline is one acquisition per operation (LGN-54).
//
// It returns the normalized update map, which the caller applies to the stored
// data, and the cascading derived effects. Editability is enforced here like
// ValidateUpdateKeys; the structural key check stays inside recalculateLocked.
func (d *DAG) NormalizeAndRecalculate(originalData, updatedData map[string]any) (
	normalized map[string]any, cascading map[string]any, err error,
) {
	if d == nil {
		return nil, nil, ErrNilDAG
	}
	if err := validateRecalculateInputs(originalData, updatedData); err != nil {
		return nil, nil, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	if err := d.validateEditableLocked(updatedData); err != nil {
		return nil, nil, err
	}

	normalized, err = d.normalizeExclusiveUpdatesLocked(updatedData)
	if err != nil {
		return nil, nil, err
	}

	cascading, err = d.recalculateLocked(originalData, normalized)
	if err != nil {
		return nil, nil, err
	}
	return normalized, cascading, nil
}

// NormalizeExclusiveUpdates returns a copy of updates with each touched
// exclusive group collapsed to at most one non-zero member. A payload that
// turns on two or more members of the same group is rejected with an
// ExclusiveGroupConflictError; a member carrying a non-numeric value is
// rejected with an ExclusiveValueTypeError; a payload that touches no exclusive
// group — or only turns members off — is returned unchanged (same map, no
// allocation).
//
// It takes the read lock itself, exactly once, like ValidateUpdateKeys, so it
// must not be called while the caller already holds it (a queued writer turns a
// second RLock into a deadlock, see Recalculate).
func (d *DAG) NormalizeExclusiveUpdates(updates map[string]any) (map[string]any, error) {
	if d == nil {
		return nil, ErrNilDAG
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	normalized, err := d.normalizeExclusiveUpdatesLocked(updates)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

// normalizeExclusiveUpdatesLocked is the lock-free core of
// NormalizeExclusiveUpdates. It must be called with mu already held.
func (d *DAG) normalizeExclusiveUpdatesLocked(updates map[string]any) (map[string]any, error) {
	var normalized map[string]any
	for group, members := range d.exclusiveGroups {
		var on []string
		for _, key := range members {
			value, present := updates[key]
			if !present {
				continue
			}
			active, valid := exclusiveValueOn(value)
			if !valid {
				return nil, ErrExclusiveValueType(group, key)
			}
			if active {
				on = append(on, key)
			}
		}

		switch len(on) {
		case 0:
			// Turning a member off is allowed: the group may end with no
			// active member, so the siblings stay untouched (EXC-06).
			continue
		case 1:
			if normalized == nil {
				normalized = copyFlatUpdates(updates)
			}
			for _, key := range members {
				if key == on[0] {
					continue
				}
				if _, present := updates[key]; present {
					continue
				}
				normalized[key] = 0.0
			}
		default:
			return nil, ErrExclusiveGroupConflict(group, on)
		}
	}

	if normalized == nil {
		return updates, nil
	}
	return normalized, nil
}

// exclusiveValueOn reports whether v is an active exclusive member and whether
// v is a value the group accepts at all. Exclusive members are numeric 0/1
// proxies, so every numeric type counts (non-zero is active) and every other
// value — a string, a bool, nil, a map — is rejected instead of being silently
// read as "off", which would let a payload bypass the at-most-one rule.
func exclusiveValueOn(v any) (on bool, valid bool) {
	f, ok := maputils.ToFloat64(v)
	if !ok {
		return false, false
	}
	return f != 0, true
}

// copyFlatUpdates shallow-copies a flat update map so normalization can add
// sibling keys without mutating the caller's payload.
func copyFlatUpdates(updates map[string]any) map[string]any {
	copied := make(map[string]any, len(updates))
	for key, value := range updates {
		copied[key] = value
	}
	return copied
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

// processNode evaluates a derived node if needed, updates both nested and flat envs,
// and returns whether the change should propagate to dependents.
// Editable nodes always propagate: they are the source of changes, and the change
// filter in Recalculate has already dropped update entries that changed nothing.
// Returns an error if formula evaluation fails, wrapped with field context.
func (d *DAG) processNode(node *Node, sheetData, engineEnv map[string]any, changedDerived map[string]any) (bool, error) {
	if node == nil {
		return false, ErrNilNode
	}

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
