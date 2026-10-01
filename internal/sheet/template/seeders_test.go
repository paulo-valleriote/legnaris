package template

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// formulaIdentRe matches the identifiers of an expr-lang formula: field
// engine keys (path segments joined by '_') and function names.
var formulaIdentRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// engineFunctions are the identifiers formulaIdentRe must not treat as
// field references.
var engineFunctions = map[string]bool{"floor": true, "max": true}

// groupKeys returns the field names of a group in a stable form for set
// comparisons (map iteration order is random).
func groupKeys(group FieldGroup) []string {
	keys := make([]string, 0, len(group))
	for key := range group {
		keys = append(keys, key)
	}
	return keys
}

// TestFantasyV1_FormulaReferencesMatchDependsOn pins the two halves of the
// dependency contract BuildDAG cannot check: a formula may only read leaves
// its DependsOn declares — an undeclared read evaluates against a stale
// environment value instead of cascading — and DependsOn must not declare
// leaves the formula never reads, which would cascade work for nothing.
func TestFantasyV1_FormulaReferencesMatchDependsOn(t *testing.T) {
	schemas, paths := collectLeaves(fantasyV1Template.Fields)
	require.NotEmpty(t, schemas)

	validEngineKeys := make(map[string]string, len(paths)) // engine key → path key
	for _, path := range paths {
		validEngineKeys[path.EngineKey()] = path.String()
	}

	for i, schema := range schemas {
		if schema.Formula == "" {
			continue
		}
		key := paths[i].String()
		require.NotEmpty(t, schema.DependsOn, "derived leaf %s declares no dependencies", key)

		declared := make(map[string]bool, len(schema.DependsOn))
		for _, dep := range schema.DependsOn {
			engineKey := Path(dep).EngineKey()
			_, exists := validEngineKeys[engineKey]
			require.True(t, exists,
				"DependsOn of %s references unknown leaf %s", key, Path(dep).String())
			declared[engineKey] = true
		}

		referenced := make(map[string]bool)
		for _, ident := range formulaIdentRe.FindAllString(schema.Formula, -1) {
			if engineFunctions[ident] {
				continue
			}
			referenced[ident] = true
			_, exists := validEngineKeys[ident]
			require.True(t, exists,
				"formula of %s references unknown identifier %s", key, ident)
		}

		for ident := range referenced {
			assert.True(t, declared[ident],
				"formula of %s reads %s but DependsOn does not declare it", key, ident)
		}
		for engineKey := range declared {
			assert.True(t, referenced[engineKey],
				"DependsOn of %s declares %s which the formula never reads", key, engineKey)
		}
	}
}

