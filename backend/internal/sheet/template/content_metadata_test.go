package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func saveProficiencyPaths(keys ...string) [][]string {
	paths := make([][]string, len(keys))
	for i, key := range keys {
		paths[i] = []string{"saves", key}
	}
	return paths
}

func saveProficiencyTemplate(paths [][]string, required int) Template {
	saves := FieldGroup{}
	for _, path := range paths {
		saves[path[len(path)-1]] = genLeaf(FieldSchema{Generatable: true, Flag: true, Min: f64(0), Max: f64(1)})
	}
	return Template{
		ID:                "gen_test",
		State:             StateActive,
		SaveProficiencies: &SaveProficiencyRule{Paths: paths, Required: required},
		Fields:            FieldGroup{"saves": {Children: saves}},
	}
}

func TestValidateGenerationMetadata_SaveProficiencies(t *testing.T) {
	tests := []struct {
		name     string
		paths    [][]string
		required int
		wantErr  bool
	}{
		{
			name:     "Happy path: exactly two of six flagged save leaves are required",
			paths:    saveProficiencyPaths("strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"),
			required: 2,
		},
		{
			name:     "Edge: requiring every path is valid",
			paths:    saveProficiencyPaths("strength", "dexterity"),
			required: 2,
		},
		{
			name:     "Error: a requirement of zero saves is rejected",
			paths:    saveProficiencyPaths("strength", "dexterity"),
			required: 0,
			wantErr:  true,
		},
		{
			name:     "Error: a requirement above the path count is rejected",
			paths:    saveProficiencyPaths("strength"),
			required: 2,
			wantErr:  true,
		},
		{
			name:     "Error: an empty path list is rejected",
			paths:    nil,
			required: 2,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateGenerationMetadata(saveProficiencyTemplate(tc.paths, tc.required))
			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrSaveProficiencyInvalid)
		})
	}

	t.Run("Error: a path must resolve to a flagged leaf", func(t *testing.T) {
		tmpl := saveProficiencyTemplate(saveProficiencyPaths("strength"), 1)
		tmpl.Fields["saves"].Children["strength"] = genLeaf(FieldSchema{Generatable: true})
		require.ErrorIs(t, ValidateGenerationMetadata(tmpl), ErrSaveProficiencyInvalid)
	})

	t.Run("Error: a path must resolve to a generatable leaf", func(t *testing.T) {
		tmpl := saveProficiencyTemplate(saveProficiencyPaths("strength"), 1)
		tmpl.Fields["saves"].Children["strength"] = genLeaf(FieldSchema{Flag: true})
		require.ErrorIs(t, ValidateGenerationMetadata(tmpl), ErrSaveProficiencyInvalid)
	})

	t.Run("Error: a path must resolve", func(t *testing.T) {
		tmpl := saveProficiencyTemplate(saveProficiencyPaths("strength"), 1)
		tmpl.SaveProficiencies.Paths = saveProficiencyPaths("wisdom")
		require.ErrorIs(t, ValidateGenerationMetadata(tmpl), ErrSaveProficiencyInvalid)
	})
}

