package template

import (
	"fmt"
	"strings"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/shared/maputils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fifthRecordListCase struct {
	name     string
	section  string
	listKey  string
	listPath string
	capacity int
	columns  []string
	shape    []string
	derived  []string
	flags    []string
}

func fifthCharacterRecordLists() []fifthRecordListCase {
	return []fifthRecordListCase{
		{
			name:     "attacks",
			section:  fifthAttacksSection,
			listKey:  fifthRecordListKey,
			listPath: "attacks.list",
			capacity: fifthAttacksCapacity,
			columns:  []string{"name", "toHit", "damageBonus", "damage", "notes"},
			shape: []string{
				"name", "abilityStrength", "abilityDexterity", "abilityConstitution",
				"abilityIntelligence", "abilityWisdom", "abilityCharisma", "proficient",
				"misc", "damage", "notes", "toHit", "damageBonus",
			},
			derived: []string{"toHit", "damageBonus"},
			flags:   []string{"proficient"},
		},
		{
			name:     "inventory",
			section:  fifthInventorySection,
			listKey:  fifthInventoryItemsKey,
			listPath: "inventory.items",
			capacity: fifthInventoryCapacity,
			columns:  []string{"name", "quantity", "weight", "totalWeight", "attuned", "notes"},
			shape:    []string{"name", "quantity", "weight", "attuned", "notes", "totalWeight"},
			derived:  []string{"totalWeight"},
			flags:    []string{"attuned"},
		},
		{
			name:     "features",
			section:  fifthFeaturesSection,
			listKey:  fifthRecordListKey,
			listPath: "features.list",
			capacity: fifthFeaturesCapacity,
			columns:  []string{"name", "source", "text"},
			shape:    []string{"name", "source", "text"},
		},
		{
			name:     "spells",
			section:  fifthSpellsSection,
			listKey:  fifthRecordListKey,
			listPath: "spells.list",
			capacity: fifthSpellsCapacity,
			columns:  []string{"name", "level", "prepared", "concentration", "ritual", "notes"},
			shape:    []string{"name", "level", "prepared", "concentration", "ritual", "notes"},
			flags:    []string{"prepared", "concentration", "ritual"},
		},
		{
			name:     "resources",
			section:  fifthResourcesSection,
			listKey:  fifthRecordListKey,
			listPath: "resources.list",
			capacity: fifthResourcesCapacity,
			columns:  []string{"name", "max", "used", "rechargeShort", "rechargeLong", "rechargeNone"},
			shape: []string{
				"name", "max", "used", "rechargeShort", "rechargeLong", "rechargeNone",
			},
		},
	}
}

func fifthCharacterRecordList(t *testing.T, tc fifthRecordListCase) ListSchema {
	t.Helper()
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	section, ok := tmpl.Fields[tc.section]
	require.True(t, ok, "missing section %s", tc.section)
	node, ok := section.Children[tc.listKey]
	require.True(t, ok, "missing list %s", tc.listPath)
	require.NotNil(t, node.List, "missing list schema %s", tc.listPath)
	return *node.List
}

func fifthRecordValue(t *testing.T, data map[string]any, listPath, recordID, field string) float64 {
	t.Helper()
	value, ok := maputils.Get(data, strings.Split(listPath+"."+recordID+"."+field, "."))
	require.True(t, ok, "missing record field %s.%s.%s", listPath, recordID, field)
	number, ok := maputils.ToFloat64(value)
	require.True(t, ok, "record field %s.%s.%s is not numeric", listPath, recordID, field)
	return number
}

func TestFifthCharacter_RecordSections(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)

	t.Run("Happy path: the record sections close the sheet reading order", func(t *testing.T) {
		want := []string{
			fifthIdentitySection, fifthAbilitiesSection, fifthSavesSection,
			fifthSkillsSection, fifthExhaustionSection, fifthPassiveSection, fifthArmorSection,
			fifthAttacksSection, fifthInventorySection, fifthFeaturesSection,
			fifthSpellsSection, fifthResourcesSection,
		}
		assert.Equal(t, want, tmpl.SectionOrder)
		assert.ElementsMatch(t, groupKeys(tmpl.Fields), tmpl.SectionOrder)
	})

	for _, tc := range fifthCharacterRecordLists() {
		t.Run("Happy path: "+tc.listPath+" declares its shape, columns and capacity", func(t *testing.T) {
			list := fifthCharacterRecordList(t, tc)
			assert.Equal(t, tc.capacity, list.Capacity)
			assert.Equal(t, tc.columns, list.Columns)
			assert.ElementsMatch(t, tc.shape, groupKeys(list.Shape))
			assert.GreaterOrEqual(t, list.Capacity, MinListCapacity)
			assert.LessOrEqual(t, list.Capacity, MaxListCapacity)

			for _, field := range tc.flags {
				schema := list.Shape[field].Schema
				require.NotNil(t, schema, field)
				assert.True(t, schema.IsEditable, "%s must be editable", field)
				assert.True(t, schema.Flag, "%s must be a numeric flag", field)
				assert.InDelta(t, 0.0, schema.Default, 1e-9, field)
			}

			for _, field := range tc.derived {
				schema := list.Shape[field].Schema
				require.NotNil(t, schema, field)
				assert.False(t, schema.IsEditable, "%s must be derived", field)
				assert.NotEmpty(t, schema.Formula, "%s must carry a formula", field)
				assert.NotEmpty(t, schema.DependsOn, "%s must declare its dependencies", field)
			}
		})
	}

	t.Run("Happy path: inventory adds the two aggregates next to the items list", func(t *testing.T) {
		section := tmpl.Fields[fifthInventorySection].Children

		require.NotNil(t, section[fifthInventoryItemsKey].List)

		totalWeight := section[fifthInventoryTotalWeightKey].Schema
		require.NotNil(t, totalWeight)
		assert.False(t, totalWeight.IsEditable)
		assert.Empty(t, totalWeight.Formula)
		require.NotNil(t, totalWeight.Aggregate)
		assert.Equal(t, AggregateSchema{
			List:  "inventory.items",
			Field: "totalWeight",
			Op:    AggregateSum,
		}, *totalWeight.Aggregate)

		attuned := section[fifthInventoryAttunedKey].Schema
		require.NotNil(t, attuned)
		assert.False(t, attuned.IsEditable)
		assert.Empty(t, attuned.Formula)
		require.NotNil(t, attuned.Aggregate)
		assert.Equal(t, AggregateSchema{
			List:  "inventory.items",
			Field: "attuned",
			Op:    AggregateCount,
		}, *attuned.Aggregate)
	})

	t.Run("Happy path: every attack ability is one exclusive choice defaulting to strength", func(t *testing.T) {
		list := fifthCharacterRecordList(t, fifthCharacterRecordLists()[0])

		chosen := 0
		for _, ability := range abilities {
			schema := list.Shape[fifthAttackAbilityKey(ability.key)].Schema
			require.NotNil(t, schema, ability.key)
			assert.True(t, schema.IsEditable)
			assert.Equal(t, fifthAttackAbilityGroup, schema.ExclusiveGroup)
			assert.Equal(t, ability.label, schema.OptionLabel)
			if schema.Default == 1 {
				chosen++
				assert.Equal(t, abilityStrength, ability.key)
			}
		}
		assert.Equal(t, 1, chosen)
	})

	t.Run("Happy path: resources recharge by one exclusive choice defaulting to none", func(t *testing.T) {
		list := fifthCharacterRecordList(t, fifthCharacterRecordLists()[4])

		defaults := map[string]float64{
			fifthResourceRechargeShortKey: 0,
			fifthResourceRechargeLongKey:  0,
			fifthResourceRechargeNoneKey:  1,
		}
		for key, want := range defaults {
			schema := list.Shape[key].Schema
			require.NotNil(t, schema, key)
			assert.True(t, schema.IsEditable)
			assert.Equal(t, fifthResourceRechargeGroup, schema.ExclusiveGroup)
			assert.InDelta(t, want, schema.Default, 1e-9, key)
		}
	})

	t.Run("Happy path: only the attack list carries a d20 test with exhaustion", func(t *testing.T) {
		attacks := fifthCharacterRecordList(t, fifthCharacterRecordLists()[0])
		assert.Equal(t, "1d20 + @.toHit + exhaustion.penalty", attacks.Dice)
		assert.False(t, attacks.NotD20)
		assert.LessOrEqual(t, len(attacks.Dice), 64)
		assert.Contains(t, attacks.Dice, "exhaustion.penalty")

		for _, tc := range fifthCharacterRecordLists()[1:] {
			assert.Empty(t, fifthCharacterRecordList(t, tc).Dice, tc.listPath)
		}
	})
}

