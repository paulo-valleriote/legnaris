package sheeterr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"legnaris/backend/internal/sheet/sheeterr"
)

func TestSentinelsAreDistinct(t *testing.T) {
	all := []error{sheeterr.ErrSheetNotFound, sheeterr.ErrVersionConflict, sheeterr.ErrSheetLimitReached}

	tests := []struct {
		name   string
		target error
	}{
		{name: "Happy path: sheet not found matches only itself", target: sheeterr.ErrSheetNotFound},
		{name: "Happy path: version conflict matches only itself", target: sheeterr.ErrVersionConflict},
		{name: "Happy path: sheet limit matches only itself", target: sheeterr.ErrSheetLimitReached},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("repository: %w", tc.target)
			for _, other := range all {
				assert.Equal(t, errors.Is(other, tc.target), errors.Is(wrapped, other))
			}
		})
	}
}