// TestFantasyV1_Structure pins the sheet layout: every section exists, the
// generated groups carry exactly their expected keys, and defaults/roles
// match the design (scores at 10, flags at 0/1, derived fields computed).
func TestFantasyV1_Structure(t *testing.T) {
	t.Run("Happy path: every section of the sheet exists", func(t *testing.T) {
		assert.ElementsMatch(t, []string{
			"attacks", "combat", "config", "derived", "identity",
			"modifiers", "rules", "saves", "skills", "spells", "stats",
		}, groupKeys(fantasyV1Template.Fields))
	})

	t.Run("Happy path: sections declare their display order, sheet on top and tuning at the bottom", func(t *testing.T) {
		want := []string{
			fieldNameIdentity, fieldNameStats, fieldNameModifiers, "saves",
			fieldNameSkills, fieldNameCombat, "attacks", fieldNameSpells,
			"derived", fieldNameRules, fieldNameConfig,
		}
		assert.Equal(t, want, fantasyV1Template.SectionOrder)
		assert.ElementsMatch(t, groupKeys(fantasyV1Template.Fields),
			fantasyV1Template.SectionOrder,
			"the order must name every section exactly once")
	})

	t.Run("Happy path: identity keeps four texts and three numeric counters", func(t *testing.T) {
		identity := fantasyV1Template.Fields[fieldNameIdentity].Children
		require.Len(t, identity, 7)
		for _, key := range []string{"characterName", "playerName", "ancestry", "origin"} {
			schema := identity[key].Schema
			require.NotNil(t, schema, "missing identity text %s", key)
			assert.Equal(t, fieldTypeText, schema.Type)
			assert.True(t, schema.IsEditable)
			assert.Empty(t, schema.Formula)
		}
		for _, key := range []string{"level", "xp", fieldNameProficiencyBonus} {
			schema := identity[key].Schema
			require.NotNil(t, schema, "missing identity counter %s", key)
			assert.Equal(t, fieldTypeNumber, schema.Type)
			assert.True(t, schema.IsEditable)
		}
		assert.Equal(t, 1.0, identity["level"].Schema.Default)
		assert.Equal(t, 2.0, identity[fieldNameProficiencyBonus].Schema.Default)
	})

	t.Run("Happy path: stats carries the six editable ability scores", func(t *testing.T) {
		stats := fantasyV1Template.Fields[fieldNameStats].Children
		require.Len(t, stats, len(abilities))
		for _, a := range abilities {
			schema := stats[a.key].Schema
			require.NotNil(t, schema, "missing score %s", a.key)
			assert.Equal(t, fieldTypeNumber, schema.Type)
			assert.True(t, schema.IsEditable)
			assert.Equal(t, 10.0, schema.Default)
			assert.Equal(t, a.label, schema.Label)
		}
	})

	t.Run("Happy path: modifiers and saves are derived from their score", func(t *testing.T) {
		modifiers := fantasyV1Template.Fields[fieldNameModifiers].Children
		saves := fantasyV1Template.Fields["saves"].Children
		require.Len(t, modifiers, len(abilities))
		require.Len(t, saves, len(abilities))
		for _, a := range abilities {
			mod := modifiers[modKey(a.key)].Schema
			require.NotNil(t, mod, "missing modifier %s", a.key)
			assert.False(t, mod.IsEditable)
			assert.NotEmpty(t, mod.Formula)
			assert.Equal(t, a.label+" modifier", mod.Label)

			save := saves[a.key].Schema
			require.NotNil(t, save, "missing save %s", a.key)
			assert.False(t, save.IsEditable)
			assert.NotEmpty(t, save.Formula)
			assert.Equal(t, a.label+" save", save.Label)
		}
	})

	t.Run("Happy path: every skill has an editable bonus and a derived total", func(t *testing.T) {
		skillGroup := fantasyV1Template.Fields[fieldNameSkills].Children
		require.Len(t, skillGroup, len(skills))
		for _, s := range skills {
			node := skillGroup[s.key]
			require.NotNil(t, node.Children, "missing skill %s", s.key)
			require.Len(t, node.Children, 2)

			bonus := node.Children[fieldNameBonus].Schema
			require.NotNil(t, bonus)
			assert.True(t, bonus.IsEditable)
			assert.Equal(t, 0.0, bonus.Default)
			assert.Equal(t, s.label+" bonus", bonus.Label)

			total := node.Children[fieldNameTotal].Schema
			require.NotNil(t, total)
			assert.False(t, total.IsEditable)
			assert.NotEmpty(t, total.Formula)
			assert.Equal(t, s.label+" total", total.Label)
		}
	})

	t.Run("Happy path: rules carry the D&D-like calibration defaults", func(t *testing.T) {
		rules := fantasyV1Template.Fields[fieldNameRules].Children
		want := map[string]float64{
			"modifierBase":  10,
			"modifierStep":  2,
			"passiveBase":   10,
			"spellSaveBase": 8,
		}
		require.Len(t, rules, len(want))
		for name, def := range want {
			schema := rules[name].Schema
			require.NotNil(t, schema, "missing rule %s", name)
			assert.True(t, schema.IsEditable)
			assert.Equal(t, fieldTypeNumber, schema.Type)
			assert.Equal(t, def, schema.Default)
		}
	})

	t.Run("Happy path: config carries exactly the 0/1 wiring flags", func(t *testing.T) {
		config := fantasyV1Template.Fields[fieldNameConfig].Children
		want := map[string]float64{"acUsesDex": 1, "acUsesCon": 0}
		for _, a := range abilities {
			want[saveProfKey(a.key)] = 0
			want[spellUsesKey(a.key)] = 0
		}
		for _, s := range skills {
			want[skillProfKey(s.key)] = 0
		}
		require.Len(t, config, len(want))
		for name, def := range want {
			schema := config[name].Schema
			require.NotNil(t, schema, "missing config flag %s", name)
			assert.True(t, schema.IsEditable)
			assert.Equal(t, def, schema.Default)
			assert.Empty(t, schema.Formula)
		}
	})

	t.Run("Happy path: combat numbers are editable with usable defaults", func(t *testing.T) {
		combat := fantasyV1Template.Fields[fieldNameCombat].Children
		want := map[string]float64{
			"baseArmorClass": 10,
			"initiative":     0,
			"speed":          30,
			"hitPointsMax":   10,
			"hitPoints":      10,
			"hitPointsTemp":  0,
		}
		require.Len(t, combat, len(want))
		for name, def := range want {
			schema := combat[name].Schema
			require.NotNil(t, schema, "missing combat field %s", name)
			assert.True(t, schema.IsEditable)
			assert.Equal(t, def, schema.Default)
		}
	})

	t.Run("Happy path: armor class and passive awareness are derived", func(t *testing.T) {
		derived := fantasyV1Template.Fields["derived"].Children
		require.Len(t, derived, 2)
		for _, name := range []string{"armorClass", "passiveAwareness"} {
			schema := derived[name].Schema
			require.NotNil(t, schema, "missing derived field %s", name)
			assert.False(t, schema.IsEditable)
			assert.NotEmpty(t, schema.Formula)
			assert.NotEmpty(t, schema.DependsOn)
		}
	})

	t.Run("Happy path: attacks expose four name/bonus/damage slots", func(t *testing.T) {
		attackGroup := fantasyV1Template.Fields["attacks"].Children
		require.Len(t, attackGroup, 4)
		for i := 1; i <= 4; i++ {
			slot := attackGroup[fmt.Sprintf("attack%d", i)].Children
			require.NotNil(t, slot, "missing attack slot %d", i)
			require.Len(t, slot, 3)
			assert.Equal(t, fieldTypeText, slot["name"].Schema.Type)
			assert.Equal(t, fieldTypeNumber, slot["bonus"].Schema.Type)
			assert.Equal(t, fieldTypeText, slot["damage"].Schema.Type)
		}
	})

	t.Run("Happy path: spells expose the header, nine slot pools and lists", func(t *testing.T) {
		spells := fantasyV1Template.Fields[fieldNameSpells].Children
		for _, name := range []string{"ability", "atWill", "list"} {
			require.Equal(t, fieldTypeText, spells[name].Schema.Type, name)
		}
		for _, name := range []string{"attackBonus", "saveDC"} {
			schema := spells[name].Schema
			require.NotNil(t, schema, name)
			assert.False(t, schema.IsEditable)
			assert.NotEmpty(t, schema.Formula)
		}
		slotCount := 0
		for name, node := range spells {
			if !strings.HasPrefix(name, "slot") {
				continue
			}
			slotCount++
			require.Len(t, node.Children, 3, name)
			assert.True(t, node.Children["total"].Schema.IsEditable)
			assert.True(t, node.Children["expended"].Schema.IsEditable)
			assert.False(t, node.Children["remaining"].Schema.IsEditable)
		}
		assert.Equal(t, 9, slotCount)
	})

	t.Run("Edge: proficiency bonus moved out of the removed others group", func(t *testing.T) {
		assert.NotContains(t, fantasyV1Template.Fields, "others")
		proficiency := fantasyV1Template.Fields[fieldNameIdentity].Children[fieldNameProficiencyBonus].Schema
		require.NotNil(t, proficiency)
		assert.True(t, proficiency.IsEditable)
		assert.Equal(t, 2.0, proficiency.Default)
	})
}

