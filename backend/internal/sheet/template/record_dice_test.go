package template

import (
	"errors"
	"testing"

	"legnaris/backend/internal/dice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recordDiceTemplate(listDice string) Template {
	tmpl := rowIdentityTemplate()
	items := tmpl.Fields["inventory"].Children["items"]
	items.List.Dice = listDice
	tmpl.Fields["inventory"].Children["items"] = items
	return tmpl
}

func TestValidateDiceDefaults_ListDice(t *testing.T) {
	tests := []struct {
		name      string
		listDice  string
		wantErr   error
		wantClass dice.Class
	}{
		{
			name:     "Happy path: a list default fits a numeric shape column through @",
			listDice: "1d20 + @.quantity",
		},
		{
			name:     "Happy path: a list default with only dice fits every record",
			listDice: "2d6",
		},
		{
			name:      "Error: a list default over a text column does not fit",
			listDice:  "1d20 + @.name",
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "inventory.items"},
			wantClass: dice.ClassNonNumericField,
		},
		{
			name:      "Error: a list default over an unknown column does not fit",
			listDice:  "1d20 + @.missing",
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "inventory.items"},
			wantClass: dice.ClassUnknownField,
		},
		{
			name:      "Error: an unreadable list default",
			listDice:  "1d20 +",
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "inventory.items"},
			wantClass: dice.ClassMissingTerm,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDiceDefaults(recordDiceTemplate(tc.listDice))

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			var want *InvalidDiceError
			require.ErrorAs(t, tc.wantErr, &want)
			var got *InvalidDiceError
			require.ErrorAs(t, err, &got)
			assert.Equal(t, want.Scope, got.Scope)
			assert.Equal(t, want.Key, got.Key)
			class, ok := dice.ClassOf(err)
			require.True(t, ok)
			assert.Equal(t, tc.wantClass, class)
		})
	}
}

func TestValidateDiceDefaults_SectionOverAList(t *testing.T) {
	tmpl := rowIdentityTemplate()
	tmpl.SectionDice = map[string]string{"inventory": "1d20 + @.quantity"}

	require.NoError(t, ValidateDiceDefaults(tmpl))

	tmpl.SectionDice = map[string]string{"inventory": "1d20 + @.name"}
	err := ValidateDiceDefaults(tmpl)
	require.Error(t, err)
	assert.Equal(t, dice.ClassNonNumericField, classOf(t, err))
}

func TestValidateDiceDefaults_ListOnlySection(t *testing.T) {
	tmpl := Template{Fields: FieldGroup{
		"items": {List: &ListSchema{
			Label:    "Items",
			Capacity: 1,
			Shape:    FieldGroup{"quantity": rowNumberLeaf()},
		}},
	}}
	tmpl.SectionDice = map[string]string{"items": "1d20 + @.quantity"}

	require.NoError(t, ValidateDiceDefaults(tmpl))
}

func TestValidateNotD20Tests_ListOnlySection(t *testing.T) {
	tmpl := Template{
		Fields: FieldGroup{
			"items": {List: &ListSchema{
				Label:    "Items",
				Capacity: 1,
				Shape:    FieldGroup{"quantity": rowNumberLeaf()},
			}},
		},
		NotD20Sections: []string{"items"},
	}

	require.NoError(t, ValidateNotD20Tests(tmpl))

	tmpl.NotD20Sections = []string{"nope"}
	assert.Equal(t, &UnknownNotD20SectionError{Key: "nope"}, ValidateNotD20Tests(tmpl))
}

func TestValidateNotD20Tests_ListMarkingNeedsNoRowKey(t *testing.T) {
	tmpl := recordDiceTemplate("")
	items := tmpl.Fields["inventory"].Children["items"]
	items.List.NotD20 = true
	tmpl.Fields["inventory"].Children["items"] = items

	require.NoError(t, ValidateNotD20Tests(tmpl))

	tmpl.NotD20Rows = []string{"inventory.items"}
	assert.Equal(t, &UnknownNotD20RowError{Key: "inventory.items"}, ValidateNotD20Tests(tmpl))
}

