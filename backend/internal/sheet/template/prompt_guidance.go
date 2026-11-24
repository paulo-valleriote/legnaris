package template

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"legnaris/backend/internal/plan"
)

var (
	ErrPromptGuidanceKindInvalid     = errors.New("prompt guidance kind is not a valid sheet kind")
	ErrPromptGuidanceKindNotAccepted = errors.New("prompt guidance kind is not accepted by the template")
	ErrPromptGuidanceEmpty           = errors.New("prompt guidance text is empty")
)

func ValidatePromptGuidance(tmpl Template) error {
	for _, kind := range slices.Sorted(maps.Keys(tmpl.PromptGuidanceByKind)) {
		if _, ok := plan.ParseSheetKind(string(kind)); !ok {
			return fmt.Errorf("%w: %q", ErrPromptGuidanceKindInvalid, kind)
		}
		if !tmpl.AcceptsKind(kind) {
			return fmt.Errorf("%w: %q", ErrPromptGuidanceKindNotAccepted, kind)
		}
		if tmpl.PromptGuidanceByKind[kind] == "" {
			return fmt.Errorf("%w: %q", ErrPromptGuidanceEmpty, kind)
		}
	}
	return nil
}
