package template_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_FetchTemplate(t *testing.T) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	svc := template.NewService(registry)

	tests := []struct {
		name       string
		cmd        *template.FetchTemplateByIDCommand
		wantID     string
		wantSentin error
		wantCode   apperr.Code
		wantStatus int
		wantMsg    string
	}{
		{
			name:   "Happy path: existing template is returned",
			cmd:    &template.FetchTemplateByIDCommand{ID: "fantasy_v1"},
			wantID: "fantasy_v1",
		},
		{
			name:       "Edge: empty ID is rejected before the registry",
			cmd:        &template.FetchTemplateByIDCommand{ID: ""},
			wantSentin: template.ErrTemplateIDNotProvided,
			wantMsg:    "template id not provided",
		},
		{
			name:       "Edge: nil command is rejected instead of panicking",
			cmd:        nil,
			wantSentin: template.ErrTemplateIDNotProvided,
			wantMsg:    "template id not provided",
		},
		{
			name:       "Error: malformed slug is rejected before the registry",
			cmd:        &template.FetchTemplateByIDCommand{ID: "Fantasy-V1"},
			wantSentin: template.ErrTemplateIDInvalid,
			wantMsg:    "template id is not a valid slug",
		},
		{
			name:       "Error: leading digit is rejected before the registry",
			cmd:        &template.FetchTemplateByIDCommand{ID: "1fantasy"},
			wantSentin: template.ErrTemplateIDInvalid,
			wantMsg:    "template id is not a valid slug",
		},
		{
			name:       "Error: unknown ID maps to 404",
			cmd:        &template.FetchTemplateByIDCommand{ID: "does_not_exist"},
			wantCode:   apperr.CodeNotFound,
			wantStatus: 404,
			wantMsg:    "template not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := svc.FetchTemplate(context.Background(), tc.cmd)

			if tc.wantSentin == nil && tc.wantCode == "" {
				require.NoError(t, err)
				require.NotNil(t, tmpl)
				assert.Equal(t, tc.wantID, tmpl.ID)
				return
			}

			require.Error(t, err)
			assert.Nil(t, tmpl)
			assert.Equal(t, tc.wantMsg, err.Error())

			if tc.wantSentin != nil {
				require.ErrorIs(t, err, tc.wantSentin)

				var appErr *apperr.Error
				assert.NotErrorAs(t, err, &appErr, "typed ID error must not be returned as apperr from the template layer")
				return
			}

			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
		})
	}
}