// TestFantasyV1_LeafContracts walks every leaf once and enforces the
// per-leaf invariants the API and the desktop renderer rely on: a humanized
// English label, a supported type, and a clear role (derived with a formula
// and dependencies, or editable without one).
func TestFantasyV1_LeafContracts(t *testing.T) {
	schemas, paths := collectLeaves(fantasyV1Template.Fields)
	require.Equal(t, 131, len(schemas), "the sheet must stay complete")

	for i, schema := range schemas {
		key := paths[i].String()

		assert.NotEmpty(t, strings.TrimSpace(schema.Label),
			"leaf %s has no label", key)
		assert.Contains(t, []string{fieldTypeNumber, fieldTypeText}, schema.Type,
			"leaf %s has an unsupported type", key)

		switch {
		case schema.Formula != "":
			assert.False(t, schema.IsEditable,
				"derived leaf %s must not be editable", key)
			assert.NotEmpty(t, schema.DependsOn,
				"derived leaf %s declares no dependencies", key)
		default:
			assert.True(t, schema.IsEditable,
				"leaf %s is neither derived nor editable", key)
		}

		if schema.Type == fieldTypeText {
			assert.Empty(t, schema.Formula,
				"text leaf %s must not carry a formula (the compile env is numeric)", key)
		}
	}
}
