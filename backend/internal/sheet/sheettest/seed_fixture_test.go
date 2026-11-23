package sheettest_test

import (
	"testing"

	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedTemplateData(t *testing.T) {
	t.Run("Happy path: leaves seed by type and lists seed as an empty map", func(t *testing.T) {
		fields := template.FieldGroup{
			"number": {Schema: &template.FieldSchema{Type: "number", Default: 7}},
			"text":   {Schema: &template.FieldSchema{Type: "text"}},
			"flag":   {Schema: &template.FieldSchema{Type: "bool"}},
			"items": {List: &template.ListSchema{
				Label:    "Items",
				Capacity: 1,
				Shape: template.FieldGroup{
					"name": {Schema: &template.FieldSchema{Type: "text"}},
				},
			}},
		}

		seeded := sheettest.SeedTemplateData(fields)

		assert.Equal(t, map[string]any{
			"number": 7.0,
			"text":   "",
			"flag":   false,
			"items":  map[string]any{},
		}, seeded)
	})

	t.Run("Happy path: nested groups keep their shape", func(t *testing.T) {
		fields := template.FieldGroup{
			"combat": {Children: template.FieldGroup{
				"hp": {Children: template.FieldGroup{
					"current": {Schema: &template.FieldSchema{Type: "number", Default: 3}},
				}},
			}},
		}

		seeded := sheettest.SeedTemplateData(fields)

		combat := seeded["combat"].(map[string]any)
		hp := combat["hp"].(map[string]any)
		assert.InDelta(t, 3.0, hp["current"], 1e-9)
	})

	t.Run("Edge: an empty group seeds as an empty map", func(t *testing.T) {
		seeded := sheettest.SeedTemplateData(template.FieldGroup{
			"empty": {Children: template.FieldGroup{}},
		})

		empty, ok := seeded["empty"].(map[string]any)
		require.True(t, ok)
		assert.Empty(t, empty)
	})

	t.Run("Edge: no fields gives an empty map", func(t *testing.T) {
		assert.Empty(t, sheettest.SeedTemplateData(template.FieldGroup{}))
	})
}
