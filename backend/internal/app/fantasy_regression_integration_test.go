package app_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_FantasyV1Regression(t *testing.T) {
	h, mdb := newIntegrationRouter(t)

	gmToken, gmID := newAuthUser(t, h, mdb, "fantasy-gm@example.com", "fantasy-gm")
	playerToken, _ := newAuthUser(t, h, mdb, "fantasy-player@example.com", "fantasy-player")

	campaignID := insertFantasyCampaign(t, mdb, gmID)

	rec := doAuthRequest(t, h, playerToken, http.MethodPost, "/campaigns/join", fmt.Sprintf(`{"code":%q}`, fantasyInviteCode(campaignID)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var joined response.Success[campaign.View]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &joined))
	require.Equal(t, campaign.RolePlayer, joined.Data.ViewerRole)
	assert.Equal(t, "fantasy_v1", joined.Data.TemplateID)
	assert.Contains(t, joined.Data.TemplateIDs, "fantasy_v1")

	createCampaignSheet := func(t *testing.T, kind, name string) sheet.SheetDTO {
		t.Helper()
		body := fmt.Sprintf(
			`{"template_id":"fantasy_v1","kind":%q,"sheet_name":%q,"campaign_id":%q}`,
			kind, name, campaignID,
		)
		rec := doAuthRequest(t, h, playerToken, http.MethodPost, "/sheets", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

		var created response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
		require.Equal(t, "fantasy_v1", created.Data.TemplateID)
		return created.Data
	}

	character := createCampaignSheet(t, "character", "Aria")
	assert.Equal(t, "character", character.Kind)
	assert.Equal(t, "fantasy_v1", character.TemplateID)
	sheetID := character.ID
	sheetPath := "/sheets/" + sheetID

	t.Run("Happy path: a retired Template still starts NPCs and enemies from its existing Campaign", func(t *testing.T) {
		for _, tc := range []struct{ kind, name string }{
			{kind: "npc", name: "Old Tomek"},
			{kind: "enemy", name: "Dire Wolf"},
		} {
			created := createCampaignSheet(t, tc.kind, tc.name)
			assert.Equal(t, tc.kind, created.Kind)
			assert.Equal(t, "fantasy_v1", created.TemplateID)
		}
	})

	t.Run("Happy path: a fantasy Sheet still edits and derives", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPatch, sheetPath,
			`{"data":{"stats":{"dexterity":16}}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var effects response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &effects))
		assert.InDelta(t, 3.0, nestedNumber(t, effects.Data, "modifiers", "dexterityMod"), 1e-9)
		assert.InDelta(t, 13.0, nestedNumber(t, effects.Data, "derived", "armorClass"), 1e-9)
	})

	t.Run("Happy path: a fantasy Sheet still saves a dice expression", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPatch, sheetPath,
			`{"dice":{"row":{"stats.dexterity":"1d20 + modifiers.dexterityMod"}}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		rec = doAuthRequest(t, h, playerToken, http.MethodGet, sheetPath, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var fetched response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fetched))
		require.NotNil(t, fetched.Data.Dice.Row["stats.dexterity"])
		assert.Equal(t, "1d20 + modifiers.dexterityMod", *fetched.Data.Dice.Row["stats.dexterity"])
	})

	t.Run("Happy path: a fantasy Character still links and reads through the Campaign", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPut, "/campaigns/"+campaignID+"/character",
			`{"sheet_id":"`+sheetID+`"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		rec = doAuthRequest(t, h, gmToken, http.MethodGet, "/campaigns/"+campaignID+"/sheets", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var summaries response.Success[[]campaign.LinkedSheetSummary]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summaries))
		require.Len(t, summaries.Data, 1)
		assert.Equal(t, sheetID, summaries.Data[0].ID)
		assert.Equal(t, "Aria", summaries.Data[0].Name)

		rec = doAuthRequest(t, h, playerToken, http.MethodGet,
			"/campaigns/"+campaignID+"/sheets/"+sheetID, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var linked response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &linked))
		assert.Equal(t, sheetID, linked.Data.ID)
		assert.Equal(t, "fantasy_v1", linked.Data.TemplateID)
		assert.InDelta(t, 16.0, nestedNumber(t, linked.Data.Data, "stats", "dexterity"), 1e-9)
		require.NotNil(t, linked.Data.Dice.Row["stats.dexterity"])
		assert.Equal(t, "1d20 + modifiers.dexterityMod", *linked.Data.Dice.Row["stats.dexterity"])
	})
}
