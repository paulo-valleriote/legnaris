package sheet

import (
	"strings"

	"legnaris/backend/shared/apperr"
)

const MsgSheetNameRequired = "sheet name is required"

func ValidateSheetName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", apperr.NewError(apperr.CodeSheetNameInvalid, MsgSheetNameRequired, nil)
	}
	return name, nil
}
