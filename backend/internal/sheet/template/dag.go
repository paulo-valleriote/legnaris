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
	mu                  sync.RWMutex
	templateID          string
	nodes               map[string]*Node
	adjacencyList       map[string][]string
	topologicalOrder    []string
	exclusiveGroups     map[string][]string
	lists               map[string]ListSchema
	recordNodes         map[string]*recordNode
	recordNodesByList   map[string][]string
	aggregates          map[string]AggregateSchema
	aggregatesByList    map[string][]string
	listExclusiveGroups map[string]map[string][]string
	actions             map[string]*compiledResetAction
	formulaEngine       *engine.FormulaEngine
	nameLeaf            string
}

type Node struct {
	Path            Path
	FormulaString   string
	compiledFormula *vm.Program
	IsEditable      bool
	isFlag          bool
	isNumber        bool
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

func (d *DAG) NameFrom(values map[string]any) string {
	if d.nameLeaf == "" {
		return ""
	}
	raw, ok := Path(strings.Split(d.nameLeaf, ".")).Get(values)
	if !ok {
		return ""
	}
	name, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(name)
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
	return BuildDAGWithActions(templateID, fields, nil, eng)
}

func BuildDAGWithActions(
	templateID string,
	fields FieldGroup,
	actions []ResetAction,
	eng *engine.FormulaEngine,
) (*DAG, error) {
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

	dag := newDAG(templateID, eng)
	dag.lists = lists

	leafSchemas, leafPaths := collectLeaves(fields)
	compilationEnv := buildCompilationEnv(leafPaths)

	if err := dag.buildAdjacencyList(leafSchemas, leafPaths); err != nil {
		return nil, err
	}

	if err := dag.compileNodeFormulas(leafSchemas, leafPaths, compilationEnv); err != nil {
		return nil, err
	}

	dag.exclusiveGroups = buildExclusiveGroups(leafSchemas, leafPaths)

	if err := dag.buildAggregates(leafSchemas, leafPaths, lists); err != nil {
		return nil, err
	}

	if err := dag.buildRecordNodes(compilationEnv); err != nil {
		return nil, err
	}

	if err := dag.buildActions(actions, compilationEnv); err != nil {
		return nil, err
	}

	topoOrder, err := topologicalSort(dag.graphKeys(), dag.adjacencyList)
	if err != nil {
		return nil, err
	}
	dag.topologicalOrder = topoOrder

	return dag, nil
}

func newDAG(templateID string, eng *engine.FormulaEngine) *DAG {
	return &DAG{
		templateID:          templateID,
		nodes:               make(map[string]*Node),
		adjacencyList:       make(map[string][]string),
		exclusiveGroups:     make(map[string][]string),
		lists:               make(map[string]ListSchema),
		recordNodes:         make(map[string]*recordNode),
		recordNodesByList:   make(map[string][]string),
		aggregates:          make(map[string]AggregateSchema),
		aggregatesByList:    make(map[string][]string),
		listExclusiveGroups: make(map[string]map[string][]string),
		actions:             make(map[string]*compiledResetAction),
		formulaEngine:       eng,
	}
}

func (d *DAG) graphKeys() map[string]bool {
	keys := make(map[string]bool, len(d.nodes)+len(d.recordNodes))
	for key := range d.nodes {
		keys[key] = true
	}
	for key := range d.recordNodes {
		keys[key] = true
	}
	return keys
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

const reservedRecordField = "record"

const recordReservedRule = "'record' is reserved for record-scoped formulas"

func validateFieldNames(group FieldGroup, prefix Path) error {
	for name, node := range group {
		currentPath := append(append(Path{}, prefix...), name)
		if !fieldNameRe.MatchString(name) {
			return ErrInvalidFieldName(currentPath.String(), fieldNameRule)
		}
		if len(prefix) == 0 && name == reservedRecordField {
			return ErrInvalidFieldName(name, recordReservedRule)
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
			isNumber:      schema.Type == fieldTypeNumber,
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

func topologicalSort(nodes map[string]bool, adj map[string][]string) ([]string, error) {
	if cycle := detectCycleKeys(nodes, adj); cycle != nil {
		return nil, ErrCycleDetected(cycle)
	}
	return kahnTopoSort(nodes, adj)
}

func detectCycle(nodes map[string]*Node, adj map[string][]string) []string {
	keys := make(map[string]bool, len(nodes))
	for key := range nodes {
		keys[key] = true
	}
	return detectCycleKeys(keys, adj)
}

func detectCycleKeys(nodes map[string]bool, adj map[string][]string) []string {
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

func kahnTopoSort(nodes map[string]bool, adj map[string][]string) ([]string, error) {
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

func initInDegree(nodes map[string]bool, adj map[string][]string) map[string]int {
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
	return d.recalculateFromLocked(originalData, originalData, updatedData, nil)
}

func (d *DAG) recalculateFromLocked(
	originalData, baseData, updatedData map[string]any,
	touchedLists []string,
) (map[string]any, error) {
	sheetData, engineEnv, err := d.prepareData(baseData, updatedData)
	if err != nil {
		return nil, err
	}

	needsRecalc, err := changedUpdates(originalData, updatedData)
	if err != nil {
		return nil, err
	}
	d.markTouchedLists(needsRecalc, touchedLists)

	return d.walkTopological(sheetData, engineEnv, needsRecalc)
}

func (d *DAG) markTouchedLists(needsRecalc map[string]bool, touchedLists []string) {
	for _, listPath := range touchedLists {
		for _, key := range d.recordNodesByList[listPath] {
			needsRecalc[key] = true
		}
		for _, key := range d.aggregatesByList[listPath] {
			needsRecalc[key] = true
		}
	}
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
		if err := checkNodeValue(node, updatedData[key]); err != nil {
			return err
		}
	}
	return nil
}

func checkNodeValue(node *Node, value any) error {
	if err := node.checkFlagValue(value); err != nil {
		return err
	}
	return node.checkTextLength(value)
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
	for _, key := range slices.Sorted(maps.Keys(updates)) {
		node, ok := d.nodes[key]
		if !ok {
			return ErrUnknownField(key)
		}
		if !node.IsEditable {
			return ErrNotEditable(key)
		}
		if err := checkNodeValue(node, updates[key]); err != nil {
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

	cascading, err = d.recalculateFromLocked(originalData, originalData, normalized, nil)
	if err != nil {
		return nil, nil, err
	}
	return normalized, cascading, nil
}

type SaveCalculation struct {
	Normalized    map[string]any
	RecordUpdates []RecordUpdate
	Merged        map[string]any
	Cascade       map[string]any
}

func (d *DAG) CalculateSave(originalData, flatUpdates map[string]any) (SaveCalculation, error) {
	if d == nil {
		return SaveCalculation{}, ErrNilDAG
	}
	if err := validateRecalculateInputs(originalData, flatUpdates); err != nil {
		return SaveCalculation{}, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	static, records, err := d.splitRecordUpdatesLocked(flatUpdates)
	if err != nil {
		return SaveCalculation{}, err
	}
	return d.saveLocked(originalData, static, records)
}

func (d *DAG) NormalizeAndRecalculateWithRecords(
	originalData, updatedData map[string]any,
	records []RecordUpdate,
) (SaveCalculation, error) {
	if d == nil {
		return SaveCalculation{}, ErrNilDAG
	}
	if err := validateRecalculateInputs(originalData, updatedData); err != nil {
		return SaveCalculation{}, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.saveLocked(originalData, updatedData, records)
}

func (d *DAG) saveLocked(
	originalData, updatedData map[string]any,
	records []RecordUpdate,
) (SaveCalculation, error) {
	if err := d.validateEditableLocked(updatedData); err != nil {
		return SaveCalculation{}, err
	}

	normalized, err := d.normalizeExclusiveUpdatesLocked(updatedData)
	if err != nil {
		return SaveCalculation{}, err
	}

	merged := maputils.DeepCopy(originalData)
	if err := d.applyRecordUpdatesLocked(merged, records); err != nil {
		return SaveCalculation{}, err
	}

	cascade, err := d.recalculateFromLocked(originalData, merged, normalized, touchedLists(records))
	if err != nil {
		return SaveCalculation{}, err
	}
	if err := maputils.ApplyUpdates(merged, normalized); err != nil {
		return SaveCalculation{}, err
	}
	if err := maputils.ApplyUpdates(merged, cascade); err != nil {
		return SaveCalculation{}, err
	}

	return SaveCalculation{
		Normalized:    normalized,
		RecordUpdates: records,
		Merged:        merged,
		Cascade:       cascade,
	}, nil
}

func touchedLists(records []RecordUpdate) []string {
	seen := make(map[string]bool, len(records))
	lists := make([]string, 0, len(records))
	for _, record := range records {
		if seen[record.ListPath] {
			continue
		}
		seen[record.ListPath] = true
		lists = append(lists, record.ListPath)
	}
	return lists
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

func (d *DAG) prepareData(baseData, updatedData map[string]any) (map[string]any, map[string]any, error) {
	sheetData := maputils.DeepCopy(baseData)
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

		var propagated bool
		var err error
		if record, ok := d.recordNodes[nodeKey]; ok {
			propagated, err = d.processRecordNode(record, sheetData, engineEnv, changedDerived)
		} else {
			node := d.nodes[nodeKey]
			propagated, err = d.processNode(node, sheetData, engineEnv, changedDerived)
		}
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

	if aggregate, ok := d.aggregates[node.MapKey()]; ok {
		return d.processAggregate(node, aggregate, sheetData, engineEnv, changedDerived)
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

func (d *DAG) processAggregate(
	node *Node,
	aggregate AggregateSchema,
	sheetData, engineEnv map[string]any,
	changedDerived map[string]any,
) (bool, error) {
	newValue := d.aggregateValue(aggregate, sheetData)
	oldValue, _ := node.Path.Get(sheetData)
	if maputils.Equal(oldValue, newValue) {
		return false, nil
	}
	if err := node.Path.Set(sheetData, newValue); err != nil {
		return false, err
	}
	engineEnv[node.Path.EngineKey()] = newValue
	changedDerived[node.MapKey()] = newValue
	return true, nil
}
