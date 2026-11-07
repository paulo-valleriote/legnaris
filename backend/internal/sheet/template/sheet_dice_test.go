package template

import (
	"fmt"
	"testing"

	"legnaris/backend/internal/dice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strp(s string) *string { return &s }

func overrides(n int) map[string]*string {
	out := make(map[string]*string, n)
	for i := range n {
		out[fmt.Sprintf("ghost%d", i)] = nil
	}
	return out
}

func TestValidateSheetDice(t *testing.T) {
	tests := []struct {
		name      string
		section   map[string]*string
		row       map[string]*string
		wantErr   error
		wantClass dice.Class
	}{
		{name: "Edge: nil maps are valid"},
		{name: "Edge: empty maps are valid", section: map[string]*string{}, row: map[string]*string{}},
		{
			name:    "Happy path: section and row overrides that fit",
			section: map[string]*string{"skills": strp("1d20 + @.total"), "stats": strp("1d20 + @")},
			row:     map[string]*string{"baseArmorClass": strp("1d20 + @"), "stats.strength": strp("2d6 + stats.dexterity")},
		},
		{
			name:    "Edge: null means explicit no dice and still needs a known key",
			section: map[string]*string{"stats": nil},
			row:     map[string]*string{"stats.strength": nil},
		},
		{
			name:    "Edge: a section override fitting only some of its rows is accepted",
			section: map[string]*string{"config": strp("1d20 + @")},
		},
		{
			name: "Edge: a row override without any field reference fits any row",
			row:  map[string]*string{"config#armorClassAbility": strp("2d6")},
		},
		{
			name:    "Error: unknown section, even with a null value",
			section: map[string]*string{"nope": nil},
			wantErr: &UnknownDiceSectionError{Key: "nope"},
		},
		{
			name:    "Error: a Section with no rows is not a section",
			section: map[string]*string{"empty": strp("1d20")},
			wantErr: &UnknownDiceSectionError{Key: "empty"},
		},
		{
			name:    "Error: unknown row, even with a null value",
			row:     map[string]*string{"stats.nope": nil},
			wantErr: &UnknownDiceRowError{Key: "stats.nope"},
		},
		{
			name:    "Error: a bare section key is not a row",
			row:     map[string]*string{"stats": strp("1d20")},
			wantErr: &UnknownDiceRowError{Key: "stats"},
		},
		{
			name:    "Error: a table column is not a row",
			row:     map[string]*string{"skills.acrobatics.total": strp("1d20")},
			wantErr: &UnknownDiceRowError{Key: "skills.acrobatics.total"},
		},
		{
			name:      "Error: invalid section expression",
			section:   map[string]*string{"stats": strp("1d20 +")},
			wantErr:   &InvalidDiceError{Scope: diceScopeSection, Key: "stats"},
			wantClass: dice.ClassMissingTerm,
		},
		{
			name:      "Error: empty string is not the same as null",
			row:       map[string]*string{"stats.strength": strp("")},
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "stats.strength"},
			wantClass: dice.ClassEmpty,
		},
		{
			name:      "Error: section override fitting no row",
			section:   map[string]*string{"stats": strp("1d20 + @.total")},
			wantErr:   &InvalidDiceError{Scope: diceScopeSection, Key: "stats"},
			wantClass: dice.ClassUnknownField,
		},
		{
			name:      "Error: row override that does not fit its row",
			row:       map[string]*string{"skills.acrobatics": strp("1d20 + @")},
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "skills.acrobatics"},
			wantClass: dice.ClassUnknownField,
		},
		{
			name:    "Error: 65 entries across both maps are refused before any key is read",
			section: overrides(33),
			row:     overrides(32),
			wantErr: &TooManyDiceOverridesError{Count: 65, Max: MaxSheetDiceOverrides},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSheetDice(layoutFixture(), tc.section, tc.row)

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			if want, ok := tc.wantErr.(*InvalidDiceError); ok {
				var got *InvalidDiceError
				require.ErrorAs(t, err, &got)
				assert.Equal(t, want.Scope, got.Scope)
				assert.Equal(t, want.Key, got.Key)
				class, ok := dice.ClassOf(err)
				require.True(t, ok)
				assert.Equal(t, tc.wantClass, class)
				return
			}
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestValidateSheetDice_ReportsKeysInStableOrder(t *testing.T) {
	section := map[string]*string{"zzz": nil, "aaa": nil, "mmm": nil}

	for range 20 {
		err := ValidateSheetDice(layoutFixture(), section, nil)
		assert.Equal(t, &UnknownDiceSectionError{Key: "aaa"}, err)
	}
}

func TestValidateSheetDice_SectionsAreCheckedBeforeRows(t *testing.T) {
	err := ValidateSheetDice(layoutFixture(),
		map[string]*string{"nope": nil},
		map[string]*string{"stats.nope": nil},
	)

	assert.Equal(t, &UnknownDiceSectionError{Key: "nope"}, err)
}

func TestValidateSheetDice_DoesNotMutateInput(t *testing.T) {
	section := map[string]*string{"stats": strp("1d20 + @")}
	row := map[string]*string{"stats.strength": nil}

	require.NoError(t, ValidateSheetDice(layoutFixture(), section, row))

	assert.Equal(t, map[string]*string{"stats": strp("1d20 + @")}, section)
	assert.Equal(t, map[string]*string{"stats.strength": nil}, row)
}

func TestValidateSheetDice_CapCountsBothMaps(t *testing.T) {
	fields := FieldGroup{}
	rows := make(map[string]*string, MaxSheetDiceOverrides)
	for i := range MaxSheetDiceOverrides {
		name := fmt.Sprintf("f%d", i)
		fields[name] = layoutLeaf("")
		rows[name] = nil
	}
	tmpl := Template{ID: "wide", Fields: fields}

	t.Run("Edge: exactly 64 entries are accepted", func(t *testing.T) {
		assert.NoError(t, ValidateSheetDice(tmpl, nil, rows))
	})

	t.Run("Edge: 63 rows plus 1 section is still 64", func(t *testing.T) {
		delete(rows, "f0")
		defer func() { rows["f0"] = nil }()
		assert.NoError(t, ValidateSheetDice(tmpl, map[string]*string{"f0": nil}, rows))
	})

	t.Run("Error: 64 rows plus 1 section is 65", func(t *testing.T) {
		err := ValidateSheetDice(tmpl, map[string]*string{"f0": nil}, rows)
		assert.Equal(t, &TooManyDiceOverridesError{Count: 65, Max: MaxSheetDiceOverrides}, err)
	})
}