func TestValidateSheetDice_RecordRows(t *testing.T) {
	tmpl := recordDiceTemplate("")

	tests := []struct {
		name      string
		section   map[string]*string
		row       map[string]*string
		wantErr   error
		wantClass dice.Class
	}{
		{
			name: "Happy path: a record row override fits its shape",
			row:  map[string]*string{"inventory.items." + rowIdentityRecordID: strp("1d20 + @.quantity")},
		},
		{
			name: "Happy path: a record row override of only dice",
			row:  map[string]*string{"inventory.items." + rowIdentityRecordID: strp("2d6")},
		},
		{
			name: "Edge: a well-formed id with no stored Record still saves",
			row:  map[string]*string{"inventory.items.rffffffffffffffffffffffffffffffff": strp("1d20")},
		},
		{
			name: "Happy path: a record row accepts an explicit no-dice null",
			row:  map[string]*string{"inventory.items." + rowIdentityRecordID: nil},
		},
		{
			name:    "Error: a malformed record id is not a row",
			row:     map[string]*string{"inventory.items.rbad": strp("1d20")},
			wantErr: &UnknownDiceRowError{Key: "inventory.items.rbad"},
		},
		{
			name:    "Error: an unknown list is not a row",
			row:     map[string]*string{"unknown.items." + rowIdentityRecordID: strp("1d20")},
			wantErr: &UnknownDiceRowError{Key: "unknown.items." + rowIdentityRecordID},
		},
		{
			name:    "Error: the list path alone is not a player row",
			row:     map[string]*string{"inventory.items": strp("1d20")},
			wantErr: &UnknownDiceRowError{Key: "inventory.items"},
		},
		{
			name:    "Error: a record field path is not a row",
			row:     map[string]*string{"inventory.items." + rowIdentityRecordID + ".quantity": strp("1d20")},
			wantErr: &UnknownDiceRowError{Key: "inventory.items." + rowIdentityRecordID + ".quantity"},
		},
		{
			name:      "Error: an unknown shape column does not fit",
			row:       map[string]*string{"inventory.items." + rowIdentityRecordID: strp("1d20 + @.missing")},
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "inventory.items." + rowIdentityRecordID},
			wantClass: dice.ClassUnknownField,
		},
		{
			name:      "Error: a text shape column is not numeric",
			row:       map[string]*string{"inventory.items." + rowIdentityRecordID: strp("1d20 + @.name")},
			wantErr:   &InvalidDiceError{Scope: diceScopeRow, Key: "inventory.items." + rowIdentityRecordID},
			wantClass: dice.ClassNonNumericField,
		},
		{
			name:    "Happy path: a section override over an empty list still fits the shape",
			section: map[string]*string{"inventory": strp("1d20 + @.quantity")},
		},
		{
			name:      "Error: a section override over a list fits no Row",
			section:   map[string]*string{"inventory": strp("1d20 + @.missing")},
			wantErr:   &InvalidDiceError{Scope: diceScopeSection, Key: "inventory"},
			wantClass: dice.ClassUnknownField,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSheetDice(tmpl, tc.section, tc.row)

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			var want *InvalidDiceError
			if errors.As(tc.wantErr, &want) {
				var got *InvalidDiceError
				require.ErrorAs(t, err, &got)
				assert.Equal(t, want.Scope, got.Scope)
				assert.Equal(t, want.Key, got.Key)
				assert.Equal(t, tc.wantClass, classOf(t, err))
				return
			}
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestValidateSheetDice_RecordRowsCountTowardTheCap(t *testing.T) {
	tmpl := recordDiceTemplate("")
	rows := make(map[string]*string, MaxSheetDiceOverrides+1)
	for i := range MaxSheetDiceOverrides + 1 {
		id := "r" + leftPadHex(i)
		rows["inventory.items."+id] = nil
	}

	err := ValidateSheetDice(tmpl, nil, rows)
	assert.Equal(t, &TooManyDiceOverridesError{Count: MaxSheetDiceOverrides + 1, Max: MaxSheetDiceOverrides}, err)
}

func leftPadHex(value int) string {
	const hexDigits = "0123456789abcdef"
	out := []byte("00000000000000000000000000000000")
	for index := len(out) - 1; index >= 0 && value > 0; index-- {
		out[index] = hexDigits[value%16]
		value /= 16
	}
	return string(out)
}

func classOf(t *testing.T, err error) dice.Class {
	t.Helper()
	class, ok := dice.ClassOf(err)
	require.True(t, ok)
	return class
}
