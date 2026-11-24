package generation_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"unicode/utf8"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/maputils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssemble(t *testing.T) {
	tmpl := fixtureTemplate()
	dag := fixtureDAG(t, tmpl)

	t.Run("Happy path: the seed, the generated values and the derived leaves are combined", func(t *testing.T) {
		expanded, err := generation.Expand(tmpl, plan.KindCharacter, parseFixture(t, validOutput))
		require.NoError(t, err)

		data, err := generation.Assemble(tmpl, dag, plan.KindCharacter, expanded)
		require.NoError(t, err)

		stats := data["stats"].(map[string]any)
		assert.InDelta(t, 15.0, stats["dexterity"], 1e-9)
		assert.InDelta(t, 12.0, stats["strength"], 1e-9)
		assert.Equal(t, true, stats["hero"])
		assert.InDelta(t, 7.0, stats["luck"], 1e-9, "a leaf the model does not fill keeps its seed value")

		modifiers := data["modifiers"].(map[string]any)
		assert.InDelta(t, 2.0, modifiers["dexterityMod"], 1e-9, "the derived leaf follows the generated dexterity")

		assert.Equal(t, map[string]any{"acUsesDex": 0.0, "acUsesCon": 1.0}, data["config"])
		assert.Equal(t, map[string]any{"base": 8.0}, data["rules"], "rules keep their seed value")
		assert.Equal(t, "Aria", data["identity"].(map[string]any)["name"])
	})

	t.Run("Happy path: a derived leaf reads a non-generated leaf that keeps its default", func(t *testing.T) {
		dependent := dependentFixtureTemplate()
		dependentDAG := fixtureDAG(t, dependent)

		data, err := generation.Assemble(dependent, dependentDAG, plan.KindCharacter, map[string]any{
			"stats": map[string]any{"dexterity": 14.0},
		})
		require.NoError(t, err)

		assert.Equal(t, map[string]any{"dexterity": 14.0}, data["stats"])
		assert.Equal(t, map[string]any{"base": 10.0}, data["rules"], "the non-generated leaf keeps its default")
		assert.Equal(t, map[string]any{"x": 2.0}, data["modifiers"], "the derived leaf sees the defaulted leaf")
	})

	t.Run("Edge: an empty value set keeps the whole seed and still derives the leaves", func(t *testing.T) {
		data, err := generation.Assemble(tmpl, dag, plan.KindCharacter, map[string]any{})
		require.NoError(t, err)
		assert.InDelta(t, 10.0, data["stats"].(map[string]any)["dexterity"], 1e-9)
		assert.InDelta(t, 0.0, data["modifiers"].(map[string]any)["dexterityMod"], 1e-9)
	})

	t.Run("Edge: a nil value set keeps the whole seed", func(t *testing.T) {
		data, err := generation.Assemble(tmpl, dag, plan.KindCharacter, nil)
		require.NoError(t, err)
		assert.InDelta(t, 10.0, data["stats"].(map[string]any)["dexterity"], 1e-9)
	})

	t.Run("Happy path: a record list seeds empty and never reaches the derivation", func(t *testing.T) {
		listed := listedFixtureTemplate()
		listedDAG := fixtureDAG(t, listed)

		data, err := generation.Assemble(listed, listedDAG, plan.KindCharacter, map[string]any{
			"identity": map[string]any{"name": "Aria"},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]any{}, data["inventory"].(map[string]any)["items"])
		assert.Equal(t, "Aria", data["identity"].(map[string]any)["name"])
		assert.InDelta(t, 0.0, data["modifiers"].(map[string]any)["dexterityMod"], 1e-9)
	})

	t.Run("Edge: a value filtered by kind is dropped and keeps its template default", func(t *testing.T) {
		kinded := kindFixtureTemplate()
		kindedDAG := fixtureDAG(t, kinded)

		data, err := generation.Assemble(kinded, kindedDAG, plan.KindNPC, map[string]any{
			"identity": map[string]any{"nickname": "Ana"},
		})
		require.NoError(t, err)
		assert.Empty(t, data["identity"].(map[string]any)["nickname"])
	})

	t.Run("Error: two active exclusive members name the group and both keys", func(t *testing.T) {
		_, err := generation.Assemble(tmpl, dag, plan.KindCharacter, map[string]any{
			"config": map[string]any{"acUsesDex": 1.0, "acUsesCon": 1.0},
		})
		require.Error(t, err)

		var outputErr *generation.OutputError
		require.ErrorAs(t, err, &outputErr)
		require.Len(t, outputErr.Problems, 1)
		assert.Contains(t, outputErr.Problems[0], "armorClassAbility")
		assert.Contains(t, outputErr.Problems[0], "config.acUsesCon")
		assert.Contains(t, outputErr.Problems[0], "config.acUsesDex")
		assert.NotErrorIs(t, err, generation.ErrAssembleFailed)
	})

	t.Run("Error: an exclusive option with a non-numeric value is model-correctable", func(t *testing.T) {
		_, err := generation.Assemble(tmpl, dag, plan.KindCharacter, map[string]any{
			"config": map[string]any{"acUsesDex": "yes"},
		})
		require.Error(t, err)

		var outputErr *generation.OutputError
		require.ErrorAs(t, err, &outputErr)
		require.Len(t, outputErr.Problems, 1)
		assert.Contains(t, outputErr.Problems[0], "armorClassAbility")
		assert.Contains(t, outputErr.Problems[0], "must be one of the listed options")
		assert.NotErrorIs(t, err, generation.ErrAssembleFailed)
	})

	t.Run("Error: a formula that cannot evaluate is an assemble failure, not an output error", func(t *testing.T) {
		broken := brokenFormulaFixtureTemplate()
		brokenDAG := fixtureDAG(t, broken)

		_, err := generation.Assemble(broken, brokenDAG, plan.KindCharacter, map[string]any{
			"identity": map[string]any{"name": "Aria"},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, generation.ErrAssembleFailed)
		require.ErrorIs(t, err, template.ErrFormulaEval)

		var outputErr *generation.OutputError
		assert.NotErrorAs(t, err, &outputErr, "a formula failure is not model-correctable")
	})

	t.Run("Error: a nil template is refused", func(t *testing.T) {
		_, err := generation.Assemble(nil, dag, plan.KindCharacter, map[string]any{})
		require.ErrorIs(t, err, generation.ErrNoTemplate)
	})

	t.Run("Error: a nil DAG is refused", func(t *testing.T) {
		_, err := generation.Assemble(tmpl, nil, plan.KindCharacter, map[string]any{})
		require.ErrorIs(t, err, generation.ErrNoTemplate)
	})
}

func TestAssemble_DefaultTemplatesContract(t *testing.T) {
	variants := []struct {
		name       string
		useMaximum bool
	}{
		{name: "Edge: every default template assembles the minimum output", useMaximum: false},
		{name: "Edge: every default template assembles the maximum output", useMaximum: true},
	}

	for _, variant := range variants {
		for _, tmpl := range template.DefaultTemplates() {
			dag, err := template.BuildDAG(tmpl.ID, tmpl.Fields, engine.NewFormulaEngine())
			require.NoError(t, err)

			for _, kind := range plan.SheetKinds() {
				t.Run(variant.name+": "+tmpl.ID+"/"+string(kind), func(t *testing.T) {
					schema, err := generation.OutputSchema(&tmpl, kind)
					if errors.Is(err, generation.ErrNothingToGenerate) {
						return
					}
					require.NoError(t, err)

					object := syntheticOutput(t, schema, variant.useMaximum)
					reconcileClassSubclass(t, &tmpl, object)
					reconcileSaveProficiencies(t, &tmpl, object)
					raw, err := json.Marshal(object)
					require.NoError(t, err)

					object, err = generation.ParseOutput(raw)
					require.NoError(t, err)
					expanded, err := generation.Expand(&tmpl, kind, object)
					require.NoError(t, err)

					values, err := generation.Assemble(&tmpl, dag, kind, expanded)
					require.NoError(t, err)
					assert.NotEmpty(t, values)
				})
			}
		}
	}
}

func reconcileClassSubclass(t *testing.T, tmpl *template.Template, object map[string]any) {
	t.Helper()
	rule := tmpl.ClassSubclass
	if rule == nil {
		return
	}
	classValue, ok := maputils.Get(object, rule.Class)
	require.True(t, ok, "the synthetic output carries the class")
	class, ok := classValue.(string)
	require.True(t, ok)
	levelValue, ok := maputils.Get(object, rule.Level)
	require.True(t, ok, "the synthetic output carries the level")
	level, ok := levelValue.(float64)
	require.True(t, ok)

	subclass := ""
	if level >= float64(rule.SubclassLevel) {
		names, ok := template.DefaultGameSystems().AllowedNames(tmpl.GameSystemID)
		require.True(t, ok)
		index := slices.Index(names.Classes, class)
		require.GreaterOrEqual(t, index, 0)
		require.Less(t, index, len(names.Subclasses))
		subclass = names.Subclasses[index]
	}
	require.NoError(t, maputils.Set(object, rule.Subclass, subclass))
}

func reconcileSaveProficiencies(t *testing.T, tmpl *template.Template, object map[string]any) {
	t.Helper()
	rule := tmpl.SaveProficiencies
	if rule == nil {
		return
	}
	for index, path := range rule.Paths {
		value := 0.0
		if index < rule.Required {
			value = 1
		}
		require.NoError(t, maputils.Set(object, path, value))
	}
}

func syntheticOutput(t *testing.T, raw json.RawMessage, useMaximum bool) map[string]any {
	t.Helper()
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))
	value := syntheticValue(t, schema, useMaximum)
	object, ok := value.(map[string]any)
	require.True(t, ok, "the root of an output schema is an object")
	return object
}

