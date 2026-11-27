package sheet_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Create_RejectsBlankSheetName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "Error: an empty name is refused before the template is read", raw: ""},
		{name: "Error: a whitespace-only name is refused before the template is read", raw: "   "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := sheet.NewService(nil, nil, nil, testLog, events.NoOp{})

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				OwnerID:    ownerUserID,
				TemplateID: "test",
				Kind:       string(plan.KindCharacter),
				SheetName:  tc.raw,
			})

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeSheetNameInvalid, appErr.Code)
			assert.Equal(t, 422, appErr.HttpStatus())
		})
	}
}
