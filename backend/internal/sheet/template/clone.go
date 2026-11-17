package template

import (
	"maps"
	"slices"
)

func cloneTemplate(tmpl Template) Template {
	tmpl.Fields = cloneFieldGroup(tmpl.Fields)
	tmpl.SectionOrder = slices.Clone(tmpl.SectionOrder)
	tmpl.SectionDice = maps.Clone(tmpl.SectionDice)
	tmpl.RowDice = maps.Clone(tmpl.RowDice)
	return tmpl
}

func cloneFieldGroup(group FieldGroup) FieldGroup {
	if group == nil {
		return nil
	}
	cloned := make(FieldGroup, len(group))
	for name, node := range group {
		cloned[name] = cloneFieldNode(node)
	}
	return cloned
}

func cloneFieldNode(node FieldNode) FieldNode {
	var cloned FieldNode
	if node.Schema != nil {
		schema := *node.Schema
		schema.DependsOn = cloneDependsOn(node.Schema.DependsOn)
		schema.GeneratableKinds = slices.Clone(node.Schema.GeneratableKinds)
		schema.Min = clonePtr(node.Schema.Min)
		schema.Max = clonePtr(node.Schema.Max)
		schema.MaxLength = clonePtr(node.Schema.MaxLength)
		cloned.Schema = &schema
	}
	cloned.Children = cloneFieldGroup(node.Children)
	cloned.Label = node.Label
	cloned.Columns = slices.Clone(node.Columns)
	cloned.Order = slices.Clone(node.Order)
	return cloned
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneDependsOn(dependsOn [][]string) [][]string {
	if dependsOn == nil {
		return nil
	}
	cloned := make([][]string, len(dependsOn))
	for i, path := range dependsOn {
		cloned[i] = slices.Clone(path)
	}
	return cloned
}
