package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dtoLeaf returns the API leaf carrying the widest DependsOn list in the
// catalog (derived.armorClass, 5 paths).
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

// TestTemplateToDTO_Edge_NilSchema covers group nodes, whose API shape is a
// children map with no schema at all.
func TestTemplateToDTO_Edge_NilSchema(t *testing.T) {
	node := FieldNode{Children: FieldGroup{"dexterity": {Schema: &FieldSchema{Type: fieldTypeNumber}}}}

	dto := node.toDTO()

	assert.Nil(t, dto.Schema)
	assert.Contains(t, dto.Children, "dexterity")
}
