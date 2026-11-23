package sheettest

import "legnaris/backend/internal/sheet/template"

func SeedTemplateData(fields template.FieldGroup) map[string]any {
	result := make(map[string]any)
	for name, node := range fields {
		switch {
		case node.IsLeaf():
			result[name] = node.Schema.SeedValue()
		case node.IsGroup():
			result[name] = SeedTemplateData(node.Children)
		case node.IsList():
			result[name] = map[string]any{}
		}
	}
	return result
}
