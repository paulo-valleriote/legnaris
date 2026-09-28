package template

import "slices"

// cloneTemplate returns a deep copy of a template: the FieldGroup tree, every
// FieldSchema pointer and every DependsOn path are new allocations, so nothing
// in the result aliases the original.
//
// The registry clones on ingest (BuildRegistry, from the DefaultTemplates()
// catalog) and on egress (List, FetchByID), which keeps the registered
// templates frozen after boot no matter who mutates what (LGN-54).
func cloneTemplate(tmpl Template) Template {
	tmpl.Fields = cloneFieldGroup(tmpl.Fields)
	return tmpl
}

// cloneFieldGroup deep-copies a field group, preserving a nil group as nil.
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

// cloneFieldNode deep-copies a single node: leaf schemas get a fresh
// FieldSchema, groups get a fresh children map.
func cloneFieldNode(node FieldNode) FieldNode {
	var cloned FieldNode
	if node.Schema != nil {
		schema := *node.Schema
		schema.DependsOn = cloneDependsOn(node.Schema.DependsOn)
		cloned.Schema = &schema
	}
	cloned.Children = cloneFieldGroup(node.Children)
	return cloned
}

// cloneDependsOn copies the dependency paths, including their segments.
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
