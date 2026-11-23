package template

func ValidateNotD20Tests(tmpl Template) error {
	if len(tmpl.NotD20Sections) == 0 && len(tmpl.NotD20Rows) == 0 {
		return nil
	}

	sections := make(map[string]bool)
	rows := make(map[string]bool)
	for _, section := range SectionRowKeys(tmpl) {
		sections[section.Section] = true
		for _, row := range section.Rows {
			rows[row] = true
		}
	}

	for _, key := range tmpl.NotD20Sections {
		if !sections[key] {
			return &UnknownNotD20SectionError{Key: key}
		}
	}
	for _, key := range tmpl.NotD20Rows {
		if !rows[key] {
			return &UnknownNotD20RowError{Key: key}
		}
	}
	return nil
}
