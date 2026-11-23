package template

func ValidateNotD20Tests(tmpl Template) error {
	if len(tmpl.NotD20Sections) == 0 && len(tmpl.NotD20Rows) == 0 {
		return nil
	}

	sections := make(map[string]bool)
	for section := range sectionRowsForFit(tmpl) {
		sections[section] = true
	}
	rows := staticRowKeys(tmpl)

	for _, key := range tmpl.NotD20Sections {
		if !sections[key] {
			return ErrUnknownNotD20Section(key)
		}
	}
	for _, key := range tmpl.NotD20Rows {
		if !rows[key] {
			return ErrUnknownNotD20Row(key)
		}
	}
	return nil
}
