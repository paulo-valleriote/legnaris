package template

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fifthStatBlockXPCases = [][2]float64{
	{0, 10},
	{0.125, 25},
	{0.25, 50},
	{0.5, 100},
	{1, 200},
	{2, 450},
	{3, 700},
	{4, 1100},
	{5, 1800},
	{6, 2300},
	{7, 2900},
	{8, 3900},
	{9, 5000},
	{10, 5900},
	{11, 7200},
	{12, 8400},
	{13, 10000},
	{14, 11500},
	{15, 13000},
	{16, 15000},
	{17, 18000},
	{18, 20000},
	{19, 22000},
	{20, 25000},
	{21, 33000},
	{22, 41000},
	{23, 50000},
	{24, 62000},
	{25, 75000},
	{26, 90000},
	{27, 105000},
	{28, 120000},
	{29, 135000},
	{30, 155000},
}

func TestFifthStatBlock_XP(t *testing.T) {
	statBlock := fifthTemplateByID(t, fifthStatBlockID)

	for _, tc := range fifthStatBlockXPCases {
		cr, want := tc[0], tc[1]
		t.Run(fmt.Sprintf("Happy path: CR %g gives %g XP", cr, want), func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{"identity.challengeRating": cr})
			assert.InDelta(t, want, fifthValue(t, data, "experience.xp"), 1e-9)
		})
	}
}

func TestFifthStatBlock_XPZero(t *testing.T) {
	tests := []struct {
		name string
		cr   float64
		zero float64
		want float64
	}{
		{name: "Happy path: CR 0 with the flag gives 0", cr: 0, zero: 1, want: 0},
		{name: "Edge: CR 0 without the flag keeps 10", cr: 0, zero: 0, want: 10},
		{name: "Edge: the flag is ignored at CR 1", cr: 1, zero: 1, want: 200},
		{name: "Edge: the flag is ignored at a fractional CR", cr: 0.5, zero: 1, want: 100},
		{name: "Edge: the flag is ignored at CR 30", cr: 30, zero: 1, want: 155000},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{
				"identity.challengeRating": tc.cr,
				"experience.zero":          tc.zero,
			})
			assert.InDelta(t, tc.want, fifthValue(t, data, "experience.xp"), 1e-9)
		})
	}
}

func TestFifthStatBlock_InLairXP(t *testing.T) {
	tests := []struct {
		name   string
		cr     float64
		inLair float64
		zero   float64
		want   float64
	}{
		{name: "Happy path: CR 10 in lair gives the XP of CR 11", cr: 10, inLair: 1, want: 7200},
		{name: "Happy path: CR 14 in lair gives the XP of CR 15", cr: 14, inLair: 1, want: 13000},
		{name: "Happy path: CR 21 in lair gives the XP of CR 22", cr: 21, inLair: 1, want: 41000},
		{name: "Happy path: CR 1 in lair gives the XP of CR 2", cr: 1, inLair: 1, want: 450},
		{name: "Happy path: CR 29 in lair gives the XP of CR 30", cr: 29, inLair: 1, want: 155000},
		{name: "Edge: the flag is off by default", cr: 10, inLair: 0, want: 5900},
		{name: "Edge: a fractional CR keeps the table XP", cr: 0.5, inLair: 1, want: 100},
		{name: "Edge: CR 30 keeps the table XP", cr: 30, inLair: 1, want: 155000},
		{name: "Edge: CR 0 keeps the table XP", cr: 0, inLair: 1, want: 10},
		{name: "Edge: XP 0 wins at CR 0", cr: 0, inLair: 1, zero: 1, want: 0},
		{name: "Edge: XP 0 is ignored when the lair value applies", cr: 10, inLair: 1, zero: 1, want: 7200},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{
				"identity.challengeRating": tc.cr,
				"experience.inLair":        tc.inLair,
				"experience.zero":          tc.zero,
			})
			assert.InDelta(t, tc.want, fifthValue(t, data, "experience.xp"), 1e-9)
		})
	}
}

func TestFifthStatBlock_CreatureHitPoints(t *testing.T) {
	tests := []struct {
		name    string
		count   float64
		size    float64
		con     float64
		wantCon float64
		wantMax float64
	}{
		{name: "Happy path: 2d8 with a +1 Constitution gives 11", count: 2, size: 8, con: 12, wantCon: 2, wantMax: 11},
		{name: "Happy path: 3d6 with no Constitution bonus gives 10", count: 3, size: 6, con: 10, wantCon: 0, wantMax: 10},
		{name: "Happy path: 13d10 with a +2 Constitution gives 97", count: 13, size: 10, con: 14, wantCon: 26, wantMax: 97},
		{name: "Edge: a negative Constitution lowers the average", count: 2, size: 8, con: 8, wantCon: -2, wantMax: 7},
		{name: "Edge: a single d4 with no bonus gives 2", count: 1, size: 4, con: 10, wantCon: 0, wantMax: 2},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{
				"hitPoints.count":              tc.count,
				"hitPoints.size":               tc.size,
				"abilities.constitution.score": tc.con,
			})
			assert.InDelta(t, tc.wantCon, fifthValue(t, data, "hitPoints.conBonus"), 1e-9)
			assert.InDelta(t, tc.wantMax, fifthValue(t, data, "hitPoints.maximum"), 1e-9)
		})
	}
}

