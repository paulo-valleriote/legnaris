package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rowIdentityRecordID  = "r0123456789abcdef0123456789abcdef"
	rowIdentityRecordID2 = "rffffffffffffffffffffffffffffffff"
)

func rowNumberLeaf() FieldNode {
	return FieldNode{Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true}}
}

func rowTextLeaf() FieldNode {
	return FieldNode{Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true}}
}

func rowBoolLeaf() FieldNode {
	return FieldNode{Schema: &FieldSchema{Type: fieldTypeBool, IsEditable: true}}
}

func rowIdentityTemplate() Template {
	return Template{
		ID: "row_identity",
		Fields: FieldGroup{
			"stats": {Children: FieldGroup{
				"strength": rowNumberLeaf(),
			}},
			"inventory": {Children: FieldGroup{
				"items": {List: &ListSchema{
					Label:    "Items",
					Columns:  []string{"name", "quantity"},
					Capacity: 2,
					Shape: FieldGroup{
						"name":     rowTextLeaf(),
						"quantity": rowNumberLeaf(),
						"active":   rowBoolLeaf(),
					},
				}},
			}},
		},
		SectionOrder: []string{"stats", "inventory"},
	}
}

func TestRecordRowKey(t *testing.T) {
	tests := []struct {
		name     string
		listPath string
		recordID string
		want     string
	}{
		{
			name:     "Happy path: joins the list path and the id",
			listPath: "inventory.items",
			recordID: rowIdentityRecordID,
			want:     "inventory.items." + rowIdentityRecordID,
		},
		{
			name:     "Edge: empty parts still compose the separator",
			listPath: "",
			recordID: "",
			want:     ".",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RecordRowKey(tc.listPath, tc.recordID))
		})
	}
}

func TestParseRecordRowKey(t *testing.T) {
	tmpl := rowIdentityTemplate()

	tests := []struct {
		name string
		key  string
		want RecordRow
		ok   bool
	}{
		{
			name: "Happy path: a well-formed record row",
			key:  "inventory.items." + rowIdentityRecordID,
			want: RecordRow{ListPath: "inventory.items", RecordID: rowIdentityRecordID, Section: "inventory"},
			ok:   true,
		},
		{
			name: "Happy path: an all-zero id",
			key:  "inventory.items.r00000000000000000000000000000000",
			want: RecordRow{
				ListPath: "inventory.items",
				RecordID: "r00000000000000000000000000000000",
				Section:  "inventory",
			},
			ok: true,
		},
		{
			name: "Error: an id one character short",
			key:  "inventory.items.r0123456789abcdef0123456789abcde",
		},
		{
			name: "Error: an uppercase id",
			key:  "inventory.items.r0123456789ABCDEF0123456789ABCDEF",
		},
		{
			name: "Error: a non-hex id",
			key:  "inventory.items.r0123456789abcdef0123456789abcdeg",
		},
		{
			name: "Error: an id without the r prefix",
			key:  "inventory.items.0123456789abcdef0123456789abcdef",
		},
		{
			name: "Error: an empty id",
			key:  "inventory.items.",
		},
		{
			name: "Error: an unknown list",
			key:  "unknown.items." + rowIdentityRecordID,
		},
		{
			name: "Error: the list path alone",
			key:  "inventory.items",
		},
		{
			name: "Error: a field below the record",
			key:  "inventory.items." + rowIdentityRecordID + ".quantity",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRecordRowKey(tmpl, tc.key)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsRowKey(t *testing.T) {
	tmpl := rowIdentityTemplate()

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "Happy path: a static leaf", key: "stats.strength", want: true},
		{name: "Happy path: a record row", key: "inventory.items." + rowIdentityRecordID, want: true},
		{name: "Edge: an all-zero record id", key: "inventory.items.r00000000000000000000000000000000", want: true},
		{name: "Error: an unknown static leaf", key: "stats.dexterity"},
		{name: "Error: a record id one character short", key: "inventory.items.r0123456789abcdef0123456789abcde"},
		{name: "Error: an unknown list", key: "unknown.items." + rowIdentityRecordID},
		{name: "Error: the list path alone", key: "inventory.items"},
		{name: "Error: a record field path", key: "inventory.items." + rowIdentityRecordID + ".quantity"},
		{name: "Error: the empty key", key: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsRowKey(tmpl, tc.key))
		})
	}
}

