package template

func fifthCharacterRowDice() map[string]string {
	rows := fifthHitDiceRowDice()
	for key, expression := range fifthSpellcastingRowDice() {
		rows[key] = expression
	}
	return rows
}
