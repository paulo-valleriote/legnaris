package template

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"legnaris/backend/internal/engine"
	"legnaris/backend/shared/maputils"

	"github.com/expr-lang/expr/vm"
)

type DAG struct {
	mu               sync.RWMutex
	templateID       string
	nodes            map[string]*Node
	adjacencyList    map[string][]string
	topologicalOrder []string
	exclusiveGroups  map[string][]string
	lists            map[string]ListSchema
	formulaEngine    *engine.FormulaEngine
}

type Node struct {
	Path            Path
	FormulaString   string
	compiledFormula *vm.Program
	IsEditable      bool
	isFlag          bool
	maxLength       int
}

func (n Node) IsDerived() bool {
	return n.compiledFormula != nil
}

func (n Node) MapKey() string {
	return n.Path.String()
}

func (n Node) checkFlagValue(value any) error {
	if !n.isFlag {
		return nil
	}
	number, ok := maputils.ToFloat64(value)
	if !ok || (number != 0 && number != 1) {
		return ErrFlagValue(n.MapKey())
	}
	return nil
}

func (n Node) checkTextLength(value any) error {
	if n.maxLength <= 0 {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return nil
	}
	if length := utf8.RuneCountInString(text); length > n.maxLength {
		return ErrTextTooLong(n.MapKey(), length, n.maxLength)
	}
	return nil
}

func (d *DAG) HasNode(key string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.nodes[key]
	return ok
}

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

func (d *DAG) GetChildren(key string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	children, ok := d.adjacencyList[key]
	if !ok {
		return nil
	}
	result := make([]string, len(children))
	copy(result, children)
	return result
}

func BuildDAG(templateID string, fields FieldGroup, eng *engine.FormulaEngine) (*DAG, error) {
	if eng == nil {
		return nil, ErrNilFormulaEngine
	}

	if err := validateFieldNames(fields, Path{}); err != nil {
		return nil, err
	}

	lists, err := collectLists(fields, Path{})
	if err != nil {
		return nil, err
	}

	leafSchemas, leafPaths := collectLeaves(fields)
	if len(leafSchemas) == 0 {
		dag := newDAG(templateID, eng)
		dag.lists = lists
		return dag, nil
	}

	dag := newDAG(templateID, eng)
	dag.lists = lists
	compilationEnv := buildCompilationEnv(leafPaths)

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

	return dag, nil
}

func newDAG(templateID string, eng *engine.FormulaEngine) *DAG {
	return &DAG{
		templateID:      templateID,
		nodes:           make(map[string]*Node),
		adjacencyList:   make(map[string][]string),
		exclusiveGroups: make(map[string][]string),
		lists:           make(map[string]ListSchema),
		formulaEngine:   eng,
	}
}

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

const fieldNamePattern = `^[A-Za-z][A-Za-z0-9]*$`

var fieldNameRe = regexp.MustCompile(fieldNamePattern)

const fieldNameRule = "must start with a letter and contain only ASCII letters " +
	"and digits ('.' and '_' are not allowed: they make DAG map keys collide with engine keys)"

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

