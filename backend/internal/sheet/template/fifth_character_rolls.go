package template

func fifthCharacterRowDice() map[string]string {
	rows := fifthHitDiceRowDice()
	for key, expression := range fifthSpellcastingRowDice() {
		rows[key] = expression
	}
	rows[Path{fifthCombatSection, fifthInitiativeKey}.String()] = fifthInitiativeDice
	rows[Path{fifthStatusSection, fifthDeathSavesKey}.String()] = fifthDeathSaveDice
	return rows
}