func syntheticValue(t *testing.T, schema map[string]any, useMaximum bool) any {
	t.Helper()
	switch schema["type"] {
	case "array":
		if !useMaximum {
			return []any{}
		}
		item, ok := schema["items"].(map[string]any)
		require.True(t, ok, "an array schema carries its item schema")
		limit, ok := schema["maxItems"].(float64)
		require.True(t, ok, "a generated array declares its cap")
		items := make([]any, 0, int(limit))
		for range int(limit) {
			items = append(items, syntheticValue(t, item, useMaximum))
		}
		return items
	case "object":
		properties, ok := schema["properties"].(map[string]any)
		require.True(t, ok, "an object schema carries properties")
		out := make(map[string]any, len(properties))
		for name, raw := range properties {
			child, ok := raw.(map[string]any)
			require.True(t, ok)
			out[name] = syntheticValue(t, child, useMaximum)
		}
		return out
	case "number":
		if options, ok := schema["enum"].([]any); ok && len(options) > 0 {
			if useMaximum {
				return options[len(options)-1]
			}
			return options[0]
		}
		lower, hasLower := schema["minimum"].(float64)
		upper, hasUpper := schema["maximum"].(float64)
		switch {
		case useMaximum && hasUpper:
			return upper
		case useMaximum && hasLower:
			return lower
		case !useMaximum && hasLower:
			return lower
		case !useMaximum && hasUpper:
			return upper
		default:
			return 10.0
		}
	case "string":
		options, ok := schema["enum"].([]any)
		if !ok {
			return truncate("x", schema)
		}
		require.NotEmpty(t, options)
		if useMaximum {
			return options[len(options)-1]
		}
		return options[0]
	case "boolean":
		return true
	default:
		require.FailNow(t, "unsupported schema type", "type=%v", schema["type"])
		return nil
	}
}

func truncate(value string, schema map[string]any) string {
	limit, ok := schema["maxLength"].(float64)
	if !ok || int(limit) >= utf8.RuneCountInString(value) {
		return value
	}
	return string([]rune(value)[:int(limit)])
}
