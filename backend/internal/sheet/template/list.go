package template

import (
	"maps"
	"slices"
	"strings"
)

const (
	ListPositionField = "position"

	MinListCapacity = 1

	MaxListCapacity = 200

	MaxListPathSegments = 3
)

type ListSchema struct {
	Label    string     `bson:"label"`
	Columns  []string   `bson:"columns,omitempty"`
	Capacity int        `bson:"capacity"`
	Shape    FieldGroup `bson:"shape"`
}

func (l *ListSchema) SeedRecord(position float64) map[string]any {
	record := make(map[string]any, len(l.Shape)+1)
	for name, node := range l.Shape {
		if node.IsLeaf() {
			record[name] = node.Schema.SeedValue()
		}
	}
	record[ListPositionField] = position
	return record
}

func (l *ListSchema) Clone() *ListSchema {
	if l == nil {
		return nil
	}
	return &ListSchema{
		Label:    l.Label,
		Columns:  slices.Clone(l.Columns),
		Capacity: l.Capacity,
		Shape:    cloneFieldGroup(l.Shape),
	}
}

func collectLists(group FieldGroup, prefix Path) (map[string]ListSchema, error) {
	lists := make(map[string]ListSchema)
	if err := collectListsRecursive(group, prefix, lists); err != nil {
		return nil, err
	}
	return lists, nil
}

func collectListsRecursive(group FieldGroup, prefix Path, lists map[string]ListSchema) error {
	for _, name := range slices.Sorted(maps.Keys(group)) {
		node := group[name]
		current := append(append(Path{}, prefix...), name)
		if node.IsList() {
			if err := validateListDeclaration(current, node.List); err != nil {
				return err
			}
			lists[current.String()] = *node.List.Clone()
			continue
		}
		if node.IsGroup() {
			if err := collectListsRecursive(node.Children, current, lists); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateListDeclaration(path Path, list *ListSchema) error {
	if len(path) > MaxListPathSegments {
		return ErrListPathTooDeep(path.String(), MaxListPathSegments)
	}
	if list == nil {
		return ErrInvalidListShape(path.String(), "list carries no shape")
	}
	if list.Capacity < MinListCapacity || list.Capacity > MaxListCapacity {
		return ErrListCapacity(path.String(), list.Capacity, MinListCapacity, MaxListCapacity)
	}

	for _, name := range slices.Sorted(maps.Keys(list.Shape)) {
		if name == ListPositionField {
			return ErrReservedListField(path.String(), name)
		}
		fieldPath := append(append(Path{}, path...), name)
		if !fieldNameRe.MatchString(name) {
			return ErrInvalidFieldName(fieldPath.String(), fieldNameRule)
		}
		node := list.Shape[name]
		if node.IsList() {
			return ErrNestedList(fieldPath.String())
		}
		if !node.IsLeaf() || node.Schema == nil {
			return ErrInvalidListShape(fieldPath.String(), "shape fields must be leaves")
		}
	}
	return nil
}

func (d *DAG) ListPaths() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return slices.Sorted(maps.Keys(d.lists))
}

func listByName(lists map[string]ListSchema, key string) (string, ListSchema, bool) {
	best := ""
	var bestSchema ListSchema
	for path, schema := range lists {
		if key != path && !strings.HasPrefix(key, path+".") {
			continue
		}
		if len(path) > len(best) {
			best = path
			bestSchema = schema
		}
	}
	if best == "" {
		return "", ListSchema{}, false
	}
	return best, bestSchema, true
}