func TestFifthCharacter_RecordListsBuildAtBoot(t *testing.T) {
	t.Run("Happy path: the registry freezes the five record lists", func(t *testing.T) {
		registry, err := BuildRegistry(engine.NewFormulaEngine())
		require.NoError(t, err)

		tmpl, ok := registry.FetchByID(fifthCharacterID)
		require.True(t, ok)
		require.NoError(t, ValidateDiceDefaults(*tmpl))

		dag, ok := registry.DAG(fifthCharacterID)
		require.True(t, ok)
		assert.Subset(t, dag.ListPaths(), []string{
			"attacks.list", "inventory.items", "features.list", "spells.list", "resources.list",
		})
	})
}

func TestFifthCharacter_AttackToHitAndDamage(t *testing.T) {
	tests := []struct {
		name      string
		ability   string
		score     float64
		level     float64
		prof      float64
		misc      float64
		wantToHit float64
		wantBonus float64
	}{
		{
			name:    "Happy path: strength adds its modifier to hit and damage",
			ability: abilityStrength, score: 16, level: 1, prof: 0, misc: 0,
			wantToHit: 3, wantBonus: 3,
		},
		{
			name:    "Happy path: dexterity adds its modifier to hit and damage",
			ability: abilityDexterity, score: 18, level: 1, prof: 0, misc: 0,
			wantToHit: 4, wantBonus: 4,
		},
		{
			name:    "Happy path: constitution adds its modifier to hit and damage",
			ability: abilityConstitution, score: 8, level: 1, prof: 0, misc: 0,
			wantToHit: -1, wantBonus: -1,
		},
		{
			name:    "Happy path: intelligence adds its modifier to hit and damage",
			ability: abilityIntelligence, score: 12, level: 1, prof: 0, misc: 0,
			wantToHit: 1, wantBonus: 1,
		},
		{
			name:    "Happy path: wisdom adds its modifier to hit and damage",
			ability: abilityWisdom, score: 20, level: 1, prof: 0, misc: 0,
			wantToHit: 5, wantBonus: 5,
		},
		{
			name:    "Happy path: charisma adds its modifier to hit and damage",
			ability: abilityCharisma, score: 13, level: 1, prof: 0, misc: 0,
			wantToHit: 1, wantBonus: 1,
		},
		{
			name:    "Happy path: proficiency adds the level-5 bonus to hit only",
			ability: abilityDexterity, score: 14, level: 5, prof: 1, misc: 0,
			wantToHit: 5, wantBonus: 2,
		},
		{
			name:    "Happy path: misc stacks on the proficient to hit",
			ability: abilityDexterity, score: 14, level: 5, prof: 1, misc: 1,
			wantToHit: 6, wantBonus: 2,
		},
		{
			name:    "Edge: a negative misc and a negative modifier still sum",
			ability: abilityStrength, score: 8, level: 1, prof: 0, misc: -1,
			wantToHit: -2, wantBonus: -1,
		},
	}

	tmpl := fifthTemplateByID(t, fifthCharacterID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, tmpl, map[string]any{
				"identity.level":                          tc.level,
				"abilities." + tc.ability + ".score":      tc.score,
				"attacks.list." + validRecordID + ".name": "Attack",
				"attacks.list." + validRecordID + "." + fifthAttackAbilityKey(tc.ability): 1.0,
				"attacks.list." + validRecordID + ".proficient":                           tc.prof,
				"attacks.list." + validRecordID + ".misc":                                 tc.misc,
			})
			assert.InDelta(t, tc.wantToHit, fifthRecordValue(t, data, "attacks.list", validRecordID, "toHit"), 1e-9)
			assert.InDelta(t, tc.wantBonus, fifthRecordValue(t, data, "attacks.list", validRecordID, "damageBonus"), 1e-9)
		})
	}

	t.Run("Edge: a new attack counts on strength and leaves exhaustion out of the total", func(t *testing.T) {
		data := fifthSheet(t, tmpl, map[string]any{
			"exhaustion.level":                        6.0,
			"attacks.list." + validRecordID + ".name": "Unarmed strike",
		})
		assert.InDelta(t, 0.0, fifthRecordValue(t, data, "attacks.list", validRecordID, "toHit"), 1e-9)
		assert.InDelta(t, 0.0, fifthRecordValue(t, data, "attacks.list", validRecordID, "damageBonus"), 1e-9)
	})

	t.Run("Happy path: switching the ability clears the previous one in that record", func(t *testing.T) {
		data := fifthSheet(t, tmpl, map[string]any{
			"abilities.dexterity.score":                           float64(18),
			"attacks.list." + validRecordID + ".abilityDexterity": 1.0,
		})
		assert.InDelta(t, 0.0, fifthRecordValue(t, data, "attacks.list", validRecordID, "abilityStrength"), 1e-9)
		assert.InDelta(t, 1.0, fifthRecordValue(t, data, "attacks.list", validRecordID, "abilityDexterity"), 1e-9)
		assert.InDelta(t, 4.0, fifthRecordValue(t, data, "attacks.list", validRecordID, "toHit"), 1e-9)
	})

	t.Run("Error: one record cannot choose two abilities", func(t *testing.T) {
		seeded := fifthSheet(t, tmpl, nil)
		_, err := fifthDAG(t, tmpl).CalculateSave(seeded, map[string]any{
			"attacks.list." + validRecordID + ".abilityStrength":  1.0,
			"attacks.list." + validRecordID + ".abilityDexterity": 1.0,
		})
		var target *ExclusiveGroupConflictError
		require.ErrorAs(t, err, &target)
		assert.Equal(t, fifthAttackAbilityGroup, target.Group)
	})

	t.Run("Happy path: two attacks may choose different abilities", func(t *testing.T) {
		data := fifthSheet(t, tmpl, map[string]any{
			"abilities.strength.score":                             float64(16),
			"abilities.dexterity.score":                            float64(18),
			"attacks.list." + validRecordID + ".abilityStrength":   1.0,
			"attacks.list." + secondRecordID + ".abilityDexterity": 1.0,
		})
		assert.InDelta(t, 3.0, fifthRecordValue(t, data, "attacks.list", validRecordID, "toHit"), 1e-9)
		assert.InDelta(t, 4.0, fifthRecordValue(t, data, "attacks.list", secondRecordID, "toHit"), 1e-9)
	})
}

