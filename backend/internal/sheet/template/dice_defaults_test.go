package template

import (
	"encoding/json"
	"errors"
	"testing"

	"legnaris/backend/internal/dice"
	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func diceFixture(sectionDice, rowDice map[string]string) Template {
	tmpl := layoutFixture()
	tmpl.SectionDice = sectionDice
	tmpl.RowDice = rowDice
	return tmpl
}

func TestValidateDiceDefaults(t *testing.T) {
	tests := []struct {
		name      string
		section   map[string]string
		row       map[string]string
		wantErr   error
		wantClass dice.Class
	}{
		{name: "Edge: nil maps are valid"},
		{name: "Edge: empty maps are valid", section: map[string]string{}, row: map[string]string{}},
		{
			name:    "Happy path: section and row defaults that fit",
			section: map[string]string{"skills": "1d20 + @.total", "stats": "1d20 + @"},
			row:     map[string]string{"baseArmorClass": "1d20 + @", "stats.strength": "1d20 + stats.dexterity"},
		},
		{
			name: "Edge: a row default without any field reference fits any row",
			row:  map[string]string{"config#armorClassAbility": "2d6"},
		},
		{
			name:    "Edge: a section default fitting only some of its rows is accepted",
			section: map[string]string{"config": "1d20 + @"},
		},
		{
			name:    "Error: unknown section",
			section: map[string]string{"nope": "1d20"},
			wantErr: &UnknownDiceSectionError{Key: "nope"},
		},
		{
			name:    "Error: empty section key",
			section: map[string]string{"": "1d20"},
			wantErr: &UnknownDiceSectionError{Key: ""},
		},
		{
			name:    "Error: a Section with no rows is not a section",
			section: map[string]string{"empty": "1d20"},
			wantErr: &UnknownDiceSectionError{Key: "empty"},
		},
		{
			name:    "Error: unknown row key",
			row:     map[string]string{"stats.nope": "1d20"},
			wantErr: &UnknownDiceRowError{Key: "stats.nope"},
		},
		{
			name:    "Error: a bare section key is not a row",
			row:     map[string]string{"stats": "1d20"},
			wantErr: &UnknownDiceRowError{Key: "stats"},
		},
		{
			name:    "Error: a table column is not a row",
			row:     map[string]string{"skills.acrobatics.total": "1d20"},
			wantErr: &UnknownDiceRowError{Key: "skills.acrobatics.total"},
		},
		{
			name:      "Error: invalid section expression",
			section:   map[string]string{"stats": "1d20 +"},
			wantErr:   &InvalidDiceError{Scope: diceScopeSection, Key: "stats"},
			wantClass: dice.ClassMissingTerm,
		},
		{
			name:      "Error: invalid row expression",
			row:       map[string]string{"stats.strength": ""},
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "stats.strength"},
			wantClass: dice.ClassEmpty,
		},
		{
			name:      "Error: section default fits no row of the section",
			section:   map[string]string{"stats": "1d20 + @.total"},
			wantErr:   &InvalidDiceError{Scope: diceScopeSection, Key: "stats"},
			wantClass: dice.ClassUnknownField,
		},
		{
			name:      "Error: row default whose @ is not a field",
			row:       map[string]string{"skills.acrobatics": "1d20 + @"},
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "skills.acrobatics"},
			wantClass: dice.ClassUnknownField,
		},
		{
			name:      "Error: row default pointing at a group",
			row:       map[string]string{"stats.strength": "1d20 + skills"},
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "stats.strength"},
			wantClass: dice.ClassUnknownField,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDiceDefaults(diceFixture(tc.section, tc.row))

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			var invalid *InvalidDiceError
			if errors.As(tc.wantErr, &invalid) {
				var got *InvalidDiceError
				require.ErrorAs(t, err, &got)
				assert.Equal(t, invalid.Scope, got.Scope)
				assert.Equal(t, invalid.Key, got.Key)
				class, ok := dice.ClassOf(err)
				require.True(t, ok)
				assert.Equal(t, tc.wantClass, class)
				return
			}
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestValidateDiceDefaults_NonNumericField(t *testing.T) {
	tmpl := Template{
		ID: "text_fixture",
		Fields: FieldGroup{"identity": {Children: FieldGroup{
			"name":  {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true}},
			"flag":  {Schema: &FieldSchema{Type: fieldTypeBool, IsEditable: true}},
			"level": layoutLeaf(""),
		}}},
	}

	tests := []struct {
		name string
		row  map[string]string
	}{
		{name: "Error: @ resolving to a text field", row: map[string]string{"identity.name": "1d20 + @"}},
		{name: "Error: @ resolving to a bool field", row: map[string]string{"identity.flag": "1d20 + @"}},
		{name: "Error: explicit path to a text field", row: map[string]string{"identity.level": "1d20 + identity.name"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl.RowDice = tc.row
			err := ValidateDiceDefaults(tmpl)

			require.Error(t, err)
			assert.ErrorIs(t, err, dice.ErrNonNumericField)
			var invalid *InvalidDiceError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestValidateDiceDefaults_ReportsFirstErrorDeterministically(t *testing.T) {
	tmpl := diceFixture(
		map[string]string{"zeta": "1d20", "alpha": "1d20"},
		map[string]string{"nope.b": "1d20", "nope.a": "1d20"},
	)

	for range 20 {
		assert.Equal(t, &UnknownDiceSectionError{Key: "alpha"}, ValidateDiceDefaults(tmpl))
	}
}

func TestDiceErrors_Messages(t *testing.T) {
	assert.Equal(t, "dice section 'x' is not a section of the template", (&UnknownDiceSectionError{Key: "x"}).Error())
	assert.Equal(t, "dice row 'x' is not a row of any section", (&UnknownDiceRowError{Key: "x"}).Error())

	inner := dice.ErrEmpty
	err := &InvalidDiceError{Scope: diceScopeRow, Key: "x", Err: inner}
	assert.Equal(t, "invalid dice expression on row 'x': the expression is empty", err.Error())
	assert.ErrorIs(t, err, dice.ErrEmpty)
}

func TestBuildRegistry_DiceDefaults(t *testing.T) {
	base := func(section, row map[string]string) Template {
		return Template{
			ID:          "dice_v1",
			Fields:      FieldGroup{"stats": {Children: FieldGroup{"strength": layoutLeaf("")}}},
			SectionDice: section,
			RowDice:     row,
		}
	}

	tests := []struct {
		name    string
		tmpl    Template
		wantIs  error
		wantMsg string
	}{
		{name: "Happy path: valid defaults boot", tmpl: base(map[string]string{"stats": "1d20 + @"}, map[string]string{"stats.strength": "1d20 + @"})},
		{name: "Edge: no dice boots", tmpl: base(nil, nil)},
		{name: "Error: invalid expression blocks boot", tmpl: base(nil, map[string]string{"stats.strength": "1d20 +"}), wantIs: dice.ErrMissingTerm},
		{name: "Error: unknown section blocks boot", tmpl: base(map[string]string{"nope": "1d20"}, nil), wantMsg: "dice section 'nope'"},
		{name: "Error: unknown row blocks boot", tmpl: base(nil, map[string]string{"stats.nope": "1d20"}), wantMsg: "dice row 'stats.nope'"},
		{name: "Error: section default fitting no row blocks boot", tmpl: base(map[string]string{"stats": "1d20 + @.total"}, nil), wantIs: dice.ErrUnknownField},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := buildRegistry(engine.NewFormulaEngine(), []Template{tc.tmpl})

			if tc.wantIs == nil && tc.wantMsg == "" {
				require.NoError(t, err)
				assert.NotNil(t, registry)
				return
			}
			require.Error(t, err)
			if tc.wantIs != nil {
				assert.ErrorIs(t, err, tc.wantIs)
			}
			assert.Contains(t, err.Error(), tc.wantMsg)
			assert.Contains(t, err.Error(), `template "dice_v1"`)
		})
	}
}

func TestCloneTemplate_DeepCopiesDice(t *testing.T) {
	original := diceFixture(
		map[string]string{"skills": "1d20 + @.total"},
		map[string]string{"baseArmorClass": "1d20 + @"},
	)

	cloned := cloneTemplate(original)
	cloned.SectionDice["skills"] = "mutated"
	cloned.SectionDice["extra"] = "1d4"
	cloned.RowDice["baseArmorClass"] = "mutated"
	delete(cloned.RowDice, "baseArmorClass")

	assert.Equal(t, map[string]string{"skills": "1d20 + @.total"}, original.SectionDice)
	assert.Equal(t, map[string]string{"baseArmorClass": "1d20 + @"}, original.RowDice)
}

func TestCloneTemplate_NilDiceStaysNil(t *testing.T) {
	cloned := cloneTemplate(layoutFixture())

	assert.Nil(t, cloned.SectionDice)
	assert.Nil(t, cloned.RowDice)
}

func TestTemplateToDTO_Dice(t *testing.T) {
	tests := []struct {
		name    string
		section map[string]string
		row     map[string]string
		want    *DiceDTO
	}{
		{name: "Edge: nil maps omit dice"},
		{name: "Edge: empty maps omit dice", section: map[string]string{}, row: map[string]string{}},
		{
			name:    "Happy path: both maps are exposed",
			section: map[string]string{"skills": "1d20 + @.total"},
			row:     map[string]string{"baseArmorClass": "1d20 + @"},
			want: &DiceDTO{
				Section: map[string]string{"skills": "1d20 + @.total"},
				Row:     map[string]string{"baseArmorClass": "1d20 + @"},
			},
		},
		{
			name:    "Edge: only section defaults",
			section: map[string]string{"skills": "1d20"},
			want:    &DiceDTO{Section: map[string]string{"skills": "1d20"}},
		},
		{
			name: "Edge: only row defaults",
			row:  map[string]string{"baseArmorClass": "1d20"},
			want: &DiceDTO{Row: map[string]string{"baseArmorClass": "1d20"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := diceFixture(tc.section, tc.row).ToDTO()

			assert.Equal(t, tc.want, dto.Dice)
		})
	}
}

func TestTemplateToDTO_DiceDoesNotAliasTemplate(t *testing.T) {
	tmpl := diceFixture(
		map[string]string{"skills": "1d20 + @.total"},
		map[string]string{"baseArmorClass": "1d20 + @"},
	)

	dto := tmpl.ToDTO()
	dto.Dice.Section["skills"] = "mutated"
	dto.Dice.Row["other"] = "1d4"

	assert.Equal(t, map[string]string{"skills": "1d20 + @.total"}, tmpl.SectionDice)
	assert.Equal(t, map[string]string{"baseArmorClass": "1d20 + @"}, tmpl.RowDice)
}

func TestTemplateDTO_MarshalJSON_Dice(t *testing.T) {
	t.Run("Edge: a template without dice keeps its payload", func(t *testing.T) {
		raw, err := json.Marshal(layoutFixture().ToDTO())

		require.NoError(t, err)
		assert.NotContains(t, string(raw), `"dice"`)
	})

	t.Run("Edge: only the non-empty side is emitted", func(t *testing.T) {
		raw, err := json.Marshal(diceFixture(map[string]string{"skills": "1d20"}, nil).ToDTO())

		require.NoError(t, err)
		assert.Contains(t, string(raw), `"dice":{"section":{"skills":"1d20"}}`)
	})

	t.Run("Happy path: both sides are emitted", func(t *testing.T) {
		raw, err := json.Marshal(diceFixture(
			map[string]string{"skills": "1d20 + @.total"},
			map[string]string{"baseArmorClass": "1d20 + @"},
		).ToDTO())

		require.NoError(t, err)
		assert.Contains(t, string(raw), `"dice":{"section":{"skills":"1d20 + @.total"},"row":{"baseArmorClass":"1d20 + @"}}`)
	})
}

func TestRegistry_ServesDiceAsDeepCopies(t *testing.T) {
	registry, err := buildRegistry(engine.NewFormulaEngine(), []Template{diceFixture(
		map[string]string{"skills": "1d20 + @.total"},
		map[string]string{"baseArmorClass": "1d20 + @"},
	)})
	require.NoError(t, err)

	fetched, ok := registry.FetchByID("layout_fixture")
	require.True(t, ok)
	fetched.SectionDice["skills"] = "mutated"
	fetched.RowDice["baseArmorClass"] = "mutated"
	registry.List()[0].SectionDice["skills"] = "mutated"

	again, _ := registry.FetchByID("layout_fixture")
	assert.Equal(t, map[string]string{"skills": "1d20 + @.total"}, again.SectionDice)
	assert.Equal(t, map[string]string{"baseArmorClass": "1d20 + @"}, again.RowDice)
}

func TestFantasyV1_DiceDefaults(t *testing.T) {
	require.NoError(t, ValidateDiceDefaults(fantasyV1Template))

	assert.Equal(t, map[string]string{
		"skills":  "1d20 + @.total",
		"saves":   "1d20 + @",
		"attacks": "1d20 + @.bonus",
	}, fantasyV1Template.SectionDice)

	assert.Equal(t, map[string]string{
		"combat.initiative":  "1d20 + @",
		"stats.strength":     "1d20 + modifiers.strengthMod",
		"stats.dexterity":    "1d20 + modifiers.dexterityMod",
		"stats.constitution": "1d20 + modifiers.constitutionMod",
		"stats.intelligence": "1d20 + modifiers.intelligenceMod",
		"stats.wisdom":       "1d20 + modifiers.wisdomMod",
		"stats.charisma":     "1d20 + modifiers.charismaMod",
	}, fantasyV1Template.RowDice)
}

func TestDefaultTemplates_AllBootWithDice(t *testing.T) {
	for _, tmpl := range DefaultTemplates() {
		t.Run(tmpl.ID, func(t *testing.T) {
			assert.NoError(t, ValidateDiceDefaults(tmpl))
		})
	}

	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	assert.Len(t, registry.List(), len(DefaultTemplates()))
}

func TestRegistry_FantasyV1ExposesDiceInDTO(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	tmpl, ok := registry.FetchByID("fantasy_v1")
	require.True(t, ok)
	dto := tmpl.ToDTO()

	require.NotNil(t, dto.Dice)
	assert.Equal(t, "1d20 + @.total", dto.Dice.Section["skills"])
	assert.Equal(t, "1d20 + @", dto.Dice.Row["combat.initiative"])
}

func TestValidateDiceDefaults_UnknownFieldTypeIsNotNumeric(t *testing.T) {
	tmpl := Template{
		ID: "future_type",
		Fields: FieldGroup{"identity": {Children: FieldGroup{
			"mood": {Schema: &FieldSchema{Type: "enum", IsEditable: true}},
		}}},
		RowDice: map[string]string{"identity.mood": "1d20 + @"},
	}

	err := ValidateDiceDefaults(tmpl)

	require.Error(t, err)
	assert.ErrorIs(t, err, dice.ErrNonNumericField)
}
