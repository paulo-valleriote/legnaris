package generation_test

import (
	"testing"

	"legnaris/backend/internal/generation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssemble(t *testing.T) {
	tmpl := fixtureTemplate()
	dag := fixtureDAG(t, tmpl)

	t.Run("Happy path: the seed, the generated values and the derived leaves are combined", func(t *testing.T) {
		expanded, err := generation.Expand(tmpl, parseFixture(t, validOutput))
		require.NoError(t, err)

		data, err := generation.Assemble(tmpl, dag, expanded)
		require.NoError(t, err)

		stats := data["stats"].(map[string]any)
		assert.Equal(t, 15.0, stats["dexterity"])
		assert.Equal(t, 12.0, stats["strength"])
		assert.Equal(t, true, stats["hero"])
		assert.Equal(t, 7.0, stats["luck"], "a leaf the model does not fill keeps its seed value")

		modifiers := data["modifiers"].(map[string]any)
		assert.Equal(t, 2.0, modifiers["dexterityMod"], "the derived leaf follows the generated dexterity")

		assert.Equal(t, map[string]any{"acUsesDex": 0.0, "acUsesCon": 1.0}, data["config"])
		assert.Equal(t, map[string]any{"base": 8.0}, data["rules"], "rules keep their seed value")
		assert.Equal(t, "Aria", data["identity"].(map[string]any)["name"])
	})

	t.Run("Edge: an empty value set keeps the whole seed and still derives the leaves", func(t *testing.T) {
		data, err := generation.Assemble(tmpl, dag, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, 10.0, data["stats"].(map[string]any)["dexterity"])
		assert.Equal(t, 0.0, data["modifiers"].(map[string]any)["dexterityMod"])
	})

	t.Run("Error: two active exclusive members break the rule of the DAG", func(t *testing.T) {
		_, err := generation.Assemble(tmpl, dag, map[string]any{
			"config": map[string]any{"acUsesDex": 1.0, "acUsesCon": 1.0},
		})
		require.Error(t, err)
	})

	t.Run("Error: a nil template is refused", func(t *testing.T) {
		_, err := generation.Assemble(nil, dag, map[string]any{})
		require.ErrorIs(t, err, generation.ErrNoTemplate)
	})

	t.Run("Error: a nil DAG is refused", func(t *testing.T) {
		_, err := generation.Assemble(tmpl, nil, map[string]any{})
		require.ErrorIs(t, err, generation.ErrNoTemplate)
	})
}
