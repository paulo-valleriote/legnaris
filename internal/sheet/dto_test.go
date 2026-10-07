package sheet_test

import (
	"encoding/json"
	"testing"
	"time"

	"legnaris/internal/sheet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tamperData rewrites a payload the way a caller holding the DTO would: one new
// top-level key plus a write into every nested group. It returns how many writes
// actually landed, so a test can tell a real mutation from a no-op.
func tamperData(data map[string]any) int {
	if data == nil {
		return 0
	}
	writes := 1
	data["tampered"] = true
	for _, value := range data {
		if nested, ok := value.(map[string]any); ok {
			nested["tampered"] = true
			writes++
		}
	}
	return writes
}

// TestSheetToDTO_DoesNotAliasData covers the LGN-60 finding that ToDTO handed
// out the Sheet's own map: mutating the response payload rewrote stored state,
// and mutating the stored sheet rewrote an already-built response.
func TestSheetToDTO_DoesNotAliasData(t *testing.T) {
	tests := []struct {
		name    string
		newData func() map[string]any
	}{
		{
			name: "Happy path: nested groups",
			newData: func() map[string]any {
				return map[string]any{
					"stats":   map[string]any{"dexterity": 12.0, "constitution": 10.0},
					"derived": map[string]any{"armorClass": 15.0},
				}
			},
		},
		{
			name:    "Edge: nil data",
			newData: func() map[string]any { return nil },
		},
		{
			name:    "Edge: empty data",
			newData: func() map[string]any { return map[string]any{} },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("tampered DTO leaves the source untouched", func(t *testing.T) {
				src := sheet.Sheet{ID: "sheet-1", Data: tc.newData()}

				dto := src.ToDTO()
				require.Equal(t, tc.newData(), dto.Data, "DTO must keep the source contents")

				writes := tamperData(dto.Data)
				if src.Data != nil {
					require.Positive(t, writes, "tampering must actually write for this fixture")
				}

				assert.Equal(t, tc.newData(), src.Data, "mutating dto.Data changed the source sheet")
			})

			t.Run("mutating the source leaves the DTO untouched", func(t *testing.T) {
				src := sheet.Sheet{ID: "sheet-1", Data: tc.newData()}

				dto := src.ToDTO()
				tamperData(src.Data)

				assert.Equal(t, tc.newData(), dto.Data, "mutating the sheet changed its DTO")
			})
		})
	}
}

// TestSheetToDTO_Edge_NilData keeps the JSON contract stable: a sheet without
// data must serialize as null, not as an empty object.
func TestSheetToDTO_Edge_NilData(t *testing.T) {
	src := sheet.Sheet{ID: "sheet-1"}

	dto := src.ToDTO()

	assert.Nil(t, dto.Data)
}

func TestSheetToDTO_HappyPath_CarriesOverEveryField(t *testing.T) {
	createdAt := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	src := sheet.Sheet{
		ID:         "sheet-1",
		TemplateID: "fantasy_v1",
		Name:       "Character",
		Data:       map[string]any{"stats": map[string]any{"dexterity": 12.0}},
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}

	dto := src.ToDTO()

	assert.Equal(t, "sheet-1", dto.ID)
	assert.Equal(t, "fantasy_v1", dto.TemplateID)
	assert.Equal(t, "Character", dto.Name)
	assert.Equal(t, createdAt, dto.CreatedAt)
	assert.Equal(t, updatedAt, dto.UpdatedAt)
	assert.Equal(t, src.Data, dto.Data)
}

// TestSheetToDTO_NeverExposesOwnerID locks the API contract that the owner is
// server-side only (OWN-01): a sheet carrying an owner must serialize with no
// owner key and no trace of the owner id, so a client can never read or forge
// possession from the wire.
func TestSheetToDTO_NeverExposesOwnerID(t *testing.T) {
	src := sheet.Sheet{
		ID:         "sheet-1",
		TemplateID: "fantasy_v1",
		OwnerID:    "01900000-0000-7000-8000-0000000000aa",
		Name:       "Owned",
		Data:       map[string]any{"stats": map[string]any{"dexterity": 12.0}},
	}

	raw, err := json.Marshal(src.ToDTO())
	require.NoError(t, err)

	assert.NotContains(t, string(raw), "owner_id")
	assert.NotContains(t, string(raw), src.OwnerID)
}
