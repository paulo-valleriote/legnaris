package sheet_test

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/sheet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSheetDTO_CampaignIDIsOmittedWhenUnlinked(t *testing.T) {
	linked := "01900000-0000-7000-8000-0000000000c1"
	tests := []struct {
		name       string
		campaignID *string
		wantKey    bool
	}{
		{name: "Edge: never linked omits campaign_id", campaignID: nil, wantKey: false},
		{name: "Happy path: linked exposes campaign_id", campaignID: &linked, wantKey: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := sheet.Sheet{ID: "01900000-0000-7000-8000-0000000000d1", CampaignID: tc.campaignID}.ToDTO()
			raw, err := json.Marshal(dto)
			require.NoError(t, err)

			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			got, present := body["campaign_id"]
			assert.Equal(t, tc.wantKey, present)
			if tc.wantKey {
				assert.Equal(t, linked, got)
			}
		})
	}
}

func TestSheetDTO_CampaignIDDoesNotAliasStoredValue(t *testing.T) {
	linked := "01900000-0000-7000-8000-0000000000c1"
	stored := sheet.Sheet{CampaignID: &linked}
	dto := stored.ToDTO()
	require.NotNil(t, dto.CampaignID)

	*dto.CampaignID = "mutated"
	assert.Equal(t, "01900000-0000-7000-8000-0000000000c1", *stored.CampaignID)
}
