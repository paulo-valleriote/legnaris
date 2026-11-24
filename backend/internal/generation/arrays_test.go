package generation_test

import (
	"encoding/json"
	"strings"
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixtureObject(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return parseFixture(t, string(raw))
}

func recordOutput(records ...map[string]any) map[string]any {
	items := records
	if items == nil {
		items = []map[string]any{}
	}
	return map[string]any{"inventory": map[string]any{"items": items}}
}

func anyStrings(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func validRecord(name string) map[string]any {
	return map[string]any{
		"name":     name,
		"quantity": 2,
		"weight":   3,
		"equipped": 1,
		"usage":    "swing",
	}
}

func identityOutput(level int, class, subclass, species, background string) map[string]any {
	return map[string]any{"identity": map[string]any{
		"level": level, "class": class, "subclass": subclass,
		"species": species, "background": background,
	}}
}

func schemaProps(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var root map[string]any
	require.NoError(t, json.Unmarshal(raw, &root))
	props, ok := root["properties"].(map[string]any)
	require.True(t, ok)
	return props
}

func TestOutputSchema_RecordLists(t *testing.T) {
	t.Run("Happy path: a capped list becomes an array of non-derived record objects", func(t *testing.T) {
		props := schemaProps(t, mustSchema(t, recordFixtureTemplate(), plan.KindCharacter))

		inventory := props["inventory"].(map[string]any)["properties"].(map[string]any)
		items := inventory["items"].(map[string]any)
		assert.Equal(t, "array", items["type"])
		assert.InDelta(t, 2.0, items["maxItems"], 1e-9)

		item := items["items"].(map[string]any)
		assert.Equal(t, "object", item["type"])
		assert.Equal(t, false, item["additionalProperties"])

		fields := item["properties"].(map[string]any)
		assert.Contains(t, fields, "name")
		assert.Contains(t, fields, "quantity")
		assert.Contains(t, fields, "weight")
		assert.Contains(t, fields, "equipped")
		assert.Contains(t, fields, "usage")
		assert.NotContains(t, fields, "lineWeight", "a derived record field is never generated")
		assert.NotContains(t, fields, "swing", "an exclusive record group is one enum, not flags")
		assert.Equal(t, []any{"swing", "thrown"}, fields["usage"].(map[string]any)["enum"])
		assert.ElementsMatch(t, []any{0.0, 1.0}, fields["equipped"].(map[string]any)["enum"])
	})

	t.Run("Edge: a list without a generation cap is not offered", func(t *testing.T) {
		tmpl := listedFixtureTemplate()
		raw := mustSchema(t, tmpl, plan.KindCharacter)
		assert.NotContains(t, string(raw), `"inventory"`)
		assert.NotContains(t, string(raw), `"maxItems"`)
	})

	t.Run("Edge: fantasy_v1 keeps its object-only schema", func(t *testing.T) {
		raw := string(mustSchema(t, fantasyV1Fixture(t), plan.KindCharacter))
		assert.NotContains(t, raw, `"maxItems"`)
		assert.NotContains(t, raw, `"type":"array"`)
	})
}

func TestOutputSchema_FifthEditionRecordLists(t *testing.T) {
	tmpl := fifthTemplateFixture(t, "fifth_character_v1")
	props := schemaProps(t, mustSchema(t, tmpl, plan.KindCharacter))

	attacks := props["attacks"].(map[string]any)["properties"].(map[string]any)["list"].(map[string]any)
	assert.Equal(t, "array", attacks["type"])
	assert.InDelta(t, 6.0, attacks["maxItems"], 1e-9)
	attackFields := attacks["items"].(map[string]any)["properties"].(map[string]any)
	assert.NotContains(t, attackFields, "toHit", "a derived record field is never generated")
	assert.NotContains(t, attackFields, "damageBonus", "a derived record field is never generated")
	assert.Contains(t, attackFields, "attackAbility", "the record exclusive group becomes one enum")
	assert.Contains(t, attackFields, "proficient")

	inventory := props["inventory"].(map[string]any)["properties"].(map[string]any)["items"].(map[string]any)
	assert.InDelta(t, 20.0, inventory["maxItems"], 1e-9)
	assert.NotContains(t, inventory["items"].(map[string]any)["properties"].(map[string]any), "lineWeight")

	identity := props["identity"].(map[string]any)["properties"].(map[string]any)
	assert.Len(t, identity["class"].(map[string]any)["enum"], 12)
	assert.Len(t, identity["species"].(map[string]any)["enum"], 9)
	assert.Len(t, identity["background"].(map[string]any)["enum"], 4)
	assert.Len(t, identity["subclass"].(map[string]any)["enum"], 13)

	resources := props["resources"].(map[string]any)["properties"].(map[string]any)["list"].(map[string]any)
	assert.InDelta(t, 6.0, resources["maxItems"], 1e-9)
	resourceFields := resources["items"].(map[string]any)["properties"].(map[string]any)
	assert.Contains(t, resourceFields, "max")
	assert.Contains(t, resourceFields, "resourceRecharge")
}

func fifthTemplateFixture(t *testing.T, id string) *template.Template {
	t.Helper()
	templates := template.DefaultTemplates()
	for i := range templates {
		if templates[i].ID == id {
			return &templates[i]
		}
	}
	require.FailNow(t, "missing default template", id)
	return nil
}

func TestSystemPrompt_GenerationEnumsAndLists(t *testing.T) {
	t.Run("Happy path: identity enums and the subclass empty option reach the prompt", func(t *testing.T) {
		prompt, err := generation.SystemPrompt(identityFixtureTemplate(), plan.KindCharacter)
		require.NoError(t, err)
		assert.Contains(t, prompt, "identity.class (Class, text) [at most 60 characters, one of: Barbarian, Bard")
		assert.Contains(t, prompt, "one of: Path of the Berserker, College of Lore")
		assert.Contains(t, prompt, ", or empty]")
	})

	t.Run("Happy path: a capped list is described with its record cap", func(t *testing.T) {
		prompt, err := generation.SystemPrompt(recordFixtureTemplate(), plan.KindCharacter)
		require.NoError(t, err)
		assert.Contains(t, prompt, "inventory.items (Items, at most 2 records)")
	})
}

func TestExpand_RecordArrays(t *testing.T) {
	tmpl := recordFixtureTemplate()
	capacity := tmpl.Fields["inventory"].Children["items"].List.MaxRecords

	t.Run("Happy path: an empty array is an empty record list", func(t *testing.T) {
		expanded, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput()))
		require.NoError(t, err)
		assert.Equal(t, []any{}, expanded["inventory"].(map[string]any)["items"])
	})

	t.Run("Happy path: exactly the cap is accepted and the exclusive group expands", func(t *testing.T) {
		records := make([]map[string]any, 0, capacity)
		for index := range capacity {
			records = append(records, validRecord("Item "+strings.Repeat("x", index)))
		}
		expanded, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput(records...)))
		require.NoError(t, err)

		items := expanded["inventory"].(map[string]any)["items"].([]any)
		require.Len(t, items, capacity)
		first := items[0].(map[string]any)
		assert.Equal(t, "Item ", first["name"])
		assert.InDelta(t, 2.0, first["quantity"], 1e-9)
		assert.InDelta(t, 1.0, first["swing"], 1e-9)
		assert.InDelta(t, 0.0, first["thrown"], 1e-9)
	})

	t.Run("Edge: a derived record field is not accepted from the model", func(t *testing.T) {
		record := validRecord("Sword")
		record["lineWeight"] = 6
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput(record)))
		requireOutputProblem(t, err, "inventory.items[0]: has 1 field the template does not allow")
	})

	t.Run("Error: cap plus one is refused", func(t *testing.T) {
		records := make([]map[string]any, 0, capacity+1)
		for index := range capacity + 1 {
			records = append(records, validRecord("Item "+strings.Repeat("x", index)))
		}
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput(records...)))
		requireOutputProblem(t, err, "inventory.items: has more than 2 records")
	})

	t.Run("Error: a list that is not an array is refused", func(t *testing.T) {
		output := map[string]any{"inventory": map[string]any{"items": map[string]any{}}}
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, output))
		requireOutputProblem(t, err, "inventory.items: must be an array")
	})

	t.Run("Error: an array item that is not an object is refused", func(t *testing.T) {
		output := map[string]any{"inventory": map[string]any{"items": []any{"Sword"}}}
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, output))
		requireOutputProblem(t, err, "inventory.items[0]: must be an object")
	})

	t.Run("Error: a missing record field is refused", func(t *testing.T) {
		record := validRecord("Sword")
		delete(record, "name")
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput(record)))
		requireOutputProblem(t, err, "inventory.items[0].name: is required")
	})

	t.Run("Error: an unknown record field is refused", func(t *testing.T) {
		record := validRecord("Sword")
		record["rarity"] = "common"
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput(record)))
		requireOutputProblem(t, err, "inventory.items[0]: has 1 field the template does not allow")
	})

	t.Run("Error: an unknown exclusive option is refused", func(t *testing.T) {
		record := validRecord("Sword")
		record["usage"] = "stab"
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput(record)))
		requireOutputProblem(t, err, "inventory.items[0].usage: must be one of the listed options")
	})

	t.Run("Error: a record flag outside zero and one is refused", func(t *testing.T) {
		record := validRecord("Sword")
		record["equipped"] = 2
		_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput(record)))
		requireOutputProblem(t, err, "inventory.items[0].equipped: must be 0 or 1")
	})
}

