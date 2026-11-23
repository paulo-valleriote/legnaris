package template

import (
	"maps"
	"slices"
	"strings"

	"legnaris/backend/internal/dice"
)

const syntheticRecordID = "r00000000000000000000000000000000"

type RecordRow struct {
	ListPath string
	RecordID string
	Section  string
}

func RecordRowKey(listPath, recordID string) string {
	return listPath + pathSeparator + recordID
}

func RecordFieldPath(listPath, recordID, field string) string {
	return RecordRowKey(listPath, recordID) + pathSeparator + field
}

func ParseRecordRowKey(tmpl Template, key string) (RecordRow, bool) {
	lists := templateLists(tmpl)
	listPath, _, ok := listByName(lists, key)
	if !ok || key == listPath {
		return RecordRow{}, false
	}
	remainder := strings.TrimPrefix(key, listPath+pathSeparator)
	if strings.Contains(remainder, pathSeparator) || ValidateRecordID(remainder) != nil {
		return RecordRow{}, false
	}
	return RecordRow{ListPath: listPath, RecordID: remainder, Section: sectionOfListPath(listPath)}, true
}

func IsRowKey(tmpl Template, key string) bool {
	if staticRowKeys(tmpl)[key] {
		return true
	}
	_, ok := ParseRecordRowKey(tmpl, key)
	return ok
}

func RowSectionKey(tmpl Template, key string) (string, bool) {
	for _, section := range SectionRowKeys(tmpl) {
		if slices.Contains(section.Rows, key) {
			return section.Section, true
		}
	}
	row, ok := ParseRecordRowKey(tmpl, key)
	if !ok {
		return "", false
	}
	return row.Section, true
}

type templateRowIndex struct {
	static      map[string]bool
	lists       map[string]ListSchema
	sectionRows map[string][]string
}

func buildTemplateRowIndex(tmpl Template) (*templateRowIndex, error) {
	lists, err := collectLists(tmpl.Fields, nil)
	if err != nil {
		return nil, err
	}
	static := make(map[string]bool)
	sectionRows := make(map[string][]string)
	for _, section := range SectionRowKeys(tmpl) {
		sectionRows[section.Section] = slices.Clone(section.Rows)
		for _, row := range section.Rows {
			static[row] = true
		}
	}
	for _, listPath := range slices.Sorted(maps.Keys(lists)) {
		section := sectionOfListPath(listPath)
		sectionRows[section] = append(
			sectionRows[section],
			RecordRowKey(listPath, syntheticRecordID),
		)
	}
	return &templateRowIndex{static: static, lists: lists, sectionRows: sectionRows}, nil
}

func (t *Template) prepareRowIndex() error {
	index, err := buildTemplateRowIndex(*t)
	if err != nil {
		return err
	}
	t.rowIndex = index
	return nil
}

func (t Template) rows() *templateRowIndex {
	if t.rowIndex != nil {
		return t.rowIndex
	}
	index, err := buildTemplateRowIndex(t)
	if err != nil {
		return &templateRowIndex{
			static:      map[string]bool{},
			lists:       map[string]ListSchema{},
			sectionRows: map[string][]string{},
		}
	}
	return index
}

func staticRowKeys(tmpl Template) map[string]bool {
	return tmpl.rows().static
}

func templateLists(tmpl Template) map[string]ListSchema {
	return tmpl.rows().lists
}

func sectionOfListPath(listPath string) string {
	section, _, _ := strings.Cut(listPath, pathSeparator)
	return section
}

func sectionRowsForFit(tmpl Template) map[string][]string {
	return tmpl.rows().sectionRows
}

type templateRowSchema struct {
	kinds dice.MapSchema
	lists map[string]ListSchema
}

func rowDiceSchema(tmpl Template) templateRowSchema {
	return templateRowSchema{kinds: fieldKinds(tmpl.Fields), lists: templateLists(tmpl)}
}

func (s templateRowSchema) FieldKind(path string) (dice.FieldKind, bool) {
	if kind, ok := s.kinds[path]; ok {
		return kind, true
	}
	listPath, list, ok := listByName(s.lists, path)
	if !ok {
		return 0, false
	}
	remainder := strings.TrimPrefix(path, listPath+pathSeparator)
	recordID, field, ok := strings.Cut(remainder, pathSeparator)
	if !ok || ValidateRecordID(recordID) != nil {
		return 0, false
	}
	node, ok := list.Shape[field]
	if !ok || !node.IsLeaf() || node.Schema == nil {
		return 0, false
	}
	return diceKind(node.Schema.Type), true
}
