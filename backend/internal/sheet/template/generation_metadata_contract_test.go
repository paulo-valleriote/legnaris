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
			name: "Happy path: a generatable leaf that prefers its default is emitted as prefer_default",
			schema: FieldSchema{
				Type: fieldTypeNumber, IsEditable: true, Generatable: true,
				Default: 1, Min: f64(1), Max: f64(20), PreferDefault: true,
			},
			want: `{"type":"number","label":"","is_editable":true,"default":1,"generatable":true,"prefer_default":true,"min":1,"max":20}`,
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
			GeneratableKinds: []plan.SheetKind{plan.KindCharacter}, PreferDefault: true,
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

	assert.InDelta(t, 3.0, *strength.Min, 1e-9)
	assert.InDelta(t, 20.0, *strength.Max, 1e-9)
	assert.Equal(t, 40, *name.MaxLength)
	assert.Equal(t, []plan.SheetKind{plan.KindCharacter}, strength.GeneratableKinds)
	assert.True(t, strength.PreferDefault)
	assert.True(t, clonedStrength.PreferDefault)
	assert.True(t, dtoStrength.PreferDefault)
}

func TestGenerationMetadata_ClassSubclassCopiesDoNotAlias(t *testing.T) {
	original := Template{
		ClassSubclass: &ClassSubclassRule{
			Class: []string{"identity", "class"}, Subclass: []string{"identity", "subclass"},
			Level: []string{"identity", "level"}, SubclassLevel: 3,
		},
		Fields: FieldGroup{"identity": {Children: FieldGroup{
			"class": genLeaf(FieldSchema{
				Type: fieldTypeText, Generatable: true, GenerationEnum: []string{"Fighter"},
			}),
		}}},
	}

	cloned := cloneTemplate(original)
	require.NotSame(t, original.ClassSubclass, cloned.ClassSubclass)
	require.NotSame(t, &original.ClassSubclass.Class[0], &cloned.ClassSubclass.Class[0])
	require.NotSame(t, &original.ClassSubclass.Subclass[0], &cloned.ClassSubclass.Subclass[0])
	require.NotSame(t, &original.ClassSubclass.Level[0], &cloned.ClassSubclass.Level[0])

	originalEnum := original.Fields["identity"].Children["class"].Schema.GenerationEnum
	clonedEnum := cloned.Fields["identity"].Children["class"].Schema.GenerationEnum
	require.NotSame(t, &originalEnum[0], &clonedEnum[0])

	cloned.ClassSubclass.Class[0] = "stats"
	clonedEnum[0] = "Wizard"

	assert.Equal(t, []string{"identity", "class"}, original.ClassSubclass.Class)
	assert.Equal(t, []string{"Fighter"}, originalEnum)
}

func TestFantasyV1_GenerationMetadata(t *testing.T) {
	schemas, paths := collectLeaves(fantasyV1Template.Fields)
	require.NotEmpty(t, schemas)

	generatable := 0
	for i, schema := range schemas {
		key := paths[i].String()
		isRules := strings.HasPrefix(key, fieldNameRules+".")
		isPlayerName := key == fieldNameIdentity+".playerName"

		if !schema.IsEditable || isRules || isPlayerName {
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
			assert.Positive(t, *schema.MaxLength, "text leaf %s has a non-positive max length", key)
		default:
			t.Errorf("generatable leaf %s has unexpected type %q", key, schema.Type)
		}
	}
	assert.Positive(t, generatable)

	playerName := fantasyV1Template.Fields[fieldNameIdentity].Children["playerName"].Schema
	require.NotNil(t, playerName)
	assert.True(t, playerName.IsEditable)
	assert.False(t, playerName.Generatable)
	assert.False(t, playerName.GeneratableFor(plan.KindCharacter))
	assert.False(t, playerName.GeneratableFor(plan.KindNPC))
	assert.False(t, playerName.GeneratableFor(plan.KindEnemy))

	require.NoError(t, ValidateGenerationMetadata(fantasyV1Template))
}

func TestFantasyV1_PreferDefault(t *testing.T) {
	schemas, paths := collectLeaves(fantasyV1Template.Fields)
	require.NotEmpty(t, schemas)

	preferred := make([]string, 0, 3)
	for i, schema := range schemas {
		if schema.PreferDefault {
			preferred = append(preferred, paths[i].String())
		}
	}
	assert.ElementsMatch(t, []string{
		fieldNameIdentity + ".level",
		fieldNameIdentity + ".xp",
		fieldNameIdentity + ".proficiencyBonus",
	}, preferred)

	identity := fantasyV1Template.Fields[fieldNameIdentity].Children

	level := identity["level"].Schema
	require.NotNil(t, level)
	assert.InDelta(t, 1.0, level.Default, 1e-9)
	assert.InDelta(t, 1.0, *level.Min, 1e-9)
	assert.InDelta(t, 20.0, *level.Max, 1e-9)

	xp := identity["xp"].Schema
	require.NotNil(t, xp)
	assert.InDelta(t, 0.0, xp.Default, 1e-9)

	proficiency := identity[fieldNameProficiencyBonus].Schema
	require.NotNil(t, proficiency)
	assert.InDelta(t, 2.0, proficiency.Default, 1e-9)

	characterName := identity["characterName"].Schema
	require.NotNil(t, characterName)
	assert.False(t, characterName.PreferDefault)
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
	assert.InDelta(t, float64(60), identity["characterName"].Schema["max_length"], 1e-9)
	assert.NotContains(t, identity["playerName"].Schema, "generatable")
	assert.NotContains(t, identity["playerName"].Schema, "generatable_kinds")
	assert.Equal(t, true, identity["level"].Schema["prefer_default"])
	assert.Equal(t, true, identity["xp"].Schema["prefer_default"])
	assert.Equal(t, true, identity[fieldNameProficiencyBonus].Schema["prefer_default"])
	assert.NotContains(t, identity["characterName"].Schema, "prefer_default")
	assert.InDelta(t, float64(1), identity["level"].Schema["min"], 1e-9)
	assert.InDelta(t, float64(20), identity["level"].Schema["max"], 1e-9)

	rulesTuning := payload.Fields["rules"].Children["modifierBase"].Schema
	assert.NotContains(t, rulesTuning, "generatable")
	assert.NotContains(t, rulesTuning, "min")
}
