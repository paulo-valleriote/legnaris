package app_test

import (
	"encoding/json"
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

	gmToken, _ := newAuthUser(t, h, mdb, "fantasy-gm@example.com", "fantasy-gm")
	playerToken, _ := newAuthUser(t, h, mdb, "fantasy-player@example.com", "fantasy-player")

	rec := doAuthRequest(t, h, playerToken, http.MethodPost, "/sheets",
		`{"template_id":"fantasy_v1","kind":"character","sheet_name":"Aria"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, "fantasy_v1", created.Data.TemplateID)
	sheetID := created.Data.ID
	sheetPath := "/sheets/" + sheetID

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

	rec = doAuthRequest(t, h, gmToken, http.MethodPost, "/campaigns",
		`{"name":"The Ember Crown","template_id":"fantasy_v1"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var campaignBody response.Success[campaign.View]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &campaignBody))
	assert.Equal(t, "fantasy_v1", campaignBody.Data.TemplateID)
	assert.Contains(t, campaignBody.Data.TemplateIDs, "fantasy_v1")
	require.NotEmpty(t, campaignBody.Data.InviteCode)
	campaignID := campaignBody.Data.ID

	rec = doAuthRequest(t, h, playerToken, http.MethodPost, "/campaigns/join",
		`{"code":"`+campaignBody.Data.InviteCode+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var joined response.Success[campaign.View]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &joined))
	require.Equal(t, campaign.RolePlayer, joined.Data.ViewerRole)

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
