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

const fifthLongRestRequestID = "01900000-0000-7000-8000-0000000000f1"

func createFifthSheet(t *testing.T, h http.Handler, token, templateID, kind, name string) sheet.SheetDTO {
	t.Helper()

	body := fmt.Sprintf(`{"template_id":%q,"kind":%q,"sheet_name":%q}`, templateID, kind, name)
	rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, templateID, created.Data.TemplateID)
	return created.Data
}

func fetchFifthSheet(t *testing.T, h http.Handler, token, sheetID string) sheet.SheetDTO {
	t.Helper()

	rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fetched))
	return fetched.Data
}

func nestedNumber(t *testing.T, data map[string]any, path ...string) float64 {
	t.Helper()

	current := data
	for _, segment := range path[:len(path)-1] {
		next, ok := current[segment].(map[string]any)
		require.True(t, ok, "%s is not an object in %v", segment, path)
		current = next
	}
	value, ok := current[path[len(path)-1]].(float64)
	require.True(t, ok, "%v is not a number in %v", path, current)
	return value
}

func TestIntegration_FifthEditionTemplates(t *testing.T) {
	h, mdb := newIntegrationStackAllowingDrafts(t)

	gmToken, _ := newAuthUser(t, h, mdb, "fifth-gm@example.com", "fifth-gm")
	playerToken, _ := newAuthUser(t, h, mdb, "fifth-player@example.com", "fifth-player")

	rec := doAuthRequest(t, h, gmToken, http.MethodPost, "/campaigns", `{"name":"Underdark","game_system":"fifth"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var campaignBody response.Success[campaign.View]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &campaignBody))
	campaignID := campaignBody.Data.ID
	require.Equal(t, "5E", campaignBody.Data.ScopeName)
	assert.ElementsMatch(t,
		[]string{"fifth_character_v1", "fifth_stat_block_v1"},
		campaignBody.Data.TemplateIDs,
	)
	require.NotEmpty(t, campaignBody.Data.InviteCode)

	rec = doAuthRequest(t, h, playerToken, http.MethodPost, "/campaigns/join",
		fmt.Sprintf(`{"code":%q}`, campaignBody.Data.InviteCode))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var joinedBody response.Success[campaign.View]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &joinedBody))
	require.Equal(t, campaign.RolePlayer, joinedBody.Data.ViewerRole)

	character := createFifthSheet(t, h, playerToken, "fifth_character_v1", "character", "Brenna")
	characterPath := "/sheets/" + character.ID

	t.Run("Happy path: a 5E Character patches an entered value and reads the derived cascade", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPatch, characterPath,
			`{"data":{"identity":{"level":5},"abilities":{"strength":{"score":16}}}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var effects response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &effects))
		assert.InDelta(t, 3.0, nestedNumber(t, effects.Data, "identity", "proficiencyBonus"), 1e-9)
		assert.InDelta(t, 3.0, nestedNumber(t, effects.Data, "abilities", "strength", "mod"), 1e-9)
	})

	t.Run("Happy path: a dice override on a d20 Row saves", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPatch, characterPath,
			`{"dice":{"row":{"skills.perception":"1d20 + @.total"}}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		stored := fetchFifthSheet(t, h, playerToken, character.ID)
		require.NotNil(t, stored.Dice.Row["skills.perception"])
		assert.Equal(t, "1d20 + @.total", *stored.Dice.Row["skills.perception"])
	})

	t.Run("Error: an unknown dice Row is refused", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPatch, characterPath,
			`{"dice":{"row":{"skills.ghost":"1d20"}}}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var failure response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &failure))
		assert.Equal(t, "INVALID_REQUEST", failure.Error.Code)
	})

	t.Run("Happy path: the 5E Character long rest restores hit dice, slots and exhaustion", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPatch, characterPath,
			`{"data":{"exhaustion":{"level":2},"hitDice":{"primary":{"spent":2}},`+
				`"spellcasting":{"slot1":{"spent":1}}}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		rec = doAuthRequest(t, h, playerToken, http.MethodPost, characterPath+"/resets/long_rest",
			fmt.Sprintf(`{"request_id":%q}`, fifthLongRestRequestID))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var effects response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &effects))
		assert.InDelta(t, 1.0, nestedNumber(t, effects.Data, "exhaustion", "level"), 1e-9)
		assert.InDelta(t, 0.0, nestedNumber(t, effects.Data, "hitDice", "primary", "spent"), 1e-9)
		assert.InDelta(t, 0.0, nestedNumber(t, effects.Data, "spellcasting", "slot1", "spent"), 1e-9)

		stored := fetchFifthSheet(t, h, playerToken, character.ID)
		assert.InDelta(t, 1.0, nestedNumber(t, stored.Data, "exhaustion", "level"), 1e-9)
		assert.InDelta(t, 0.0, nestedNumber(t, stored.Data, "hitDice", "primary", "spent"), 1e-9)
	})

	t.Run("Happy path: the Player links the 5E Character into the fifth Campaign", func(t *testing.T) {
		rec := doAuthRequest(t, h, playerToken, http.MethodPut, "/campaigns/"+campaignID+"/character",
			fmt.Sprintf(`{"sheet_id":%q}`, character.ID))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var linked response.Success[campaign.View]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &linked))
		var characterName string
		for _, member := range linked.Data.Members {
			if member.Role == campaign.RolePlayer {
				characterName = member.CharacterName
			}
		}
		require.Equal(t, "Brenna", characterName)
	})

	statBlock := createFifthSheet(t, h, gmToken, "fifth_stat_block_v1", "npc", "Gray Ooze")
	statBlockPath := "/sheets/" + statBlock.ID

	t.Run("Happy path: a 5E Stat Block derives XP, proficiency bonus and creature hit points", func(t *testing.T) {
		rec := doAuthRequest(t, h, gmToken, http.MethodPatch, statBlockPath,
			`{"data":{"identity":{"challengeRating":5},"hitPoints":{"count":3,"size":8}}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var effects response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &effects))
		assert.InDelta(t, 3.0, nestedNumber(t, effects.Data, "identity", "proficiencyBonus"), 1e-9)
		assert.InDelta(t, 1800.0, nestedNumber(t, effects.Data, "experience", "xp"), 1e-9)
		assert.InDelta(t, 13.0, nestedNumber(t, effects.Data, "hitPoints", "maximum"), 1e-9)
	})

	t.Run("Happy path: the GM links the 5E Stat Block into the fifth Campaign as an NPC", func(t *testing.T) {
		rec := doAuthRequest(t, h, gmToken, http.MethodPut, "/campaigns/"+campaignID+"/npcs",
			fmt.Sprintf(`{"sheet_id":%q}`, statBlock.ID))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var linked response.Success[campaign.NpcLinkView]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &linked))
		assert.Equal(t, statBlock.ID, linked.Data.SheetID)
		assert.False(t, linked.Data.Revealed)

		rec = doAuthRequest(t, h, playerToken, http.MethodGet, "/campaigns/"+campaignID+"/npcs", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var hidden response.Success[[]campaign.NpcLinkView]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &hidden))
		assert.Empty(t, hidden.Data, "a Player sees no unrevealed NPC link")
	})
}
