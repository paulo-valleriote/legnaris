package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestIntegration_Sheets_Layout(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	ctx := context.Background()

	token, _ := newAuthUser(t, h, "layout-owner@example.com", "layout-owner")
	otherToken, _ := newAuthUser(t, h, "layout-other@example.com", "layout-other")

	createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Layout"}`)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID
	path := "/sheets/" + sheetID + "/layout"

	fetch := func(t *testing.T) response.Success[sheet.SheetDTO] {
		t.Helper()
		rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
		require.Equal(t, http.StatusOK, rec.Code)
		var got response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		return got
	}

	layoutBody := `{"layout":{"section_order":["config","stats"],` +
		`"pinned":["skills.stealth","config#spellcastingAbility","stats.dexterity"]}}`
	wantLayout := &sheet.Layout{
		SectionOrder: []string{"config", "stats"},
		Pinned:       []string{"skills.stealth", "config#spellcastingAbility", "stats.dexterity"},
	}

	t.Run("Edge: a fresh sheet has no layout key", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
		assert.NotContains(t, rec.Body.String(), `"layout"`)
	})

	t.Run("Error: no token is 401", func(t *testing.T) {
		rec := doAuthRequest(t, h, "", http.MethodPut, path, layoutBody)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Error: another user gets the same 404 and nothing is stored", func(t *testing.T) {
		rec := doAuthRequest(t, h, otherToken, http.MethodPut, path, layoutBody)
		require.Equal(t, http.StatusNotFound, rec.Code)
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "NOT_FOUND", body.Error.Code)
		assert.Nil(t, fetch(t).Data.Layout)
	})

	t.Run("Error: an invalid layout is 400 and nothing is stored", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, `{"layout":{"pinned":["stats.nope"]}}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
		assert.Nil(t, fetch(t).Data.Layout)
	})

	t.Run("Error: the removed row_order field is 400 INVALID_JSON and nothing is stored", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, `{"layout":{"row_order":{"stats":["stats.strength"]}}}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_JSON", body.Error.Code)
		assert.Nil(t, fetch(t).Data.Layout)
	})

	t.Run("Error: a malformed sheet id is 400", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, "/sheets/not-a-uuid/layout", layoutBody)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Happy path: PUT stores the layout and GET returns it", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, layoutBody)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var put response.Success[struct {
			Layout *sheet.Layout `json:"layout"`
		}]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &put))
		assert.Equal(t, wantLayout, put.Data.Layout)

		assert.Equal(t, wantLayout, fetch(t).Data.Layout)
	})

	t.Run("Edge: re-putting the same layout is idempotent", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, layoutBody)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, wantLayout, fetch(t).Data.Layout)
	})

	t.Run("Edge: a PATCH of data keeps the layout and a PUT keeps the data and derived values", func(t *testing.T) {
		patch := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":16}}}`)
		require.Equal(t, http.StatusOK, patch.Code)

		afterPatch := fetch(t)
		assert.Equal(t, wantLayout, afterPatch.Data.Layout)

		put := doAuthRequest(t, h, token, http.MethodPut, path, `{"layout":{"pinned":["stats.dexterity"]}}`)
		require.Equal(t, http.StatusOK, put.Code)

		afterPut := fetch(t)
		assert.Equal(t, &sheet.Layout{Pinned: []string{"stats.dexterity"}}, afterPut.Data.Layout)
		assert.Equal(t, afterPatch.Data.Data, afterPut.Data.Data, "a layout write must not change values or derived fields")
		assert.Equal(t, afterPatch.Data.UpdatedAt, afterPut.Data.UpdatedAt)
	})

	t.Run("Edge: an empty layout resets to default and the key disappears", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, `{"layout":{}}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"data":{"layout":null}}`, rec.Body.String())

		get := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
		assert.NotContains(t, get.Body.String(), `"layout"`)

		var stored sheet.Sheet
		require.NoError(t, mdb.Collection("sheets").FindOne(ctx, bson.D{{Key: "_id", Value: sheetID}}).Decode(&stored))
		assert.Nil(t, stored.Layout)
	})
}
