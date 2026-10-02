package template

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dtoLeaf returns the API leaf used as the tamper target: a stable derived
// catalog leaf (derived.armorClass) carrying a non-empty DependsOn list.
func dtoLeaf(dto TemplateDTO) *FieldSchemaDTO {
	return dto.Fields["derived"].Children["armorClass"].Schema
}

// TestTemplateToDTO_DoesNotAliasFields covers the LGN-60 finding that the
// template DTO shared its dependency paths with the registered template:
// rewriting a response payload rewrote the template every later request is
// served from.
func TestTemplateToDTO_DoesNotAliasFields(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(dto *TemplateDTO)
	}{
		{
			name: "Happy path: rewrite a dependency path segment",
			tamper: func(dto *TemplateDTO) {
				dependsOn := dtoLeaf(*dto).DependsOn
				dependsOn[0][0] = "tampered"
			},
		},
		{
			name: "Edge: append a dependency path",
			tamper: func(dto *TemplateDTO) {
				schema := dtoLeaf(*dto)
				schema.DependsOn = append(schema.DependsOn, []string{"tampered"})
			},
		},
		{
			name: "Edge: rewrite the last segment of the widest path",
			tamper: func(dto *TemplateDTO) {
				dependsOn := dtoLeaf(*dto).DependsOn
				last := dependsOn[len(dependsOn)-1]
				last[len(last)-1] = "tampered"
			},
		},
		{
			name: "Edge: rewrite a schema scalar",
			tamper: func(dto *TemplateDTO) {
				schema := dtoLeaf(*dto)
				schema.Formula = "tampered"
				schema.IsEditable = !schema.IsEditable
				schema.OptionLabel = "tampered"
			},
		},
		{
			name: "Edge: add a field to a nested group",
			tamper: func(dto *TemplateDTO) {
				dto.Fields["derived"].Children["tampered"] = FieldNodeDTO{}
			},
		},
		{
			name: "Edge: add a field to the top-level group",
			tamper: func(dto *TemplateDTO) {
				dto.Fields["tampered"] = FieldNodeDTO{}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := cloneTemplate(fantasyV1Template)
			untampered := before.ToDTO()

			dto := fantasyV1Template.ToDTO()
			tc.tamper(&dto)
			require.NotEqual(t, untampered, dto, "tampering must actually write through the DTO")

			assert.Equal(t, before, fantasyV1Template,
				"mutating the DTO changed the registered template")
		})
	}
}

// TestTemplateToDTO_HappyPath_KeepsDependencyContents pins the other half of
// the contract: cloning the paths must not drop, reorder or alter them.
func TestTemplateToDTO_HappyPath_KeepsDependencyContents(t *testing.T) {
	original := fantasyV1Template.Fields["derived"].Children["armorClass"].Schema
	require.NotEmpty(t, original.DependsOn)

	dto := fantasyV1Template.ToDTO()
	leaf := dtoLeaf(dto)

	assert.Equal(t, original.DependsOn, leaf.DependsOn)
	assert.Equal(t, original.Type, leaf.Type)
	assert.Equal(t, original.Label, leaf.Label)
	assert.Equal(t, original.Formula, leaf.Formula)
	assert.Equal(t, original.IsEditable, leaf.IsEditable)
	assert.Equal(t, original.Default, leaf.Default)
	assert.Equal(t, fantasyV1Template.ID, dto.ID)
	assert.Equal(t, fantasyV1Template.Name, dto.Name)
	assert.Equal(t, fantasyV1Template.Default, dto.Default)
}

// TestTemplateToDTO_Edge_NilDependsOn documents that an empty dependency list
// stays absent from the JSON payload instead of showing up as [].
func TestTemplateToDTO_Edge_NilDependsOn(t *testing.T) {
	schema := &FieldSchema{Type: fieldTypeNumber, IsEditable: true}

	dto := schema.toDTO()

	require.NotNil(t, dto)
	assert.Nil(t, dto.DependsOn)
}

// TestFieldSchemaDTO_MarshalJSON_ExclusiveGroup pins the contract metadata key
// (EXC-01/EXC-02): a declared group is serialized verbatim, an empty group is
// omitted so every untagged leaf stays byte-identical to the pre-LGN-105
// payload.
func TestFieldSchemaDTO_MarshalJSON_ExclusiveGroup(t *testing.T) {
	tests := []struct {
		name           string
		exclusiveGroup string
		want           string
	}{
		{
			name:           "Happy path: a declared group is serialized",
			exclusiveGroup: "armorClassAbility",
			want:           `{"type":"number","label":"","is_editable":true,"default":0,"exclusive_group":"armorClassAbility"}`,
		},
		{
			name:           "Edge: an empty group omits the key",
			exclusiveGroup: "",
			want:           `{"type":"number","label":"","is_editable":true,"default":0}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(FieldSchemaDTO{
				Type:           fieldTypeNumber,
				IsEditable:     true,
				ExclusiveGroup: tc.exclusiveGroup,
			})
			require.NoError(t, err)

			assert.JSONEq(t, tc.want, string(raw))
		})
	}
}

// TestFieldSchemaToDTO_CopiesExclusiveGroup pins the copy half of the contract
// (EXC-01): toDTO carries the group into a fresh schema, so a caller that
// mutates the DTO never reaches the registered template.
func TestFieldSchemaToDTO_CopiesExclusiveGroup(t *testing.T) {
	source := &FieldSchema{
		Type:           fieldTypeNumber,
		IsEditable:     true,
		ExclusiveGroup: "armorClassAbility",
	}

	dto := source.toDTO()
	require.NotNil(t, dto)
	assert.Equal(t, "armorClassAbility", dto.ExclusiveGroup)

	dto.ExclusiveGroup = "tampered"
	assert.Equal(t, "armorClassAbility", source.ExclusiveGroup,
		"mutating the DTO changed the source schema")
}

// TestFieldSchemaDTO_MarshalJSON_OptionLabel pins the option-label metadata
// (EXC-23/EXC-24): a declared label is serialized verbatim, while an absent or
// empty label omits the key so every untagged leaf stays byte-identical to the
// pre-LGN-105 payload.
func TestFieldSchemaDTO_MarshalJSON_OptionLabel(t *testing.T) {
	tests := []struct {
		name        string
		optionLabel string
		want        string
	}{
		{
			name:        "Happy path: a declared label is serialized",
			optionLabel: "Dexterity",
			want:        `{"type":"number","label":"","is_editable":true,"default":0,"option_label":"Dexterity"}`,
		},
		{
			name:        "Edge: an empty label omits the key",
			optionLabel: "",
			want:        `{"type":"number","label":"","is_editable":true,"default":0}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(FieldSchemaDTO{
				Type:        fieldTypeNumber,
				IsEditable:  true,
				OptionLabel: tc.optionLabel,
			})
			require.NoError(t, err)

			assert.JSONEq(t, tc.want, string(raw))
		})
	}
}

