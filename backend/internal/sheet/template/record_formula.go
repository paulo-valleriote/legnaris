package template

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"legnaris/backend/shared/maputils"

	"github.com/expr-lang/expr/vm"
)

type recordNode struct {
	ListPath        string
	Field           string
	FormulaString   string
	compiledFormula *vm.Program
}

func (n *recordNode) key() string {
	return n.ListPath + "." + n.Field
}

func (d *DAG) buildRecordNodes(staticEnv map[string]any) error {
	for _, listPath := range slices.Sorted(maps.Keys(d.lists)) {
		list := d.lists[listPath]
		d.listExclusiveGroups[listPath] = buildListExclusiveGroups(list)
		for _, field := range slices.Sorted(maps.Keys(list.Shape)) {
			if err := d.buildRecordNode(listPath, list, field, staticEnv); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DAG) buildRecordNode(
	listPath string,
	list ListSchema,
	field string,
	staticEnv map[string]any,
) error {
	node := list.Shape[field]
	if !node.IsLeaf() || node.Schema == nil {
		return nil
	}

	record := &recordNode{ListPath: listPath, Field: field, FormulaString: node.Schema.Formula}
	key := record.key()

	env := maps.Clone(staticEnv)
	for _, dependency := range node.Schema.DependsOn {
		if err := d.addRecordDependency(listPath, list, key, dependency, env); err != nil {
			return err
		}
	}

	if node.Schema.Formula != "" {
		program, err := d.formulaEngine.Compile(node.Schema.Formula, env)
		if err != nil {
			return NewFormulaCompileError(node.Schema.Formula, key, err)
		}
		record.compiledFormula = program
	}

	d.recordNodes[key] = record
	d.recordNodesByList[listPath] = append(d.recordNodesByList[listPath], key)
	return nil
}

func (d *DAG) addRecordDependency(
	listPath string,
	list ListSchema,
	target string,
	dependency []string,
	env map[string]any,
) error {
	switch len(dependency) {
	case 0:
		return nil
	case 1:
		field := dependency[0]
		if _, ok := list.Shape[field]; ok {
			env["record_"+field] = 0.0
			source := listPath + "." + field
			d.adjacencyList[source] = append(d.adjacencyList[source], target)
			return nil
		}
		if _, ok := d.nodes[field]; ok {
			d.adjacencyList[field] = append(d.adjacencyList[field], target)
			return nil
		}
		return ErrUnknownRecordField(listPath, field)
	default:
		source := Path(dependency).String()
		if _, ok := d.nodes[source]; !ok {
			return ErrInvalidDependency(dependency, target)
		}
		d.adjacencyList[source] = append(d.adjacencyList[source], target)
		return nil
	}
}

func (d *DAG) processRecordNode(
	node *recordNode,
	sheetData, engineEnv map[string]any,
	changedDerived map[string]any,
) (bool, error) {
	if node.compiledFormula == nil {
		return true, nil
	}

	list := d.lists[node.ListPath]
	stored, found := maputils.Get(sheetData, strings.Split(node.ListPath, "."))
	if !found {
		return false, nil
	}
	records, ok := stored.(map[string]any)
	if !ok {
		return false, nil
	}

	changed := false
	for _, recordID := range slices.Sorted(maps.Keys(records)) {
		record, ok := records[recordID].(map[string]any)
		if !ok {
			continue
		}
		value, err := d.formulaEngine.Evaluate(
			node.compiledFormula, d.recordEngineEnv(engineEnv, list, record),
		)
		if err != nil {
			key := node.ListPath + "." + recordID + "." + node.Field
			return false, fmt.Errorf(
				"%w for field '%s' (formula: %s): %w",
				ErrFormulaEval, key, node.FormulaString, err,
			)
		}
		if maputils.Equal(record[node.Field], value) {
			continue
		}
		record[node.Field] = value
		changedDerived[node.ListPath+"."+recordID+"."+node.Field] = value
		changed = true
	}
	return changed, nil
}

func (d *DAG) recordEngineEnv(base map[string]any, list ListSchema, record map[string]any) map[string]any {
	env := make(map[string]any, len(d.nodes)+len(list.Shape))
	for _, node := range d.nodes {
		key := node.Path.EngineKey()
		env[key] = base[key]
	}
	for name, node := range list.Shape {
		if !node.IsLeaf() {
			continue
		}
		env["record_"+name] = record[name]
	}
	return env
}

func buildListExclusiveGroups(list ListSchema) map[string][]string {
	groups := make(map[string][]string)
	for name, node := range list.Shape {
		if !node.IsLeaf() || !node.Schema.IsEditable || node.Schema.ExclusiveGroup == "" {
			continue
		}
		groups[node.Schema.ExclusiveGroup] = append(groups[node.Schema.ExclusiveGroup], name)
	}
	for group, members := range groups {
		if len(members) < 2 {
			delete(groups, group)
			continue
		}
		slices.Sort(members)
	}
	return groups
}

func (d *DAG) normalizeRecordExclusiveFields(
	listPath, recordID string,
	fields map[string]any,
) (map[string]any, error) {
	var normalized map[string]any
	for _, group := range slices.Sorted(maps.Keys(d.listExclusiveGroups[listPath])) {
		members := d.listExclusiveGroups[listPath][group]
		var on []string
		for _, member := range members {
			value, present := fields[member]
			if !present {
				continue
			}
			active, valid := exclusiveValueOn(value)
			if !valid {
				return nil, ErrExclusiveValueType(group, listPath+"."+recordID+"."+member)
			}
			if active {
				on = append(on, listPath+"."+recordID+"."+member)
			}
		}

		switch len(on) {
		case 0:
			continue
		case 1:
			if normalized == nil {
				normalized = copyFlatUpdates(fields)
			}
			for _, member := range members {
				if listPath+"."+recordID+"."+member == on[0] {
					continue
				}
				if _, present := fields[member]; present {
					continue
				}
				normalized[member] = 0.0
			}
		default:
			return nil, ErrExclusiveGroupConflict(group, on)
		}
	}
	if normalized == nil {
		return fields, nil
	}
	return normalized, nil
}
