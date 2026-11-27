package sheet_test

import (
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSheetName(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantName   string
		wantCode   apperr.Code
		wantStatus int
	}{
		{name: "Happy path: a plain name is kept", raw: "Aria", wantName: "Aria"},
		{name: "Happy path: surrounding spaces are trimmed", raw: "  Aria  ", wantName: "Aria"},
		{name: "Edge: inner spaces are preserved", raw: " Aria  Storm ", wantName: "Aria  Storm"},
		{name: "Edge: a name of one rune is valid", raw: "A", wantName: "A"},
		{name: "Error: an empty name is invalid", raw: "", wantCode: apperr.CodeSheetNameInvalid, wantStatus: 422},
		{name: "Error: a whitespace-only name is invalid", raw: "   ", wantCode: apperr.CodeSheetNameInvalid, wantStatus: 422},
		{name: "Error: tabs and newlines are invalid", raw: "\t\n", wantCode: apperr.CodeSheetNameInvalid, wantStatus: 422},
		{name: "Error: a non-breaking space is invalid", raw: "\u00a0", wantCode: apperr.CodeSheetNameInvalid, wantStatus: 422},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, err := sheet.ValidateSheetName(tc.raw)
			if tc.wantCode == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.wantName, name)
				return
			}
			require.Error(t, err)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
		})
	}
}
