package template

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"legnaris/backend/shared/maputils"

	"github.com/expr-lang/expr/vm"
)

type ResetAction struct {
	ID          string            `bson:"id" json:"id"`
	Label       string            `bson:"label" json:"label"`
	Assignments []ResetAssignment `bson:"assignments" json:"assignments"`
}

type ResetAssignment struct {
	Target     string `bson:"target" json:"target"`
	Expression string `bson:"expression" json:"expression"`
	Filter     string `bson:"filter,omitempty" json:"filter,omitempty"`
}

func (a ResetAction) Clone() ResetAction {
	cloned := a
	cloned.Assignments = slices.Clone(a.Assignments)
	return cloned
}

func cloneResetActions(actions []ResetAction) []ResetAction {
	if actions == nil {
		return nil
	}
	cloned := make([]ResetAction, len(actions))
	for i, action := range actions {
		cloned[i] = action.Clone()
	}
	return cloned
}

type compiledResetAction struct {
	id          string
	label       string
	assignments []compiledResetAssignment
}

type compiledResetAssignment struct {
	target     string
	targetPath Path
	expression string
	program    *vm.Program
	listPath   string
	field      string
	filter     string
}

type ResetCalculation struct {
	SaveCalculation
	ChangedKeys []string
}

var resetActionIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func (d *DAG) buildActions(actions []ResetAction, staticEnv map[string]any) error {
	d.actions = make(map[string]*compiledResetAction, len(actions))
	for _, action := range actions {
		if !resetActionIDRe.MatchString(action.ID) {
			return ErrInvalidResetAction(action.ID, "id must match ^[a-z][a-z0-9_]*$")
		}
		if _, exists := d.actions[action.ID]; exists {
			return ErrInvalidResetAction(action.ID, "duplicate action id")
		}
		compiled := &compiledResetAction{id: action.ID, label: action.Label}
		for _, assignment := range action.Assignments {
			compiledAssignment, err := d.compileResetAssignment(action.ID, assignment, staticEnv)
			if err != nil {
				return err
			}
			compiled.assignments = append(compiled.assignments, *compiledAssignment)
		}
		d.actions[action.ID] = compiled
	}
	return nil
}

func (d *DAG) compileResetAssignment(
	actionID string,
	assignment ResetAssignment,
	staticEnv map[string]any,
) (*compiledResetAssignment, error) {
	if assignment.Target == "" {
		return nil, ErrInvalidResetAction(actionID, "assignment target is empty")
	}
	if assignment.Expression == "" {
		return nil, ErrInvalidResetAction(actionID, "assignment expression on '"+assignment.Target+"' is empty")
	}

	target := Path(strings.Split(assignment.Target, "."))
	compiled := &compiledResetAssignment{
		target:     assignment.Target,
		targetPath: target,
		expression: assignment.Expression,
	}

	if listPath, list, field, ok := d.resetListTarget(target); ok {
		compiled.listPath = listPath
		compiled.field = field
		compiled.filter = assignment.Filter
		if err := validateResetListTarget(actionID, listPath, list, field); err != nil {
			return nil, err
		}
		if err := validateResetFilter(actionID, listPath, list, assignment); err != nil {
			return nil, err
		}
		program, err := d.formulaEngine.Compile(assignment.Expression, d.resetRecordCompileEnv(list, staticEnv))
		if err != nil {
			return nil, NewFormulaCompileError(assignment.Expression, resetCompileKey(actionID, assignment.Target), err)
		}
		compiled.program = program
		return compiled, nil
	}

	if assignment.Filter != "" {
		return nil, ErrInvalidResetAction(
			actionID,
			"filter '"+assignment.Filter+"' requires a list field target",
		)
	}

	node, ok := d.nodes[target.String()]
	if !ok {
		return nil, ErrInvalidResetAction(actionID, "target '"+assignment.Target+"' is not a field")
	}
	if node.IsDerived() {
		return nil, ErrInvalidResetAction(actionID, "target '"+assignment.Target+"' is derived")
	}
	if !node.IsEditable {
		return nil, ErrInvalidResetAction(actionID, "target '"+assignment.Target+"' is not editable")
	}
	if !node.isNumber {
		return nil, ErrInvalidResetAction(actionID, "target '"+assignment.Target+"' must be a number field")
	}

	program, err := d.formulaEngine.Compile(assignment.Expression, staticEnv)
	if err != nil {
		return nil, NewFormulaCompileError(assignment.Expression, resetCompileKey(actionID, assignment.Target), err)
	}
	compiled.program = program
	return compiled, nil
}

func (d *DAG) resetListTarget(target Path) (string, ListSchema, string, bool) {
	if len(target) < 2 {
		return "", ListSchema{}, "", false
	}
	listPath := target[:len(target)-1].String()
	list, ok := d.lists[listPath]
	if !ok {
		return "", ListSchema{}, "", false
	}
	return listPath, list, target[len(target)-1], true
}

