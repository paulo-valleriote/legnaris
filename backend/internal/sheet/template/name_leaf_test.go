package template

import (
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nameLeafFixture returns a Template whose identity group holds a text naming
// leaf and a number leaf, with nameLeaf as its declared NameLeaf.
func nameLeafFixture(nameLeaf string) Template {
	return Template{
		ID:       "naming",
		NameLeaf: nameLeaf,
		Fields: FieldGroup{
			"identity": {Children: FieldGroup{
				"characterName": {Schema: &FieldSchema{Type: fieldTypeText, Label: "Name", IsEditable: true}},
				"level":         {Schema: &FieldSchema{Type: "number", Label: "Level", IsEditable: true}},
			}},
		},
	}
}

// TestSuggestName covers the derivation of the Sheet name from generated values.
func TestSuggestName(t *testing.T) {
	tests := []struct {
		name     string
		nameLeaf string
		values   map[string]any
		want     string
	}{
		{name: "Edge: a template without a NameLeaf suggests nothing", nameLeaf: "", values: map[string]any{"identity": map[string]any{"characterName": "Aragorn"}}, want: ""},
		{name: "Edge: nil values suggest nothing", nameLeaf: "identity.characterName", values: nil, want: ""},
		{name: "Edge: empty values suggest nothing", nameLeaf: "identity.characterName", values: map[string]any{}, want: ""},
		{name: "Edge: a missing leaf suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"identity": map[string]any{"level": 3}}, want: ""},
		{name: "Edge: a missing parent group suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"stats": map[string]any{}}, want: ""},
		{name: "Edge: an intermediate that is not a map suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"identity": "Aragorn"}, want: ""},
		{name: "Edge: an empty string suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"identity": map[string]any{"characterName": ""}}, want: ""},
		{name: "Edge: a whitespace-only string suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"identity": map[string]any{"characterName": " \t\n "}}, want: ""},
		{name: "Edge: a non-string value suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"identity": map[string]any{"characterName": 42}}, want: ""},
		{name: "Edge: a nil value suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"identity": map[string]any{"characterName": nil}}, want: ""},
		{name: "Edge: a list value suggests nothing", nameLeaf: "identity.characterName", values: map[string]any{"identity": map[string]any{"characterName": []string{"Aragorn"}}}, want: ""},
		{name: "Happy path: the naming leaf is trimmed and returned", nameLeaf: "identity.characterName", values: map[string]any{"identity": map[string]any{"characterName": "  Aragorn  "}}, want: "Aragorn"},
		{name: "Happy path: a single segment leaf is read at the root", nameLeaf: "name", values: map[string]any{"name": "Boromir"}, want: "Boromir"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := nameLeafFixture(tc.nameLeaf)
			assert.Equal(t, tc.want, tmpl.SuggestName(tc.values))
		})
	}
}

// TestValidateNameLeaf covers the boot check that a declared NameLeaf is a text leaf.
func TestValidateNameLeaf(t *testing.T) {
	tests := []struct {
		name     string
		nameLeaf string
		wantErr  error
	}{
		{name: "Edge: an empty NameLeaf is valid", nameLeaf: ""},
		{name: "Edge: a leaf that was renamed away fails", nameLeaf: "identity.heroName", wantErr: ErrNameLeafMissing},
		{name: "Edge: a leaf that was moved away fails", nameLeaf: "name", wantErr: ErrNameLeafMissing},
		{name: "Edge: a group path is not a leaf", nameLeaf: "identity", wantErr: ErrNameLeafMissing},
		{name: "Edge: a path through a leaf fails", nameLeaf: "identity.characterName.first", wantErr: ErrNameLeafMissing},
		{name: "Edge: a trailing separator fails", nameLeaf: "identity.", wantErr: ErrNameLeafMissing},
		{name: "Edge: a number leaf is not a name", nameLeaf: "identity.level", wantErr: ErrNameLeafNotText},
		{name: "Happy path: a text leaf is valid", nameLeaf: "identity.characterName"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNameLeaf(nameLeafFixture(tc.nameLeaf))
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// TestFantasyV1_NameLeaf pins the fantasy template's naming leaf and that it boots.
func TestFantasyV1_NameLeaf(t *testing.T) {
	var fantasy Template
	for _, tmpl := range DefaultTemplates() {
		if tmpl.ID == "fantasy_v1" {
			fantasy = tmpl
		}
	}
	require.Equal(t, "fantasy_v1", fantasy.ID)
	assert.Equal(t, "identity.characterName", fantasy.NameLeaf)
	assert.NoError(t, ValidateNameLeaf(fantasy))
	assert.Equal(t, "Aragorn", fantasy.SuggestName(map[string]any{"identity": map[string]any{"characterName": "Aragorn"}}))
}

// TestBuildRegistry_RejectsBrokenNameLeaf checks that a template whose naming leaf
// no longer resolves stops the registry from booting.
func TestBuildRegistry_RejectsBrokenNameLeaf(t *testing.T) {
	tests := []struct {
		name     string
		nameLeaf string
		wantErr  error
	}{
		{name: "Edge: a renamed naming leaf stops the boot", nameLeaf: "identity.heroName", wantErr: ErrNameLeafMissing},
		{name: "Edge: a number naming leaf stops the boot", nameLeaf: "identity.level", wantErr: ErrNameLeafNotText},
		{name: "Happy path: a valid naming leaf boots", nameLeaf: "identity.characterName"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildRegistry(engine.NewFormulaEngine(), []Template{nameLeafFixture(tc.nameLeaf)})
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}