func TestFifthStatBlock_HitPointRoll(t *testing.T) {
	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	row := "hitPoints.maximum"

	t.Run("Happy path: the roll reads the count and size from Fields", func(t *testing.T) {
		expression := statBlock.RowDice[row]
		assert.Equal(t, "{hitPoints.count}d{hitPoints.size} + hitPoints.conBonus", expression)
		assert.LessOrEqual(t, len(expression), 64)
	})

	t.Run("Happy path: the roll is not a d20 test", func(t *testing.T) {
		assert.Contains(t, statBlock.NotD20Rows, row)
		assert.NotContains(t, statBlock.NotD20Sections, fifthStatBlockHitPointsSection)
	})

	t.Run("Happy path: the dice defaults validate", func(t *testing.T) {
		require.NoError(t, ValidateDiceDefaults(statBlock))
		require.NoError(t, ValidateNotD20Tests(statBlock))
	})

	t.Run("Edge: the hit point Section carries no Section dice", func(t *testing.T) {
		_, ok := statBlock.SectionDice[fifthStatBlockHitPointsSection]
		assert.False(t, ok)
	})
}

func TestFifthStatBlock_Initiative(t *testing.T) {
	tests := []struct {
		name     string
		modifier float64
		want     float64
	}{
		{name: "Happy path: a positive modifier raises the score", modifier: 3, want: 13},
		{name: "Happy path: the default score is 10", modifier: 0, want: 10},
		{name: "Edge: a negative modifier lowers the score", modifier: -2, want: 8},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{"initiative.modifier": tc.modifier})
			assert.InDelta(t, tc.want, fifthValue(t, data, "initiative.score"), 1e-9)
		})
	}

	t.Run("Happy path: the score is derived", func(t *testing.T) {
		score := statBlock.Fields[fifthStatBlockInitiativeSection].Children[fifthStatBlockInitiativeScoreKey].Schema
		require.NotNil(t, score)
		assert.False(t, score.IsEditable)
		assert.NotEmpty(t, score.Formula)
	})

	t.Run("Happy path: the initiative roll is a d20 test with the exhaustion penalty", func(t *testing.T) {
		expression := statBlock.RowDice["initiative.score"]
		assert.Equal(t, "1d20 + initiative.modifier + exhaustion.penalty", expression)
		assert.LessOrEqual(t, len(expression), 64)
		assert.NotContains(t, statBlock.NotD20Rows, "initiative.score")
	})

	t.Run("Edge: exhaustion never changes the score", func(t *testing.T) {
		data := fifthSheet(t, statBlock, map[string]any{
			"initiative.modifier": float64(3),
			"exhaustion.level":    float64(6),
		})
		assert.InDelta(t, 13.0, fifthValue(t, data, "initiative.score"), 1e-9)
	})
}

func TestFifthStatBlock_Speed(t *testing.T) {
	tests := []struct {
		name      string
		exhausted float64
		want      float64
	}{
		{name: "Happy path: no exhaustion keeps the printed speed", exhausted: 0, want: 40},
		{name: "Happy path: one level removes 5 feet", exhausted: 1, want: 35},
		{name: "Happy path: three levels remove 15 feet", exhausted: 3, want: 25},
		{name: "Edge: a low speed may go below zero", exhausted: 6, want: 10},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{
				"speed.walk":       float64(40),
				"exhaustion.level": tc.exhausted,
			})
			assert.InDelta(t, tc.want, fifthValue(t, data, "speed.walkTotal"), 1e-9)
		})
	}

	t.Run("Happy path: every speed applies the same penalty", func(t *testing.T) {
		data := fifthSheet(t, statBlock, map[string]any{
			"speed.walk":       float64(30),
			"speed.fly":        float64(60),
			"speed.swim":       float64(40),
			"speed.climb":      float64(20),
			"speed.burrow":     float64(10),
			"exhaustion.level": float64(2),
		})
		assert.InDelta(t, 20.0, fifthValue(t, data, "speed.walkTotal"), 1e-9)
		assert.InDelta(t, 50.0, fifthValue(t, data, "speed.flyTotal"), 1e-9)
		assert.InDelta(t, 30.0, fifthValue(t, data, "speed.swimTotal"), 1e-9)
		assert.InDelta(t, 10.0, fifthValue(t, data, "speed.climbTotal"), 1e-9)
		assert.InDelta(t, 0.0, fifthValue(t, data, "speed.burrowTotal"), 1e-9)
	})

	t.Run("Happy path: walking defaults to 30 feet", func(t *testing.T) {
		data := fifthSheet(t, statBlock, nil)
		assert.InDelta(t, 30.0, fifthValue(t, data, "speed.walkTotal"), 1e-9)
	})

	t.Run("Edge: a speed that is not printed stays zero", func(t *testing.T) {
		data := fifthSheet(t, statBlock, nil)
		assert.InDelta(t, 0.0, fifthValue(t, data, "speed.flyTotal"), 1e-9)
	})
}

func TestFifthStatBlock_CombatDiceStayInsideTheCap(t *testing.T) {
	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for row, expression := range statBlock.RowDice {
		assert.LessOrEqual(t, len(expression), 64, row)
		assert.False(t, strings.HasPrefix(strings.TrimSpace(expression), "-"), row)
	}
}