func TestAssemble_RecordArrays(t *testing.T) {
	tmpl := recordFixtureTemplate()
	dag := fixtureDAG(t, tmpl)

	t.Run("Happy path: generated records become list records with ids, positions and derivations", func(t *testing.T) {
		output := recordOutput(validRecord("Sword"), validRecord("Shield"))
		expanded, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, output))
		require.NoError(t, err)

		data, err := generation.Assemble(tmpl, dag, plan.KindCharacter, expanded)
		require.NoError(t, err)

		inventory := data["inventory"].(map[string]any)
		items := inventory["items"].(map[string]any)
		require.Len(t, items, 2)

		first := items["r"+strings.Repeat("0", 32)].(map[string]any)
		assert.Equal(t, "Sword", first["name"])
		assert.InDelta(t, 0.0, first["position"], 1e-9)
		assert.InDelta(t, 6.0, first["lineWeight"], 1e-9)
		assert.InDelta(t, 1.0, first["swing"], 1e-9)

		second := items["r"+strings.Repeat("0", 31)+"1"].(map[string]any)
		assert.Equal(t, "Shield", second["name"])
		assert.InDelta(t, 1.0, second["position"], 1e-9)
		assert.InDelta(t, 12.0, inventory["total"], 1e-9)
	})

	t.Run("Edge: an empty array assembles as an empty list", func(t *testing.T) {
		expanded, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, recordOutput()))
		require.NoError(t, err)

		data, err := generation.Assemble(tmpl, dag, plan.KindCharacter, expanded)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{}, data["inventory"].(map[string]any)["items"])
		assert.InDelta(t, 0.0, data["inventory"].(map[string]any)["total"], 1e-9)
	})
}

