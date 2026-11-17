package template

import (
	"encoding/json"
	"strings"
	"testing"

	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldSchemaDTO_MarshalJSON_GenerationMetadata(t *testing.T) {
	tests := []struct {
		name   string
		schema FieldSchema
		want   string
	}{
		{
			name:   "Happy path: a generatable number carries its range",
			schema: FieldSchema{Type: fieldTypeNumber, IsEditable: true, Generatable: true, Min: f64(3), Max: f64(20)},
			want:   `{"type":"number","label":"","is_editable":true,"default":0,"generatable":true,"min":3,"max":20}`,
		},
		{
			name:   "Happy path: a generatable text carries its max length",
			schema: FieldSchema{Type: fieldTypeText, IsEditable: true, Generatable: true, MaxLength: intPtr(60)},
			want:   `{"type":"text","label":"","is_editable":true,"default":0,"generatable":true,"max_length":60}`,
		},
		{
			name: "Happy path: a kind restriction is emitted as generatable_kinds",
			schema: FieldSchema{
				Type: fieldTypeText, IsEditable: true, Generatable: true,
				GeneratableKinds: []plan.SheetKind{plan.KindCharacter},
			},
			want: `{"type":"text","label":"","is_editable":true,"default":0,"generatable":true,"generatable_kinds":["character"]}`,
		},
		{
			name:   "Edge: an explicit zero minimum is emitted, not dropped",
			schema: FieldSchema{Type: fieldTypeNumber, IsEditable: true, Min: f64(0)},
			want:   `{"type":"number","label":"","is_editable":true,"default":0,"min":0}`,
		},
		{
			name:   "Edge: no metadata keeps the legacy payload",
			schema: FieldSchema{Type: fieldTypeNumber, IsEditable: true},
			want:   `{"type":"number","label":"","is_editable":true,"default":0}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := tc.schema.toDTO()
			require.NotNil(t, dto)
			raw, err := json.Marshal(dto)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(raw))
		})
	}
}

func TestGenerationMetadata_CopiesDoNotAlias(t *testing.T) {
	original := Template{Fields: FieldGroup{"stats": {Children: FieldGroup{
		"strength": {Schema: &FieldSchema{
			Type: fieldTypeNumber, IsEditable: true, Generatable: true, Min: f64(3), Max: f64(20),
			GeneratableKinds: []plan.SheetKind{plan.KindCharacter},
		}},
		"name": genLeaf(FieldSchema{Type: fieldTypeText, MaxLength: intPtr(40)}),
	}}}}

	cloned := cloneTemplate(original)
	dto := original.ToDTO()

	strength := original.Fields["stats"].Children["strength"].Schema
	name := original.Fields["stats"].Children["name"].Schema
	clonedStrength := cloned.Fields["stats"].Children["strength"].Schema
	clonedName := cloned.Fields["stats"].Children["name"].Schema
	dtoStrength := dto.Fields["stats"].Children["strength"].Schema
	dtoName := dto.Fields["stats"].Children["name"].Schema

	require.NotSame(t, strength.Min, clonedStrength.Min)
	require.NotSame(t, strength.Max, clonedStrength.Max)
	require.NotSame(t, name.MaxLength, clonedName.MaxLength)
	require.NotSame(t, strength.Min, dtoStrength.Min)
	require.NotSame(t, strength.Max, dtoStrength.Max)
	require.NotSame(t, name.MaxLength, dtoName.MaxLength)
	require.NotSame(t, &strength.GeneratableKinds[0], &clonedStrength.GeneratableKinds[0])
	require.NotSame(t, &strength.GeneratableKinds[0], &dtoStrength.GeneratableKinds[0])

	*clonedStrength.Min = -99
	*clonedName.MaxLength = 1
	*dtoStrength.Max = -99
	*dtoName.MaxLength = 1
	clonedStrength.GeneratableKinds[0] = plan.KindEnemy
	dtoStrength.GeneratableKinds[0] = plan.KindEnemy

	assert.Equal(t, 3.0, *strength.Min)
	assert.Equal(t, 20.0, *strength.Max)
	assert.Equal(t, 40, *name.MaxLength)
	assert.Equal(t, []plan.SheetKind{plan.KindCharacter}, strength.GeneratableKinds)
}

func TestFantasyV1_GenerationMetadata(t *testing.T) {
	schemas, paths := collectLeaves(fantasyV1Template.Fields)
	require.NotEmpty(t, schemas)

	generatable := 0
	for i, schema := range schemas {
		key := paths[i].String()
		isRules := strings.HasPrefix(key, fieldNameRules+".")

		if !schema.IsEditable || isRules {
			assert.False(t, schema.Generatable, "leaf %s must not be generatable", key)
			continue
		}

		require.True(t, schema.Generatable, "editable leaf %s must be generatable", key)
		generatable++
		switch schema.Type {
		case fieldTypeNumber:
			require.NotNil(t, schema.Min, "number leaf %s needs a minimum", key)
			require.NotNil(t, schema.Max, "number leaf %s needs a maximum", key)
			assert.Nil(t, schema.MaxLength, "number leaf %s must not carry a max length", key)
		case fieldTypeText:
			require.NotNil(t, schema.MaxLength, "text leaf %s needs a max length", key)
			assert.Nil(t, schema.Min, "text leaf %s must not carry a range", key)
			assert.Greater(t, *schema.MaxLength, 0, "text leaf %s has a non-positive max length", key)
		default:
			t.Errorf("generatable leaf %s has unexpected type %q", key, schema.Type)
		}
	}
	assert.Positive(t, generatable)

	playerName := fantasyV1Template.Fields[fieldNameIdentity].Children["playerName"].Schema
	require.NotNil(t, playerName)
	assert.Equal(t, []plan.SheetKind{plan.KindCharacter}, playerName.GeneratableKinds)
	assert.True(t, playerName.GeneratableFor(plan.KindCharacter))
	assert.False(t, playerName.GeneratableFor(plan.KindNPC))
	assert.False(t, playerName.GeneratableFor(plan.KindEnemy))

	require.NoError(t, ValidateGenerationMetadata(fantasyV1Template))
}

func TestFantasyV1_GenerationMetadataPayload(t *testing.T) {
	raw, err := json.Marshal(fantasyV1Template.ToDTO())
	require.NoError(t, err)

	var payload struct {
		Fields map[string]struct {
			Children map[string]struct {
				Schema map[string]any `json:"schema"`
			} `json:"children"`
		} `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))

	identity := payload.Fields["identity"].Children
	assert.Equal(t, true, identity["characterName"].Schema["generatable"])
	assert.Equal(t, float64(60), identity["characterName"].Schema["max_length"])
	assert.Equal(t, []any{"character"}, identity["playerName"].Schema["generatable_kinds"])
	assert.Equal(t, float64(1), identity["level"].Schema["min"])
	assert.Equal(t, float64(20), identity["level"].Schema["max"])

	rulesTuning := payload.Fields["rules"].Children["modifierBase"].Schema
	assert.NotContains(t, rulesTuning, "generatable")
	assert.NotContains(t, rulesTuning, "min")
}