func TestFifthCharacter_InventoryAggregates(t *testing.T) {
	type line struct {
		quantity float64
		weight   float64
		attuned  float64
	}
	tests := []struct {
		name        string
		lines       []line
		wantWeight  float64
		wantAttuned float64
	}{
		{
			name: "Edge: an empty list aggregates to zero",
		},
		{
			name:       "Happy path: quantity times weight sums into the sheet total",
			lines:      []line{{quantity: 3, weight: 2, attuned: 1}},
			wantWeight: 6, wantAttuned: 1,
		},
		{
			name:       "Edge: a zero quantity line adds no weight and no attunement",
			lines:      []line{{quantity: 0, weight: 5, attuned: 0}},
			wantWeight: 0, wantAttuned: 0,
		},
		{
			name: "Happy path: several lines sum and count only the attuned ones",
			lines: []line{
				{quantity: 2, weight: 1.5, attuned: 1},
				{quantity: 1, weight: 4, attuned: 0},
				{quantity: 4, weight: 0.25, attuned: 1},
			},
			wantWeight: 8, wantAttuned: 2,
		},
		{
			name:       "Edge: an unweighed item still counts when attuned",
			lines:      []line{{quantity: 0, weight: 0, attuned: 1}},
			wantWeight: 0, wantAttuned: 1,
		},
	}

	tmpl := fifthTemplateByID(t, fifthCharacterID)
	ids := []string{validRecordID, secondRecordID, thirdRecordID}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updates := map[string]any{}
			for i, item := range tc.lines {
				id := "inventory.items." + ids[i]
				updates[id+".name"] = "Item"
				updates[id+".quantity"] = item.quantity
				updates[id+".weight"] = item.weight
				updates[id+".attuned"] = item.attuned
			}

			data := fifthSheet(t, tmpl, updates)
			assert.InDelta(t, tc.wantWeight, fifthValue(t, data, "inventory.totalWeight"), 1e-9)
			assert.InDelta(t, tc.wantAttuned, fifthValue(t, data, "inventory.attuned"), 1e-9)

			for i, item := range tc.lines {
				wantLine := item.quantity * item.weight
				assert.InDelta(t, wantLine,
					fifthRecordValue(t, data, "inventory.items", ids[i], "totalWeight"), 1e-9)
			}
		})
	}
}