func validateResetListTarget(actionID, listPath string, list ListSchema, field string) error {
	node, ok := list.Shape[field]
	if !ok || !node.IsLeaf() || node.Schema == nil {
		return ErrInvalidResetAction(actionID, "target '"+listPath+"."+field+"' is not a list field")
	}
	if node.Schema.Formula != "" {
		return ErrInvalidResetAction(actionID, "target '"+listPath+"."+field+"' is derived")
	}
	if !node.Schema.IsEditable {
		return ErrInvalidResetAction(actionID, "target '"+listPath+"."+field+"' is not editable")
	}
	if node.Schema.Type != fieldTypeNumber {
		return ErrInvalidResetAction(actionID, "target '"+listPath+"."+field+"' must be a number field")
	}
	return nil
}

func validateResetFilter(actionID, listPath string, list ListSchema, assignment ResetAssignment) error {
	if assignment.Filter == "" {
		return nil
	}
	node, ok := list.Shape[assignment.Filter]
	if !ok || !node.IsLeaf() || node.Schema == nil {
		return ErrInvalidResetAction(
			actionID,
			"filter '"+assignment.Filter+"' is not a field of list '"+listPath+"'",
		)
	}
	if node.Schema.Type != fieldTypeNumber {
		return ErrInvalidResetAction(actionID, "filter '"+assignment.Filter+"' must be a number field")
	}
	return nil
}

func (d *DAG) resetRecordCompileEnv(list ListSchema, staticEnv map[string]any) map[string]any {
	env := maps.Clone(staticEnv)
	env["record"] = recordMock(list)
	for name, node := range list.Shape {
		if node.IsLeaf() {
			env["record_"+name] = 0.0
		}
	}
	return env
}

func resetCompileKey(actionID, target string) string {
	return "reset." + actionID + "." + target
}

func (d *DAG) CalculateReset(actionID string, data map[string]any) (ResetCalculation, error) {
	if d == nil {
		return ResetCalculation{}, ErrNilDAG
	}
	if data == nil {
		return ResetCalculation{}, ErrNilOriginalData
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	action, ok := d.actions[actionID]
	if !ok {
		return ResetCalculation{}, ErrUnknownResetAction(actionID)
	}

	flat, changed, err := d.evaluateResetLocked(action, data)
	if err != nil {
		return ResetCalculation{}, err
	}

	static, records, err := d.splitRecordUpdatesLocked(flat)
	if err != nil {
		return ResetCalculation{}, err
	}
	calculation, err := d.saveLocked(data, static, records)
	if err != nil {
		return ResetCalculation{}, err
	}
	return ResetCalculation{SaveCalculation: calculation, ChangedKeys: changed}, nil
}

func (d *DAG) evaluateResetLocked(
	action *compiledResetAction,
	data map[string]any,
) (map[string]any, []string, error) {
	engineEnv, err := maputils.ToEngineEnv(data)
	if err != nil {
		return nil, nil, err
	}

	flat := make(map[string]any)
	changed := make([]string, 0, len(action.assignments))
	for _, assignment := range action.assignments {
		if assignment.listPath == "" {
			value, evaluateErr := d.formulaEngine.Evaluate(assignment.program, engineEnv)
			if evaluateErr != nil {
				return nil, nil, resetEvalErr(action.id, assignment, evaluateErr)
			}
			flat[assignment.target] = value
			old, found := maputils.Get(data, assignment.targetPath)
			if !found || !maputils.Equal(old, value) {
				changed = append(changed, assignment.target)
			}
			continue
		}

		listChanged, evaluateErr := d.evaluateResetListLocked(action.id, assignment, data, engineEnv, flat)
		if evaluateErr != nil {
			return nil, nil, evaluateErr
		}
		changed = append(changed, listChanged...)
	}
	return flat, uniqueSorted(changed), nil
}

func (d *DAG) evaluateResetListLocked(
	actionID string,
	assignment compiledResetAssignment,
	data, engineEnv map[string]any,
	flat map[string]any,
) ([]string, error) {
	list := d.lists[assignment.listPath]
	stored, found := maputils.Get(data, strings.Split(assignment.listPath, "."))
	if !found {
		return nil, nil
	}
	records, ok := stored.(map[string]any)
	if !ok {
		return nil, nil
	}

	changed := make([]string, 0, len(records))
	for _, recordID := range slices.Sorted(maps.Keys(records)) {
		record, ok := records[recordID].(map[string]any)
		if !ok || !resetFilterMatches(record, assignment.filter) {
			continue
		}
		value, err := d.formulaEngine.Evaluate(
			assignment.program, d.recordEngineEnv(engineEnv, list, record),
		)
		if err != nil {
			return nil, resetEvalErr(actionID, assignment, err)
		}
		key := assignment.listPath + "." + recordID + "." + assignment.field
		flat[key] = value
		if !maputils.Equal(record[assignment.field], value) {
			changed = append(changed, key)
		}
	}
	return changed, nil
}

func resetFilterMatches(record map[string]any, filter string) bool {
	if filter == "" {
		return true
	}
	value, ok := maputils.ToFloat64(record[filter])
	return ok && value != 0
}

func resetEvalErr(actionID string, assignment compiledResetAssignment, err error) error {
	return fmt.Errorf(
		"%w for reset action '%s' (expression: %s): %w",
		ErrFormulaEval, actionID, assignment.expression, err,
	)
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	slices.Sort(values)
	return slices.Compact(values)
}
