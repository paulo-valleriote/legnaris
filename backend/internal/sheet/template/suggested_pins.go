package template

// MaxSuggestedPins is the capacity of the client's vitals strip; a template
// may not suggest more Rows than the strip can show.
const MaxSuggestedPins = 6

// ValidateSuggestedPins checks the template's suggested pins against the Rows
// it actually has. Unlike a player's Layout, the list is template content
// reviewed in code, so it is strict: every key must be a Row of some Section
// (same grammar as Layout pins), appear once, and the list may not exceed
// MaxSuggestedPins. An empty or absent list is valid.
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
