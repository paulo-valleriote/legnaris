package template

import (
	"slices"
	"strings"
)

const (
	pathSeparator          = "."
	exclusiveRowSeparator  = "#"
	layoutListSectionOrder = "section_order"
	layoutListPinned       = "pinned"
)

// SectionRows is one Section of a Template with the keys of its Rows in the
// Template's default order.
type SectionRows struct {
	Section string
	Rows    []string
}

// SectionRowKeys lists the Template's Sections in default display order, each
// with the keys of its Rows.
//
// It mirrors the web client's buildRows, because a Row key is only meaningful
// if both sides derive the same set from the same Template: a lone leaf is
// keyed by its dotted path, a group declaring Columns that resolve to at least
// one leaf is one table Row keyed by the group path, and the leaves sharing a
// non-empty exclusive group collapse into one Row keyed
// "<parentPath>#<exclusive_group>" at the first member's position — unless the
// group has a single member, which stays a lone leaf. Children are walked
// alphabetically, as the API serializes them. A Section with no leaf is
// omitted.
func SectionRowKeys(tmpl Template) []SectionRows {
	sections := make([]SectionRows, 0, len(tmpl.Fields))
	for _, key := range orderedKeys(tmpl.Fields, tmpl.SectionOrder) {
		rows := buildRowKeys(key, tmpl.Fields[key])
		if len(rows) == 0 {
			continue
		}
		sections = append(sections, SectionRows{Section: key, Rows: rows})
	}
	return sections
}

type exclusiveRow struct {
	index   int
	parent  string
	members int
}

func buildRowKeys(prefix string, root FieldNode) []string {
	var rows []string
	exclusive := map[string]*exclusiveRow{}
	var exclusiveOrder []string

	addLeaf := func(path string, schema *FieldSchema) {
		id := schema.ExclusiveGroup
		if id == "" {
			rows = append(rows, path)
			return
		}
		if row, seen := exclusive[id]; seen {
			row.members++
			return
		}
		exclusive[id] = &exclusiveRow{index: len(rows), parent: parentPath(path), members: 1}
		exclusiveOrder = append(exclusiveOrder, id)
		rows = append(rows, path)
	}

	var walk func(path string, node FieldNode)
	walk = func(path string, node FieldNode) {
		if node.IsLeaf() {
			addLeaf(path, node.Schema)
			return
		}
		if hasResolvableColumn(node) {
			rows = append(rows, path)
			return
		}
		for _, key := range orderedKeys(node.Children, node.Order) {
			walk(path+pathSeparator+key, node.Children[key])
		}
	}
	walk(prefix, root)

	for _, id := range exclusiveOrder {
		row := exclusive[id]
		if row.members < 2 {
			continue
		}
		rows[row.index] = row.parent + exclusiveRowSeparator + id
	}
	return rows
}

func hasResolvableColumn(node FieldNode) bool {
	for _, column := range node.Columns {
		child, ok := node.Children[column]
		if ok && child.IsLeaf() {
			return true
		}
	}
	return false
}

func parentPath(path string) string {
	index := strings.LastIndex(path, pathSeparator)
	if index < 0 {
		return path
	}
	return path[:index]
}

func orderedKeys(group FieldGroup, order []string) []string {
	keys := make([]string, 0, len(group))
	emitted := make(map[string]bool, len(group))
	for _, key := range order {
		if _, ok := group[key]; ok && !emitted[key] {
			keys = append(keys, key)
			emitted[key] = true
		}
	}
	rest := make([]string, 0, len(group)-len(keys))
	for key := range group {
		if !emitted[key] {
			rest = append(rest, key)
		}
	}
	slices.Sort(rest)
	return append(keys, rest...)
}

// ValidateLayout checks a player's Layout, given as its two ordered lists,
// against the Rows the Template actually has. Lists may be partial (missing
// keys fall back to Template order on read), but every key present must be
// known and appear once per list: an unknown Section, a Pin that is no Row of
// any Section, or a duplicate returns a typed error.
func ValidateLayout(tmpl Template, sectionOrder []string, pinned []string) error {
	sections := make(map[string]bool)
	rows := make(map[string]bool)
	for _, section := range SectionRowKeys(tmpl) {
		sections[section.Section] = true
		for _, row := range section.Rows {
			rows[row] = true
		}
	}

	if err := validateSectionOrder(sections, sectionOrder); err != nil {
		return err
	}
	return validatePinned(rows, pinned)
}

func validateSectionOrder(sections map[string]bool, sectionOrder []string) error {
	seen := make(map[string]bool, len(sectionOrder))
	for _, key := range sectionOrder {
		if !sections[key] {
			return &UnknownLayoutSectionError{Key: key}
		}
		if seen[key] {
			return &DuplicateLayoutKeyError{List: layoutListSectionOrder, Key: key}
		}
		seen[key] = true
	}
	return nil
}

func validatePinned(rows map[string]bool, pinned []string) error {
	seen := make(map[string]bool, len(pinned))
	for _, key := range pinned {
		if !rows[key] {
			return &UnknownLayoutRowError{Key: key}
		}
		if seen[key] {
			return &DuplicateLayoutKeyError{List: layoutListPinned, Key: key}
		}
		seen[key] = true
	}
	return nil
}
