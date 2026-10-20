package app

import (
	"errors"
	"fmt"
	"testing"

	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateError covers the translation the template controller relies on
// (LGN-66, LGN-86): the missing-ID and malformed-ID sentinels become 400
// apperrs, while anything else — including an apperr the service already
// built — passes through untouched so its status survives to the HTTP layer.
func TestTemplateError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   apperr.Code
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "Happy path: missing template ID maps to 400",
			err:        template.ErrTemplateIDNotProvided,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "template id not provided",
		},
		{
			name:       "Happy path: malformed template ID maps to 400",
			err:        template.ErrTemplateIDInvalid,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "template id is not a valid slug",
		},
		{
			name:       "Edge: wrapped sentinel still maps to 400, keeping the wrapper's context",
			err:        fmt.Errorf("fetch template: %w", template.ErrTemplateIDNotProvided),
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "fetch template: template id not provided",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := templateError(tc.err)
			require.Error(t, got)

			var appErr *apperr.Error
			require.ErrorAs(t, got, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
			assert.Equal(t, tc.wantMsg, appErr.Message)
		})
	}
}

// TestTemplateError_Passthrough keeps the translator honest: it must not wrap
// errors it does not own, or a service's apperr would lose its code and status
// (and an unrelated error would be re-reported as a 400).
func TestTemplateError_Passthrough(t *testing.T) {
	notFound := apperr.NewError(apperr.CodeNotFound, "template not found", nil)

	tests := []struct {
		name string
		err  error
	}{
		{name: "Edge: nil error stays nil", err: nil},
		{name: "Edge: apperr from the service keeps its code", err: notFound},
		{name: "Edge: unrelated error is returned as-is", err: errors.New("boom")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := templateError(tc.err)
			if tc.err == nil {
				assert.NoError(t, got)
				return
			}
			assert.Same(t, tc.err, got)
		})
	}
}
