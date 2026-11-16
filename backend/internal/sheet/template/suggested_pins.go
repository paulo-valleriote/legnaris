package template

const MaxSuggestedPins = 6

func ValidateSuggestedPins(tmpl Template) error {
	if len(tmpl.SuggestedPins) > MaxSuggestedPins {
		return &TooManySuggestedPinsError{Count: len(tmpl.SuggestedPins), Max: MaxSuggestedPins}
	}

	rows := make(map[string]bool)
	for _, section := range SectionRowKeys(tmpl) {
		for _, row := range section.Rows {
			rows[row] = true
		}
	}

	seen := make(map[string]bool, len(tmpl.SuggestedPins))
	for _, key := range tmpl.SuggestedPins {
		if !rows[key] {
			return &UnknownSuggestedPinError{Key: key}
		}
		if seen[key] {
			return &DuplicateSuggestedPinError{Key: key}
		}
		seen[key] = true
	}
	return nil
}
