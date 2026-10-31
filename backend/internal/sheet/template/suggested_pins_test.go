package template

import (
	"encoding/json"
	"strings"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSuggestedPins(t *testing.T) {
	sevenRows := Template{Fields: FieldGroup{"s": {Children: FieldGroup{
		"a": layoutLeaf(""), "b": layoutLeaf(""), "c": layoutLeaf(""), "d": layoutLeaf(""),
		"e": layoutLeaf(""), "f": layoutLeaf(""), "g": layoutLeaf(""),
	}}}}

	tests := []struct {
		name    string
		pins    []string
		tmpl    Template
		wantErr error
	}{
		{name: "Edge: nil list is valid", tmpl: layoutFixture()},
		{name: "Edge: empty list is valid", pins: []string{}, tmpl: layoutFixture()},
		{
			name: "Happy path: lone leaf, table row and exclusive row keys are accepted in suggestion order",
			pins: []string{"config#armorClassAbility", "stats.strength", "skills.acrobatics", "baseArmorClass"},
			tmpl: layoutFixture(),
		},
		{
			name: "Edge: exactly the strip capacity is accepted",
			pins: []string{"s.a", "s.b", "s.c", "s.d", "s.e", "s.f"},
			tmpl: sevenRows,
		},
		{
			name:    "Error: one more than the strip capacity",
			pins:    []string{"s.a", "s.b", "s.c", "s.d", "s.e", "s.f", "s.g"},
			tmpl:    sevenRows,
			wantErr: &TooManySuggestedPinsError{Count: 7, Max: MaxSuggestedPins},
		},
		{
			name:    "Error: unknown key",
			pins:    []string{"stats.nope"},
			tmpl:    layoutFixture(),
			wantErr: &UnknownSuggestedPinError{Key: "stats.nope"},
		},
		{
			name:    "Error: empty string key",
			pins:    []string{""},
			tmpl:    layoutFixture(),
			wantErr: &UnknownSuggestedPinError{Key: ""},
		},
		{
			name:    "Error: a bare section key is not a row",
			pins:    []string{"stats"},
			tmpl:    layoutFixture(),
			wantErr: &UnknownSuggestedPinError{Key: "stats"},
		},
		{
			name:    "Error: a table column is not a row",
			pins:    []string{"skills.acrobatics.bonus"},
			tmpl:    layoutFixture(),
			wantErr: &UnknownSuggestedPinError{Key: "skills.acrobatics.bonus"},
		},
		{
			name:    "Error: an exclusive member is not a row, only its group is",
			pins:    []string{"config.acUsesDex"},
			tmpl:    layoutFixture(),
			wantErr: &UnknownSuggestedPinError{Key: "config.acUsesDex"},
		},
		{
			name:    "Error: duplicate key",
			pins:    []string{"stats.strength", "baseArmorClass", "stats.strength"},
			tmpl:    layoutFixture(),
			wantErr: &DuplicateSuggestedPinError{Key: "stats.strength"},
		},
		{
			name:    "Edge: any pin on a template with no rows is unknown",
			pins:    []string{"x"},
			tmpl:    Template{ID: "empty"},
			wantErr: &UnknownSuggestedPinError{Key: "x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := tc.tmpl
			tmpl.SuggestedPins = tc.pins

			err := ValidateSuggestedPins(tmpl)

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestSuggestedPinsErrors_Messages(t *testing.T) {
	assert.Equal(t, "suggested pin 'x' is not a row of any section", (&UnknownSuggestedPinError{Key: "x"}).Error())
	assert.Equal(t, "duplicate suggested pin 'x'", (&DuplicateSuggestedPinError{Key: "x"}).Error())
	assert.Equal(t, "7 suggested pins exceed the maximum of 6", (&TooManySuggestedPinsError{Count: 7, Max: 6}).Error())
}

func TestFantasyV1_SuggestedPins(t *testing.T) {
	require.NoError(t, ValidateSuggestedPins(fantasyV1Template))
	require.NotEmpty(t, fantasyV1Template.SuggestedPins)

	for _, key := range fantasyV1Template.SuggestedPins {
		path := strings.Split(key, ".")
		require.Len(t, path, 2, "suggested pin %s must be section.field", key)
		node := fantasyV1Template.Fields[path[0]].Children[path[1]]
		require.NotNil(t, node.Schema, "suggested pin %s must be a lone leaf", key)
		assert.Equal(t, fieldTypeNumber, node.Schema.Type, "suggested pin %s must be numeric", key)
	}
}

func TestBuildRegistry_SuggestedPins(t *testing.T) {
	leafSection := func(pins []string) Template {
		return Template{
			ID:            "pins_v1",
			Fields:        FieldGroup{"stats": {Children: FieldGroup{"strength": layoutLeaf("")}}},
			SuggestedPins: pins,
		}
	}

	tests := []struct {
		name    string
		tmpl    Template
		wantErr error
	}{
		{name: "Happy path: valid suggestions boot", tmpl: leafSection([]string{"stats.strength"})},
		{name: "Edge: no suggestions boot", tmpl: leafSection(nil)},
		{
			name:    "Error: unknown suggestion fails boot with the typed error",
			tmpl:    leafSection([]string{"stats.ghost"}),
			wantErr: &UnknownSuggestedPinError{Key: "stats.ghost"},
		},
		{
			name:    "Error: duplicate suggestion fails boot with the typed error",
			tmpl:    leafSection([]string{"stats.strength", "stats.strength"}),
			wantErr: &DuplicateSuggestedPinError{Key: "stats.strength"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := buildRegistry(engine.NewFormulaEngine(), []Template{tc.tmpl})

			if tc.wantErr == nil {
				require.NoError(t, err)
				assert.NotNil(t, registry)
				return
			}
			assert.Nil(t, registry)
			var unknown *UnknownSuggestedPinError
			var duplicate *DuplicateSuggestedPinError
			switch tc.wantErr.(type) {
			case *UnknownSuggestedPinError:
				require.ErrorAs(t, err, &unknown)
				assert.Equal(t, tc.wantErr, unknown)
			case *DuplicateSuggestedPinError:
				require.ErrorAs(t, err, &duplicate)
				assert.Equal(t, tc.wantErr, duplicate)
			}
			assert.ErrorContains(t, err, tc.tmpl.ID)
		})
	}
}

func TestRegistry_SuggestedPinsAreDeepCopied(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	first, ok := registry.FetchByID(fantasyV1Template.ID)
	require.True(t, ok)
	require.NotEmpty(t, first.SuggestedPins)
	first.SuggestedPins[0] = "tampered"

	listed := registry.List()
	listed[0].SuggestedPins[0] = "tampered"

	second, ok := registry.FetchByID(fantasyV1Template.ID)
	require.True(t, ok)
	assert.Equal(t, fantasyV1Template.SuggestedPins, second.SuggestedPins)
	assert.Equal(t, fantasyV1Template.SuggestedPins, DefaultTemplates()[0].SuggestedPins)
}

func TestTemplateDTO_SuggestedPins(t *testing.T) {
	topLevel := func(t *testing.T, dto TemplateDTO) map[string]json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(dto)
		require.NoError(t, err)
		var decoded map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &decoded))
		return decoded
	}

	t.Run("Happy path: fantasy_v1 exposes its suggestions in order", func(t *testing.T) {
		decoded := topLevel(t, fantasyV1Template.ToDTO())

		var pins []string
		require.NoError(t, json.Unmarshal(decoded["suggested_pins"], &pins))
		assert.Equal(t, fantasyV1Template.SuggestedPins, pins)
	})

	t.Run("Edge: an empty or nil list is omitted from the payload", func(t *testing.T) {
		for _, pins := range [][]string{nil, {}} {
			dto := TemplateDTO{ID: "t", Name: "T", Fields: FieldGroupDTO{}, SuggestedPins: pins}
			assert.NotContains(t, topLevel(t, dto), "suggested_pins")
		}
	})

	t.Run("Edge: ToDTO does not alias the template's list", func(t *testing.T) {
		tmpl := Template{ID: "t", SuggestedPins: []string{"a.b"}}
		dto := tmpl.ToDTO()
		dto.SuggestedPins[0] = "tampered"

		assert.Equal(t, []string{"a.b"}, tmpl.SuggestedPins)
	})

	t.Run("Edge: a template without suggestions keeps the original payload keys", func(t *testing.T) {
		decoded := topLevel(t, Template{ID: "t", Name: "T"}.ToDTO())

		assert.ElementsMatch(t, []string{"id", "name", "fields", "default"}, mapKeys(decoded))
	})
}

func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