func TestFifthCharacter_RecordListCapacity(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	list := fifthCharacterRecordList(t, fifthCharacterRecordLists()[0])

	t.Run("Happy path: the attack list fills to its capacity", func(t *testing.T) {
		updates := map[string]any{}
		for i := range list.Capacity {
			updates[fmt.Sprintf("attacks.list.r%032x.name", i)] = "Attack"
		}
		data := fifthSheet(t, tmpl, updates)
		_, ok := maputils.Get(data, []string{"attacks", "list"})
		assert.True(t, ok)
	})

	t.Run("Error: one record over capacity is refused", func(t *testing.T) {
		seeded := fifthSheet(t, tmpl, nil)
		updates := map[string]any{}
		for i := range list.Capacity + 1 {
			updates[fmt.Sprintf("attacks.list.r%032x.name", i)] = "Attack"
		}

		_, err := fifthDAG(t, tmpl).CalculateSave(seeded, updates)
		var target *ListCapacityExceededError
		require.ErrorAs(t, err, &target)
		assert.Equal(t, "attacks.list", target.Path)
		assert.Equal(t, list.Capacity+1, target.Count)
		assert.Equal(t, list.Capacity, target.Capacity)
	})
}

func TestFifthCharacter_ResourcesRechargePerRecord(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)

	t.Run("Happy path: a resource starts with no recharge and can switch to short", func(t *testing.T) {
		data := fifthSheet(t, tmpl, map[string]any{
			"resources.list." + validRecordID + ".name": "Second Wind",
			"resources.list." + validRecordID + ".max":  2.0,
			"resources.list." + validRecordID + ".used": 1.0,
		})
		assert.InDelta(t, 1.0, fifthRecordValue(t, data, "resources.list", validRecordID, "rechargeNone"), 1e-9)

		data = fifthSheet(t, tmpl, map[string]any{
			"resources.list." + validRecordID + ".name":          "Second Wind",
			"resources.list." + validRecordID + ".max":           2.0,
			"resources.list." + validRecordID + ".used":          1.0,
			"resources.list." + validRecordID + ".rechargeShort": 1.0,
		})
		assert.InDelta(t, 1.0, fifthRecordValue(t, data, "resources.list", validRecordID, "rechargeShort"), 1e-9)
		assert.InDelta(t, 0.0, fifthRecordValue(t, data, "resources.list", validRecordID, "rechargeNone"), 1e-9)
		assert.InDelta(t, 0.0, fifthRecordValue(t, data, "resources.list", validRecordID, "rechargeLong"), 1e-9)
	})

	t.Run("Happy path: two resources may recharge on different rests", func(t *testing.T) {
		data := fifthSheet(t, tmpl, map[string]any{
			"resources.list." + validRecordID + ".name":          "Second Wind",
			"resources.list." + validRecordID + ".rechargeShort": 1.0,
			"resources.list." + secondRecordID + ".name":         "Channel Divinity",
			"resources.list." + secondRecordID + ".rechargeLong": 1.0,
		})
		assert.InDelta(t, 1.0, fifthRecordValue(t, data, "resources.list", validRecordID, "rechargeShort"), 1e-9)
		assert.InDelta(t, 1.0, fifthRecordValue(t, data, "resources.list", secondRecordID, "rechargeLong"), 1e-9)
	})

	t.Run("Error: one resource cannot recharge on two rests", func(t *testing.T) {
		seeded := fifthSheet(t, tmpl, nil)
		_, err := fifthDAG(t, tmpl).CalculateSave(seeded, map[string]any{
			"resources.list." + validRecordID + ".rechargeShort": 1.0,
			"resources.list." + validRecordID + ".rechargeLong":  1.0,
		})
		var target *ExclusiveGroupConflictError
		require.ErrorAs(t, err, &target)
		assert.Equal(t, fifthResourceRechargeGroup, target.Group)
	})
}

