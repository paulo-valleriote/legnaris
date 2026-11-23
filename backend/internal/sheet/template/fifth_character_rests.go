package template

const (
	fifthLongRestActionID  = "long_rest"
	fifthShortRestActionID = "short_rest"
)

func fifthCharacterRestActions() []ResetAction {
	return []ResetAction{
		{
			ID:          fifthLongRestActionID,
			Label:       "Long rest",
			Assignments: fifthLongRestAssignments(),
		},
		{
			ID:          fifthShortRestActionID,
			Label:       "Short rest",
			Assignments: fifthShortRestAssignments(),
		},
	}
}

func fifthLongRestAssignments() []ResetAssignment {
	assignments := []ResetAssignment{
		{
			Target:     Path{fifthCombatSection, fifthHitPointsKey, fifthHPCurrentKey}.String(),
			Expression: Path{fifthCombatSection, fifthHitPointsKey, fifthHPMaxKey}.EngineKey(),
		},
		{
			Target: Path{fifthExhaustionSection, fifthExhaustionLevelKey}.String(),
			Expression: "max(0, " +
				Path{fifthExhaustionSection, fifthExhaustionLevelKey}.EngineKey() + " - 1)",
		},
		{
			Target:     Path{fifthStatusSection, fifthDeathSavesKey, fifthSuccessesKey}.String(),
			Expression: "0",
		},
		{
			Target:     Path{fifthStatusSection, fifthDeathSavesKey, fifthFailuresKey}.String(),
			Expression: "0",
		},
		{
			Target:     Path{fifthSpellcastingSection, fifthPactKey, fifthPactUsedKey}.String(),
			Expression: "0",
		},
	}

	for _, target := range fifthHitDieSpentTargets() {
		assignments = append(assignments, ResetAssignment{Target: target, Expression: "0"})
	}
	for level := 1; level <= 9; level++ {
		assignments = append(assignments, ResetAssignment{
			Target: Path{
				fifthSpellcastingSection, fifthSpellSlotKey(level), fifthSpellSlotSpentKey,
			}.String(),
			Expression: "0",
		})
	}

	return append(assignments, fifthResourceResetAssignment(
		"record_"+fifthResourceUsedKey+" * record_"+fifthResourceRechargeNoneKey,
	))
}

func fifthShortRestAssignments() []ResetAssignment {
	return []ResetAssignment{
		{
			Target:     Path{fifthSpellcastingSection, fifthPactKey, fifthPactUsedKey}.String(),
			Expression: "0",
		},
		fifthResourceResetAssignment(
			"record_" + fifthResourceUsedKey + " * (1 - record_" + fifthResourceRechargeShortKey + ")",
		),
	}
}

func fifthResourceResetAssignment(expression string) ResetAssignment {
	return ResetAssignment{
		Target:     Path{fifthResourcesSection, fifthRecordListKey, fifthResourceUsedKey}.String(),
		Expression: expression,
	}
}

func fifthHitDieSpentTargets() []string {
	targets := make([]string, 0, len(fifthExtraHitDice)+1)
	targets = append(targets, Path{
		fifthHitDiceSection, fifthHitDiePrimaryKey, fifthHitDieSpentKey,
	}.String())
	for _, size := range fifthExtraHitDice {
		targets = append(targets, Path{fifthHitDiceSection, size, fifthHitDieSpentKey}.String())
	}
	return targets
}