func TestFifthTemplates_ContentMetadata(t *testing.T) {
	character := fifthTemplateByID(t, fifthCharacterID)
	statBlock := fifthTemplateByID(t, fifthStatBlockID)

	t.Run("Happy path: the character requires exactly two save proficiencies", func(t *testing.T) {
		rule := character.SaveProficiencies
		require.NotNil(t, rule)
		assert.Equal(t, 2, rule.Required)
		assert.Equal(t, [][]string{
			{"saves", "strength", "prof"},
			{"saves", "dexterity", "prof"},
			{"saves", "constitution", "prof"},
			{"saves", "intelligence", "prof"},
			{"saves", "wisdom", "prof"},
			{"saves", "charisma", "prof"},
		}, rule.Paths)
		for _, path := range rule.Paths {
			schema := generatedLeafAt(character.Fields, path)
			require.NotNil(t, schema)
			assert.True(t, schema.Generatable)
			assert.True(t, schema.Flag)
		}
	})

	t.Run("Happy path: the character hit die only allows 6, 8, 10 and 12", func(t *testing.T) {
		schema := generatedLeafAt(character.Fields, []string{fifthHitDiceSection, fifthHitDiePrimaryKey, fifthHitDieSizeKey})
		require.NotNil(t, schema)
		assert.True(t, schema.Generatable)
		assert.Equal(t, []float64{6, 8, 10, 12}, schema.GenerationValues)
	})

	t.Run("Happy path: the character spell slots are generatable and non-negative", func(t *testing.T) {
		for level := 1; level <= 9; level++ {
			schema := generatedLeafAt(character.Fields, []string{fifthSpellcastingSection, fifthSpellSlotKey(level), fieldNameTotal})
			require.NotNil(t, schema, "slot %d", level)
			assert.True(t, schema.Generatable, "slot %d", level)
			require.NotNil(t, schema.Min, "slot %d", level)
			assert.InDelta(t, 0.0, *schema.Min, 1e-9, "slot %d", level)
		}
		pact := generatedLeafAt(character.Fields, []string{fifthSpellcastingSection, fifthPactKey, fifthPactCountKey})
		require.NotNil(t, pact)
		assert.True(t, pact.Generatable)
		require.NotNil(t, pact.Min)
		assert.InDelta(t, 0.0, *pact.Min, 1e-9)
	})

	t.Run("Happy path: the character level prefers one and experience prefers zero", func(t *testing.T) {
		level := generatedLeafAt(character.Fields, []string{fifthIdentitySection, fifthLevelKey})
		require.NotNil(t, level)
		assert.True(t, level.PreferDefault)
		assert.InDelta(t, 1.0, level.Default, 1e-9)

		xp := generatedLeafAt(character.Fields, []string{fifthStatusSection, fifthXPKey})
		require.NotNil(t, xp)
		assert.True(t, xp.Generatable)
		assert.True(t, xp.PreferDefault)
		assert.InDelta(t, 0.0, xp.Default, 1e-9)
	})

	t.Run("Happy path: the challenge rating comes from the concept with no preferred default", func(t *testing.T) {
		cr := generatedLeafAt(statBlock.Fields, []string{fifthIdentitySection, fifthChallengeRatingKey})
		require.NotNil(t, cr)
		assert.True(t, cr.Generatable)
		assert.False(t, cr.PreferDefault)
	})

	t.Run("Happy path: feature and trait text is capped at 300 runes", func(t *testing.T) {
		feature := generatedLeafAt(character.Fields, []string{
			fifthFeaturesSection, fifthRecordListKey, fifthFeatureTextKey,
		})
		require.NotNil(t, feature)
		require.NotNil(t, feature.MaxLength)
		assert.Equal(t, fifthRecordTextMaxLength, *feature.MaxLength)

		trait := generatedLeafAt(statBlock.Fields, []string{
			fifthStatBlockTraitsSection, fifthStatBlockRecordTextKey,
		})
		require.NotNil(t, trait)
		require.NotNil(t, trait.MaxLength)
		assert.Equal(t, fifthRecordTextMaxLength, *trait.MaxLength)
	})

	t.Run("Happy path: record state leaves opt out of generation", func(t *testing.T) {
		resourceUsed := generatedLeafAt(character.Fields, []string{
			fifthResourcesSection, fifthRecordListKey, fifthResourceUsedKey,
		})
		require.NotNil(t, resourceUsed)
		assert.True(t, resourceUsed.NotGeneratable)

		statBlockUsed := generatedLeafAt(statBlock.Fields, []string{
			fifthStatBlockResourcesSection, fifthStatBlockRecordUsedKey,
		})
		require.NotNil(t, statBlockUsed)
		assert.True(t, statBlockUsed.NotGeneratable)
	})
}