func buildCompilationEnv(paths []Path) map[string]any {
	env := make(map[string]any, len(paths))
	for _, path := range paths {
		env[path.EngineKey()] = 0.0
	}
	return env
}

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
			isFlag:        schema.Flag,
		}

		if schema.MaxLength != nil {
			node.maxLength = *schema.MaxLength
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

func topologicalSort(nodes map[string]*Node, adj map[string][]string) ([]string, error) {
	if cycle := detectCycle(nodes, adj); cycle != nil {
		return nil, ErrCycleDetected(cycle)
	}
	return kahnTopoSort(nodes, adj)
}

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

func cycleFromPath(path []string, start string) []string {
	if i := slices.Index(path, start); i >= 0 {
		return slices.Clone(path[i:])
	}
	return slices.Clone(path)
}

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

func (d *DAG) Recalculate(originalData, updatedData map[string]any) (map[string]any, error) {
	if d == nil {
		return nil, ErrNilDAG
	}
	if err := validateRecalculateInputs(originalData, updatedData); err != nil {
		return nil, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.recalculateLocked(originalData, updatedData)
}

func (d *DAG) recalculateLocked(originalData, updatedData map[string]any) (map[string]any, error) {
	if err := d.validateUpdateKeysLocked(updatedData); err != nil {
		return nil, err
	}

	sheetData, engineEnv, err := d.prepareData(originalData, updatedData)
	if err != nil {
		return nil, err
	}

	needsRecalc, err := changedUpdates(originalData, updatedData)
	if err != nil {
		return nil, err
	}

	changedDerived, err := d.walkTopological(sheetData, engineEnv, needsRecalc)
	if err != nil {
		return nil, err
	}

	return changedDerived, nil
}

func changedUpdates(originalData, updatedData map[string]any) (map[string]bool, error) {
	changed := make(map[string]bool, len(updatedData))
	for key, newValue := range updatedData {
		if group, ok := newValue.(map[string]any); ok {
			flat, err := maputils.FlattenPathKeys(group)
			if err != nil {
				return nil, err
			}
			for leafKey, leafValue := range flat {
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
	return changed, nil
}

func hasChanged(originalData map[string]any, key string, newValue any) bool {
	oldValue, found := maputils.GetFlat(originalData, key)
	return !found || !maputils.Equal(oldValue, newValue)
}

func (d *DAG) validateUpdateKeysLocked(updatedData map[string]any) error {
	for key := range updatedData {
		if _, ok := d.nodes[key]; ok {
			continue
		}
		if !d.hasNodePrefix(key) {
			return ErrUnknownField(key)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(updatedData)) {
		node, ok := d.nodes[key]
		if !ok {
			continue
		}
		if err := node.checkFlagValue(updatedData[key]); err != nil {
			return err
		}
		if err := node.checkTextLength(updatedData[key]); err != nil {
			return err
		}
	}
	return nil
}

func (d *DAG) hasNodePrefix(key string) bool {
	prefix := key + "."
	for nodeKey := range d.nodes {
		if strings.HasPrefix(nodeKey, prefix) {
			return true
		}
	}
	return false
}

func (d *DAG) ValidateUpdateKeys(updates map[string]any) error {
	if d == nil {
		return ErrNilDAG
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.validateEditableLocked(updates)
}

func (d *DAG) validateEditableLocked(updates map[string]any) error {
	for key := range updates {
		node, ok := d.nodes[key]
		if !ok {
			return ErrUnknownField(key)
		}
		if !node.IsEditable {
			return ErrNotEditable(key)
		}
		if err := node.checkFlagValue(updates[key]); err != nil {
			return err
		}
	}
	return nil
}

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

func (d *DAG) normalizeExclusiveUpdatesLocked(updates map[string]any) (map[string]any, error) {
	var normalized map[string]any
	for _, group := range slices.Sorted(maps.Keys(d.exclusiveGroups)) {
		members := d.exclusiveGroups[group]
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

func exclusiveValueOn(v any) (on bool, valid bool) {
	f, ok := maputils.ToFloat64(v)
	if !ok {
		return false, false
	}
	return f != 0, true
}

func copyFlatUpdates(updates map[string]any) map[string]any {
	copied := make(map[string]any, len(updates))
	for key, value := range updates {
		copied[key] = value
	}
	return copied
}

func (d *DAG) prepareData(originalData, updatedData map[string]any) (map[string]any, map[string]any, error) {
	sheetData := maputils.DeepCopy(originalData)
	if err := maputils.ApplyUpdates(sheetData, updatedData); err != nil {
		return nil, nil, err
	}
	engineEnv, err := maputils.ToEngineEnv(sheetData)
	if err != nil {
		return nil, nil, err
	}
	return sheetData, engineEnv, nil
}

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