// TestFieldSchemaToDTO_CopiesOptionLabel pins the copy half of the option-label
// contract (EXC-23): toDTO carries the label into a fresh schema, so a caller
// that mutates the DTO never reaches the registered template.
func TestFieldSchemaToDTO_CopiesOptionLabel(t *testing.T) {
	source := &FieldSchema{
		Type:        fieldTypeNumber,
		IsEditable:  true,
		OptionLabel: "Charisma",
	}

	dto := source.toDTO()
	require.NotNil(t, dto)
	assert.Equal(t, "Charisma", dto.OptionLabel)

	dto.OptionLabel = "tampered"
	assert.Equal(t, "Charisma", source.OptionLabel,
		"mutating the DTO changed the source schema")
}

// TestTemplateToDTO_Edge_NilSchema covers group nodes, whose API shape is a
// children map with no schema at all.
func TestTemplateToDTO_Edge_NilSchema(t *testing.T) {
	node := FieldNode{Children: FieldGroup{"dexterity": {Schema: &FieldSchema{Type: fieldTypeNumber}}}}

	dto := node.toDTO()

	assert.Nil(t, dto.Schema)
	assert.Contains(t, dto.Children, "dexterity")
}

// topLevelFieldKeys reads the `fields` object of a marshaled TemplateDTO and
// returns its keys in emission order — json.Unmarshal would flatten the object
// into a map and lose exactly what this contract is about.
func topLevelFieldKeys(t *testing.T, raw []byte) []string {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(raw))
	_, err := decoder.Token()
	require.NoError(t, err, "payload must open as an object")

	for decoder.More() {
		token, err := decoder.Token()
		require.NoError(t, err)
		key, ok := token.(string)
		require.True(t, ok, "object keys must be strings")
		if key != "fields" {
			var skip json.RawMessage
			require.NoError(t, decoder.Decode(&skip))
			continue
		}

		_, err = decoder.Token()
		require.NoError(t, err, "fields must open as an object")
		keys := []string{}
		for decoder.More() {
			token, err := decoder.Token()
			require.NoError(t, err)
			key, ok := token.(string)
			require.True(t, ok, "field keys must be strings")
			keys = append(keys, key)
			var skip json.RawMessage
			require.NoError(t, decoder.Decode(&skip))
		}
		return keys
	}

	t.Fatal("payload has no fields object")
	return nil
}

// TestTemplateDTO_MarshalJSON_SectionOrder pins the display-order contract:
// the top-level keys of `fields` must arrive in the template's declared
// SectionOrder, because the sheet renders sections exactly as they arrive and
// encoding/json would otherwise sort them alphabetically (identity would land
// in the middle and config in the middle too, instead of sheet-first,
// machinery-last).
func TestTemplateDTO_MarshalJSON_SectionOrder(t *testing.T) {
	raw, err := json.Marshal(fantasyV1Template.ToDTO())
	require.NoError(t, err)

	assert.Equal(t, fantasyV1Template.SectionOrder, topLevelFieldKeys(t, raw))
}

// TestTemplateDTO_MarshalJSON_Edge_OrderFallbacks documents the safety nets:
// a DTO with no order keeps the alphabetical output encoding/json always
// produced, ghost entries in the order are skipped, duplicates are emitted
// once, and unlisted keys still show up after the declared ones.
func TestTemplateDTO_MarshalJSON_Edge_OrderFallbacks(t *testing.T) {
	tests := []struct {
		name  string
		order []string
		want  []string
	}{
		{
			name:  "Edge: no order at all falls back to alphabetical",
			order: nil,
			want:  []string{"config", "identity", "stats"},
		},
		{
			name:  "Edge: ghost and duplicate entries in the order are skipped",
			order: []string{"stats", "ghost", "stats", "config"},
			want:  []string{"stats", "config", "identity"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := TemplateDTO{
				ID:           "test",
				Name:         "Test",
				Default:      true,
				SectionOrder: tc.order,
				Fields: FieldGroupDTO{
					"stats":    {},
					"identity": {},
					"config":   {},
				},
			}

			raw, err := json.Marshal(dto)
			require.NoError(t, err)

			assert.Equal(t, tc.want, topLevelFieldKeys(t, raw))
		})
	}
}
