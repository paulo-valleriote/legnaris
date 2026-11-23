package template

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func notD20Fixture(sections, rows []string) Template {
	tmpl := layoutFixture()
	tmpl.NotD20Sections = sections
	tmpl.NotD20Rows = rows
	return tmpl
}

func TestValidateNotD20Tests(t *testing.T) {
	tests := []struct {
		name     string
		sections []string
		rows     []string
		wantErr  error
	}{
		{name: "Edge: nil declarations are valid"},
		{name: "Edge: empty declarations are valid", sections: []string{}, rows: []string{}},
		{
			name:     "Happy path: a Section and a Row are valid",
			sections: []string{"skills"},
			rows:     []string{"baseArmorClass"},
		},
		{
			name: "Edge: a table Row and an exclusive Row are Rows",
			rows: []string{"skills.acrobatics", "config#armorClassAbility"},
		},
		{
			name:     "Edge: a marking repeated in the same list is tolerated",
			sections: []string{"skills", "skills"},
			rows:     []string{"baseArmorClass", "baseArmorClass"},
		},
		{
			name:     "Error: an unknown Section",
			sections: []string{"nope"},
			wantErr:  &UnknownNotD20SectionError{Key: "nope"},
		},
		{
			name:     "Error: a Section without Rows is not a Section",
			sections: []string{"empty"},
			wantErr:  &UnknownNotD20SectionError{Key: "empty"},
		},
		{
			name:     "Error: an empty Section key",
			sections: []string{""},
			wantErr:  &UnknownNotD20SectionError{Key: ""},
		},
		{
			name:    "Error: an unknown Row",
			rows:    []string{"stats.nope"},
			wantErr: &UnknownNotD20RowError{Key: "stats.nope"},
		},
		{
			name:    "Error: a bare Section key is not a Row",
			rows:    []string{"stats"},
			wantErr: &UnknownNotD20RowError{Key: "stats"},
		},
		{
			name:    "Error: a table column is not a Row",
			rows:    []string{"skills.acrobatics.total"},
			wantErr: &UnknownNotD20RowError{Key: "skills.acrobatics.total"},
		},
		{
			name:     "Error: Sections are checked before Rows",
			sections: []string{"nope"},
			rows:     []string{"stats.nope"},
			wantErr:  &UnknownNotD20SectionError{Key: "nope"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNotD20Tests(notD20Fixture(tc.sections, tc.rows))

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestNotD20Errors_Messages(t *testing.T) {
	assert.Equal(t, "not a d20 test section 'x' is not a section of the template", (&UnknownNotD20SectionError{Key: "x"}).Error())
	assert.Equal(t, "not a d20 test row 'x' is not a row of any section", (&UnknownNotD20RowError{Key: "x"}).Error())
}

func TestBuildRegistry_NotD20Tests(t *testing.T) {
	base := func(sections, rows []string) Template {
		return Template{
			ID:             "not_d20_v1",
			State:          StateActive,
			Fields:         FieldGroup{"stats": {Children: FieldGroup{"strength": layoutLeaf("")}}},
			NotD20Sections: sections,
			NotD20Rows:     rows,
		}
	}

	tests := []struct {
		name    string
		tmpl    Template
		wantMsg string
	}{
		{name: "Happy path: valid declarations boot", tmpl: base([]string{"stats"}, []string{"stats.strength"})},
		{name: "Edge: no declarations boot", tmpl: base(nil, nil)},
		{name: "Error: an unknown Section blocks boot", tmpl: base([]string{"nope"}, nil), wantMsg: "not a d20 test section 'nope'"},
		{name: "Error: an unknown Row blocks boot", tmpl: base(nil, []string{"stats.nope"}), wantMsg: "not a d20 test row 'stats.nope'"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := buildRegistry(engine.NewFormulaEngine(), []Template{tc.tmpl})

			if tc.wantMsg == "" {
				require.NoError(t, err)
				assert.NotNil(t, registry)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
			assert.Contains(t, err.Error(), `template "not_d20_v1"`)
		})
	}
}

func TestFantasyV1_DeclaresNoNotD20Tests(t *testing.T) {
	require.NoError(t, ValidateNotD20Tests(fantasyV1Template))

	assert.Empty(t, fantasyV1Template.NotD20Sections)
	assert.Empty(t, fantasyV1Template.NotD20Rows)
}

func TestCloneTemplate_DeepCopiesNotD20Tests(t *testing.T) {
	original := notD20Fixture([]string{"skills"}, []string{"baseArmorClass"})

	cloned := cloneTemplate(original)
	cloned.NotD20Sections[0] = "mutated"
	cloned.NotD20Rows[0] = "mutated"

	assert.Equal(t, []string{"skills"}, original.NotD20Sections)
	assert.Equal(t, []string{"baseArmorClass"}, original.NotD20Rows)
}

func TestRegistry_ServesNotD20TestsAsDeepCopies(t *testing.T) {
	registry, err := buildRegistry(engine.NewFormulaEngine(), []Template{
		notD20Fixture([]string{"skills"}, []string{"baseArmorClass"}),
	})
	require.NoError(t, err)

	fetched, ok := registry.FetchByID("layout_fixture")
	require.True(t, ok)
	fetched.NotD20Sections[0] = "mutated"
	fetched.NotD20Rows[0] = "mutated"
	registry.List()[0].NotD20Sections[0] = "mutated"

	again, _ := registry.FetchByID("layout_fixture")
	assert.Equal(t, []string{"skills"}, again.NotD20Sections)
	assert.Equal(t, []string{"baseArmorClass"}, again.NotD20Rows)
}

func TestTemplateToDTO_NotD20Tests(t *testing.T) {
	tests := []struct {
		name     string
		sections []string
		rows     []string
		want     *NotD20DTO
	}{
		{name: "Edge: nil declarations omit the marking"},
		{name: "Edge: empty declarations omit the marking", sections: []string{}, rows: []string{}},
		{
			name:     "Happy path: both lists are exposed",
			sections: []string{"skills"},
			rows:     []string{"baseArmorClass"},
			want:     &NotD20DTO{Section: []string{"skills"}, Row: []string{"baseArmorClass"}},
		},
		{
			name:     "Edge: only sections",
			sections: []string{"skills"},
			want:     &NotD20DTO{Section: []string{"skills"}},
		},
		{
			name: "Edge: only rows",
			rows: []string{"baseArmorClass"},
			want: &NotD20DTO{Row: []string{"baseArmorClass"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := notD20Fixture(tc.sections, tc.rows).ToDTO()

			if tc.want == nil {
				assert.Nil(t, dto.Dice)
				return
			}
			require.NotNil(t, dto.Dice)
			assert.Equal(t, tc.want, dto.Dice.NotD20)
		})
	}
}

func TestTemplateToDTO_NotD20DoesNotAliasTemplate(t *testing.T) {
	tmpl := notD20Fixture([]string{"skills"}, []string{"baseArmorClass"})

	dto := tmpl.ToDTO()
	dto.Dice.NotD20.Section[0] = "mutated"
	dto.Dice.NotD20.Row[0] = "mutated"

	assert.Equal(t, []string{"skills"}, tmpl.NotD20Sections)
	assert.Equal(t, []string{"baseArmorClass"}, tmpl.NotD20Rows)
}

func TestTemplateDTO_MarshalJSON_NotD20Tests(t *testing.T) {
	t.Run("Edge: a marking without dice defaults still emits dice", func(t *testing.T) {
		raw, err := json.Marshal(notD20Fixture([]string{"skills"}, []string{"baseArmorClass"}).ToDTO())

		require.NoError(t, err)
		assert.Contains(t, string(raw), `"dice":{"not_d20":{"section":["skills"],"row":["baseArmorClass"]}}`)
	})

	t.Run("Happy path: defaults and the marking are emitted together", func(t *testing.T) {
		tmpl := notD20Fixture([]string{"skills"}, []string{"baseArmorClass"})
		tmpl.SectionDice = map[string]string{"skills": "1d20 + @.total"}
		tmpl.RowDice = map[string]string{"baseArmorClass": "1d20 + @"}

		raw, err := json.Marshal(tmpl.ToDTO())

		require.NoError(t, err)
		assert.Contains(t, string(raw), `"not_d20":{"section":["skills"],"row":["baseArmorClass"]}`)
		assert.Contains(t, string(raw), `"section":{"skills":"1d20 + @.total"}`)
		assert.Contains(t, string(raw), `"row":{"baseArmorClass":"1d20 + @"}`)
	})

	t.Run("Edge: an empty list side is omitted", func(t *testing.T) {
		raw, err := json.Marshal(notD20Fixture(nil, []string{"baseArmorClass"}).ToDTO())

		require.NoError(t, err)
		assert.Contains(t, string(raw), `"not_d20":{"row":["baseArmorClass"]}`)
		assert.NotContains(t, string(raw), `"section":[]`)
	})
}