func TestOutputSchema_IdentityEnums(t *testing.T) {
	t.Run("Happy path: class, subclass, species and background carry their enums", func(t *testing.T) {
		props := schemaProps(t, mustSchema(t, identityFixtureTemplate(), plan.KindCharacter))
		identity := props["identity"].(map[string]any)["properties"].(map[string]any)

		names, ok := template.DefaultGameSystems().AllowedNames(template.GameSystemFifth)
		require.True(t, ok)

		assert.Equal(t, anyStrings(names.Classes), identity["class"].(map[string]any)["enum"])
		assert.Equal(t, anyStrings(names.Species), identity["species"].(map[string]any)["enum"])
		assert.Equal(t, anyStrings(names.Backgrounds), identity["background"].(map[string]any)["enum"])

		subclasses := identity["subclass"].(map[string]any)["enum"].([]any)
		assert.Len(t, subclasses, len(names.Subclasses)+1)
		assert.Contains(t, subclasses, "")
		for _, name := range names.Subclasses {
			assert.Contains(t, subclasses, name)
		}
	})
}

func TestExpand_IdentityEnums(t *testing.T) {
	tmpl := identityFixtureTemplate()
	names, ok := template.DefaultGameSystems().AllowedNames(template.GameSystemFifth)
	require.True(t, ok)
	require.NotEmpty(t, names.Classes)
	require.NotEmpty(t, names.Subclasses)
	require.NotEmpty(t, names.Species)
	require.NotEmpty(t, names.Backgrounds)

	class := names.Classes[0]
	subclass := names.Subclasses[0]
	species := names.Species[0]
	background := names.Backgrounds[0]

	tests := []struct {
		name   string
		raw    map[string]any
		absent string
	}{
		{name: "Happy path: a level five character chooses the class subclass", raw: identityOutput(5, class, subclass, species, background)},
		{name: "Edge: level two accepts an empty subclass", raw: identityOutput(2, class, "", species, background)},
		{name: "Edge: level three is the first level that needs a subclass", raw: identityOutput(3, class, subclass, species, background)},
		{name: "Error: level two with a subclass is refused", raw: identityOutput(2, class, subclass, species, background), absent: "identity.subclass: must be empty below level 3"},
		{name: "Error: level three without a subclass is refused", raw: identityOutput(3, class, "", species, background), absent: "identity.subclass: must be"},
		{name: "Error: another class's subclass is refused", raw: identityOutput(3, class, names.Subclasses[1], species, background), absent: "identity.subclass: must be"},
		{name: "Error: an unknown class is refused", raw: identityOutput(5, "Artificer", subclass, species, background), absent: "identity.class: must be one of the listed options"},
		{name: "Error: a multiclass value is refused", raw: identityOutput(5, class+" / "+names.Classes[1], subclass, species, background), absent: "identity.class: must be one of the listed options"},
		{name: "Error: an unknown subclass is refused", raw: identityOutput(5, class, "Bladesinger", species, background), absent: "identity.subclass: must be one of the listed options"},
		{name: "Error: an unknown species is refused", raw: identityOutput(5, class, subclass, "Kender", background), absent: "identity.species: must be one of the listed options"},
		{name: "Error: an unknown background is refused", raw: identityOutput(5, class, subclass, species, "Pirate"), absent: "identity.background: must be one of the listed options"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, tc.raw))
			if tc.absent == "" {
				require.NoError(t, err)
				return
			}
			requireOutputProblem(t, err, tc.absent)
		})
	}
}

func requireOutputProblem(t *testing.T, err error, want string) {
	t.Helper()
	require.ErrorIs(t, err, generation.ErrInvalidOutput)
	var outputErr *generation.OutputError
	require.ErrorAs(t, err, &outputErr)
	require.NotEmpty(t, outputErr.Problems)
	assert.Contains(t, strings.Join(outputErr.Problems, "\n"), want)
}
