package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func layoutLeaf(exclusive string) FieldNode {
	return FieldNode{Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, ExclusiveGroup: exclusive}}
}

func layoutFixture() Template {
	return Template{
		ID: "layout_fixture",
		Fields: FieldGroup{
			"stats": {Children: FieldGroup{
				"strength":  layoutLeaf(""),
				"dexterity": layoutLeaf(""),
			}},
			"baseArmorClass": layoutLeaf(""),
			"skills": {Children: FieldGroup{
				"acrobatics": {
					Children: FieldGroup{"bonus": layoutLeaf(""), "total": layoutLeaf("")},
					Columns:  []string{"bonus", "total"},
				},
				"arcana": {
					Children: FieldGroup{"bonus": layoutLeaf(""), "total": layoutLeaf("")},
					Columns:  []string{"bonus", "total"},
				},
			}},
			"config": {Children: FieldGroup{
				"acUsesCon": layoutLeaf("armorClassAbility"),
				"acUsesDex": layoutLeaf("armorClassAbility"),
				"flag":      layoutLeaf(""),
				"lonely":    layoutLeaf("solo"),
			}},
			"empty": {Children: FieldGroup{}},
		},
		SectionOrder: []string{"stats", "baseArmorClass", "skills", "config"},
	}
}

func TestSectionRowKeys(t *testing.T) {
	tests := []struct {
		name string
		tmpl Template
		want []SectionRows
	}{
		{
			name: "Edge: template with no fields has no sections",
			tmpl: Template{ID: "empty"},
			want: []SectionRows{},
		},
		{
			name: "Edge: a group without any leaf is omitted",
			tmpl: Template{Fields: FieldGroup{"empty": {Children: FieldGroup{}}}},
			want: []SectionRows{},
		},
		{
			name: "Edge: a top-level leaf is its own section and row",
			tmpl: Template{Fields: FieldGroup{"baseArmorClass": layoutLeaf("")}},
			want: []SectionRows{{Section: "baseArmorClass", Rows: []string{"baseArmorClass"}}},
		},
		{
			name: "Edge: sections without a declared order are alphabetical",
			tmpl: Template{Fields: FieldGroup{
				"zeta":  {Children: FieldGroup{"a": layoutLeaf("")}},
				"alpha": {Children: FieldGroup{"a": layoutLeaf("")}},
			}},
			want: []SectionRows{
				{Section: "alpha", Rows: []string{"alpha.a"}},
				{Section: "zeta", Rows: []string{"zeta.a"}},
			},
		},
		{
			name: "Edge: ghost and duplicate order entries are skipped, unlisted keys follow",
			tmpl: Template{
				Fields: FieldGroup{
					"b": {Children: FieldGroup{"x": layoutLeaf("")}},
					"a": {Children: FieldGroup{"x": layoutLeaf("")}},
					"c": {Children: FieldGroup{"x": layoutLeaf("")}},
				},
				SectionOrder: []string{"ghost", "c", "c"},
			},
			want: []SectionRows{
				{Section: "c", Rows: []string{"c.x"}},
				{Section: "a", Rows: []string{"a.x"}},
				{Section: "b", Rows: []string{"b.x"}},
			},
		},
		{
			name: "Edge: a table group whose columns resolve to nothing falls back to stacked leaves",
			tmpl: Template{Fields: FieldGroup{"s": {Children: FieldGroup{
				"row": {
					Children: FieldGroup{"a": layoutLeaf(""), "b": layoutLeaf("")},
					Columns:  []string{"ghost", "nested"},
				},
			}}}},
			want: []SectionRows{{Section: "s", Rows: []string{"s.row.a", "s.row.b"}}},
		},
		{
			name: "Edge: a table with a partly resolvable column list is still one row",
			tmpl: Template{Fields: FieldGroup{"s": {Children: FieldGroup{
				"row": {
					Children: FieldGroup{"a": layoutLeaf(""), "b": layoutLeaf("")},
					Columns:  []string{"ghost", "a"},
				},
			}}}},
			want: []SectionRows{{Section: "s", Rows: []string{"s.row"}}},
		},
		{
			name: "Edge: exclusive members under a table are not separate rows",
			tmpl: Template{Fields: FieldGroup{"s": {Children: FieldGroup{
				"row": {
					Children: FieldGroup{"a": layoutLeaf("g"), "b": layoutLeaf("g")},
					Columns:  []string{"a", "b"},
				},
			}}}},
			want: []SectionRows{{Section: "s", Rows: []string{"s.row"}}},
		},
		{
			name: "Edge: a single-member exclusive group stays a lone leaf row",
			tmpl: Template{Fields: FieldGroup{"s": {Children: FieldGroup{
				"only": layoutLeaf("solo"),
			}}}},
			want: []SectionRows{{Section: "s", Rows: []string{"s.only"}}},
		},
		{
			name: "Edge: non-adjacent exclusive members collapse at the first member position",
			tmpl: Template{Fields: FieldGroup{"s": {Children: FieldGroup{
				"a": layoutLeaf("g"),
				"b": layoutLeaf(""),
				"c": layoutLeaf("g"),
			}}}},
			want: []SectionRows{{Section: "s", Rows: []string{"s#g", "s.b"}}},
		},
		{
			name: "Edge: exclusive key uses the parent path of the first member",
			tmpl: Template{Fields: FieldGroup{"s": {Children: FieldGroup{
				"inner": {Children: FieldGroup{"a": layoutLeaf("g"), "b": layoutLeaf("g")}},
			}}}},
			want: []SectionRows{{Section: "s", Rows: []string{"s.inner#g"}}},
		},
		{
			name: "Edge: the same exclusive id in two sections never merges across them",
			tmpl: Template{Fields: FieldGroup{
				"p": {Children: FieldGroup{"a": layoutLeaf("g"), "b": layoutLeaf("g")}},
				"q": {Children: FieldGroup{"a": layoutLeaf("g")}},
			}},
			want: []SectionRows{
				{Section: "p", Rows: []string{"p#g"}},
				{Section: "q", Rows: []string{"q.a"}},
			},
		},
		{
			name: "Happy path: mixed fixture derives lone, table and exclusive rows in order",
			tmpl: layoutFixture(),
			want: []SectionRows{
				{Section: "stats", Rows: []string{"stats.dexterity", "stats.strength"}},
				{Section: "baseArmorClass", Rows: []string{"baseArmorClass"}},
				{Section: "skills", Rows: []string{"skills.acrobatics", "skills.arcana"}},
				{Section: "config", Rows: []string{"config#armorClassAbility", "config.flag", "config.lonely"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SectionRowKeys(tc.tmpl))
		})
	}
}

func TestSectionRowKeys_DefaultTemplatesAreUniqueAndNonEmpty(t *testing.T) {
	for _, tmpl := range DefaultTemplates() {
		t.Run(tmpl.ID, func(t *testing.T) {
			sections := SectionRowKeys(tmpl)
			require.NotEmpty(t, sections)

			seen := map[string]bool{}
			for _, section := range sections {
				require.NotEmpty(t, section.Rows, section.Section)
				for _, row := range section.Rows {
					assert.False(t, seen[row], "row key %q repeated", row)
					seen[row] = true
				}
			}
		})
	}
}

func TestSectionRowKeys_FantasyV1KnownRows(t *testing.T) {
	var fantasy Template
	for _, tmpl := range DefaultTemplates() {
		if tmpl.ID == "fantasy_v1" {
			fantasy = tmpl
		}
	}
	require.NotEmpty(t, fantasy.ID)

	byKey := map[string][]string{}
	for _, section := range SectionRowKeys(fantasy) {
		byKey[section.Section] = section.Rows
	}

	assert.Contains(t, byKey["stats"], "stats.dexterity")
	assert.Contains(t, byKey["skills"], "skills.perception")
	assert.Contains(t, byKey["attacks"], "attacks.attack1")
	assert.Contains(t, byKey["spells"], "spells.slot1")
	assert.Contains(t, byKey["config"], "config#armorClassAbility")
	assert.Contains(t, byKey["config"], "config#spellcastingAbility")
	assert.NotContains(t, byKey["config"], "config.acUsesDex")
	assert.NotContains(t, byKey["skills"], "skills.perception.total")
}

func TestValidateLayout(t *testing.T) {
	tmpl := layoutFixture()

	tests := []struct {
		name         string
		sectionOrder []string
		pinned       []string
		wantErr      error
	}{
		{name: "Edge: nil everything is valid"},
		{
			name:         "Edge: empty non-nil lists are valid",
			sectionOrder: []string{},
			pinned:       []string{},
		},
		{
			name:         "Edge: a partial section_order is valid",
			sectionOrder: []string{"config"},
		},
		{
			name:         "Edge: the exact template order is accepted",
			sectionOrder: []string{"stats", "baseArmorClass", "skills", "config"},
		},
		{
			name:         "Error: unknown section in section_order",
			sectionOrder: []string{"stats", "nope"},
			wantErr:      &UnknownLayoutSectionError{Key: "nope"},
		},
		{
			name:         "Error: a group without leaves is not a section",
			sectionOrder: []string{"empty"},
			wantErr:      &UnknownLayoutSectionError{Key: "empty"},
		},
		{
			name:         "Error: empty string section key",
			sectionOrder: []string{""},
			wantErr:      &UnknownLayoutSectionError{Key: ""},
		},
		{
			name:         "Error: duplicate section in section_order",
			sectionOrder: []string{"stats", "config", "stats"},
			wantErr:      &DuplicateLayoutKeyError{List: "section_order", Key: "stats"},
		},
		{
			name:    "Error: pinned uses a column path instead of the table row key",
			pinned:  []string{"skills.acrobatics.bonus"},
			wantErr: &UnknownLayoutRowError{Key: "skills.acrobatics.bonus"},
		},
		{
			name:    "Error: pinned uses a collapsed exclusive member path",
			pinned:  []string{"config.acUsesDex"},
			wantErr: &UnknownLayoutRowError{Key: "config.acUsesDex"},
		},
		{
			name:    "Error: empty string pinned key",
			pinned:  []string{""},
			wantErr: &UnknownLayoutRowError{Key: ""},
		},
		{
			name:    "Error: pinned key that is no row of any section",
			pinned:  []string{"stats.strength", "stats.charisma"},
			wantErr: &UnknownLayoutRowError{Key: "stats.charisma"},
		},
		{
			name:    "Error: pinned section key is not a row",
			pinned:  []string{"stats"},
			wantErr: &UnknownLayoutRowError{Key: "stats"},
		},
		{
			name:    "Error: duplicate pinned key",
			pinned:  []string{"skills.arcana", "skills.arcana"},
			wantErr: &DuplicateLayoutKeyError{List: "pinned", Key: "skills.arcana"},
		},
		{
			name:         "Happy path: full layout with pins across sections kept in given order",
			sectionOrder: []string{"config", "stats"},
			pinned:       []string{"skills.arcana", "stats.strength", "config#armorClassAbility"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLayout(tmpl, tc.sectionOrder, tc.pinned)
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestLayoutErrors_Messages(t *testing.T) {
	assert.Equal(t, "unknown layout section 'x'", (&UnknownLayoutSectionError{Key: "x"}).Error())
	assert.Equal(t, "unknown pinned row 'x'", (&UnknownLayoutRowError{Key: "x"}).Error())
	assert.Equal(t, "duplicate key 'x' in pinned", (&DuplicateLayoutKeyError{List: "pinned", Key: "x"}).Error())
	assert.Equal(t, "duplicate key 'x' in section_order", (&DuplicateLayoutKeyError{List: "section_order", Key: "x"}).Error())
}

func TestValidateLayout_FantasyV1_StoredLayoutSurvivesSectionReorder(t *testing.T) {
	legacyOrder := []string{
		"identity", "stats", "modifiers", "saves", "skills", "combat",
		"attacks", "spells", "derived", "rules", "config",
	}
	legacyPins := []string{
		"stats.strength", "skills.perception", "combat.hitPoints",
		"derived.armorClass", "config#armorClassAbility",
	}

	tests := []struct {
		name         string
		sectionOrder []string
		pinned       []string
	}{
		{name: "Happy path: the full order stored before the reorder still validates", sectionOrder: legacyOrder, pinned: legacyPins},
		{name: "Edge: a partial stored order still validates", sectionOrder: []string{"skills", "stats"}},
		{name: "Edge: stored pins alone still validate", pinned: legacyPins},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, ValidateLayout(fantasyV1Template, tc.sectionOrder, tc.pinned))
		})
	}

	t.Run("Happy path: the reorder changes presentation only, every stored key still resolves", func(t *testing.T) {
		sections := map[string]bool{}
		rows := map[string]bool{}
		for _, section := range SectionRowKeys(fantasyV1Template) {
			sections[section.Section] = true
			for _, row := range section.Rows {
				rows[row] = true
			}
		}
		for _, key := range legacyOrder {
			assert.True(t, sections[key], "section %q no longer resolves", key)
		}
		assert.Len(t, sections, len(legacyOrder))
		for _, key := range legacyPins {
			assert.True(t, rows[key], "row %q no longer resolves", key)
		}
	})
}