func TestFifthCharacter_RecordShapesAreFlatLeaves(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	_, staticPaths := collectLeaves(tmpl.Fields)
	staticKeys := make(map[string]bool, len(staticPaths))
	for _, path := range staticPaths {
		staticKeys[path.EngineKey()] = true
	}

	for _, tc := range fifthCharacterRecordLists() {
		t.Run("Happy path: "+tc.listPath+" declares only labelled leaves", func(t *testing.T) {
			list := fifthCharacterRecordList(t, tc)
			groups := make(map[string][]string)

			for name, node := range list.Shape {
				assert.True(t, node.IsLeaf(), "%s.%s must be a leaf", tc.listPath, name)
				schema := node.Schema
				require.NotNil(t, schema, name)
				assert.NotEmpty(t, strings.TrimSpace(schema.Label), name)
				assert.Contains(t, []string{fieldTypeNumber, fieldTypeText}, schema.Type, name)

				switch {
				case schema.Formula != "":
					assert.False(t, schema.IsEditable, "%s must be derived", name)
					assert.NotEmpty(t, schema.DependsOn, "%s must declare dependencies", name)
					for _, dep := range schema.DependsOn {
						if len(dep) == 1 {
							_, ok := list.Shape[dep[0]]
							assert.True(t, ok, "%s reads unknown record field %s", name, dep[0])
							continue
						}
						assert.True(t, staticKeys[Path(dep).EngineKey()],
							"%s reads unknown sheet field %s", name, Path(dep).String())
					}
				default:
					assert.True(t, schema.IsEditable, "%s is neither derived nor editable", name)
				}

				if schema.Type == fieldTypeText {
					assert.Empty(t, schema.Formula, "text leaf %s must not carry a formula", name)
				}
				if schema.ExclusiveGroup != "" {
					groups[schema.ExclusiveGroup] = append(groups[schema.ExclusiveGroup], name)
				}
			}

			for group, members := range groups {
				require.GreaterOrEqual(t, len(members), 2, "%s group %s", tc.listPath, group)
				defaults := 0
				for _, member := range members {
					if list.Shape[member].Schema.Default == 1 {
						defaults++
					}
				}
				assert.Equal(t, 1, defaults, "%s group %s needs one default", tc.listPath, group)
			}
		})
	}
}