func TestRowSectionKey(t *testing.T) {
	tmpl := rowIdentityTemplate()

	tests := []struct {
		name    string
		key     string
		section string
		ok      bool
	}{
		{
			name:    "Happy path: a static row answers its section",
			key:     "stats.strength",
			section: "stats",
			ok:      true,
		},
		{
			name:    "Happy path: a record row answers its list's section",
			key:     "inventory.items." + rowIdentityRecordID,
			section: "inventory",
			ok:      true,
		},
		{name: "Error: an unknown key", key: "inventory.items.rbad"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			section, ok := RowSectionKey(tmpl, tc.key)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.section, section)
		})
	}
}

func TestSectionRowsForFit(t *testing.T) {
	tmpl := rowIdentityTemplate()
	tmpl.Fields["inventory"].Children["gems"] = FieldNode{List: &ListSchema{
		Label:    "Gems",
		Capacity: 1,
		Shape: FieldGroup{
			"name":     rowTextLeaf(),
			"quantity": rowNumberLeaf(),
		},
	}}
	tmpl.Fields["spells"] = FieldNode{Children: FieldGroup{
		"known": {List: &ListSchema{
			Label:    "Known",
			Capacity: 1,
			Shape: FieldGroup{
				"level": rowNumberLeaf(),
			},
		}},
	}}

	rows := sectionRowsForFit(tmpl)

	assert.Equal(t, []string{"stats.strength"}, rows["stats"])
	assert.ElementsMatch(t, []string{
		"inventory.items." + syntheticRecordID,
		"inventory.gems." + syntheticRecordID,
	}, rows["inventory"])
	assert.Equal(t, []string{"spells.known." + syntheticRecordID}, rows["spells"])
}

func TestSectionRowsForFit_ListOnlySectionIsASection(t *testing.T) {
	tmpl := Template{Fields: FieldGroup{
		"items": {List: &ListSchema{
			Label:    "Items",
			Capacity: 1,
			Shape:    FieldGroup{"quantity": rowNumberLeaf()},
		}},
	}}

	assert.Equal(t, []string{"items." + syntheticRecordID}, sectionRowsForFit(tmpl)["items"])
}

func TestRecordRowSchema(t *testing.T) {
	tmpl := rowIdentityTemplate()
	schema := rowDiceSchema(tmpl)

	tests := []struct {
		name string
		path string
		kind string
		ok   bool
	}{
		{
			name: "Happy path: a numeric shape column",
			path: "inventory.items." + rowIdentityRecordID + ".quantity",
			kind: "number",
			ok:   true,
		},
		{
			name: "Happy path: a text shape column",
			path: "inventory.items." + rowIdentityRecordID + ".name",
			kind: "text",
			ok:   true,
		},
		{
			name: "Happy path: a static leaf",
			path: "stats.strength",
			kind: "number",
			ok:   true,
		},
		{name: "Error: the reserved position", path: "inventory.items." + rowIdentityRecordID + ".position"},
		{name: "Error: an unknown shape column", path: "inventory.items." + rowIdentityRecordID + ".missing"},
		{name: "Error: a malformed id", path: "inventory.items.rbad.quantity"},
		{name: "Error: an unknown list", path: "unknown.items." + rowIdentityRecordID + ".quantity"},
		{name: "Error: the record itself", path: "inventory.items." + rowIdentityRecordID},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind, ok := schema.FieldKind(tc.path)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, diceKind(tc.kind), kind)
			}
		})
	}
}
