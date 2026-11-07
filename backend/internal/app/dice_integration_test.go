package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestIntegration_Sheets_Dice(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	ctx := context.Background()

	token, _ := newAuthUser(t, h, "dice-owner@example.com", "dice-owner")
	otherToken, _ := newAuthUser(t, h, "dice-other@example.com", "dice-other")

	createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Dice"}`)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID
	path := "/sheets/" + sheetID + "/dice"

	fetch := func(t *testing.T) response.Success[sheet.SheetDTO] {
		t.Helper()
		rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
		require.Equal(t, http.StatusOK, rec.Code)
		var got response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		return got
	}
	emptyBlock := sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}}
	errorCode := func(t *testing.T, rec interface{ Bytes() []byte }) string {
		t.Helper()
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Bytes(), &body))
		return body.Error.Code
	}

	body := `{"dice":{"section":{"skills":"1d20 + @.total","saves":null},` +
		`"row":{"stats.dexterity":"2d6 + modifiers.dexterityMod","combat.initiative":null}}}`
	want := sheet.Dice{
		Section: map[string]*string{"skills": diceText("1d20 + @.total"), "saves": nil},
		Row:     map[string]*string{"stats.dexterity": diceText("2d6 + modifiers.dexterityMod"), "combat.initiative": nil},
	}

	t.Run("Edge: a fresh sheet returns an empty dice block", func(t *testing.T) {
		assert.Equal(t, emptyBlock, fetch(t).Data.Dice)
		rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
		assert.Contains(t, rec.Body.String(), `"dice":{"section":{},"row":{}}`)
	})

	t.Run("Error: no token is 401", func(t *testing.T) {
		rec := doAuthRequest(t, h, "", http.MethodPut, path, body)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Error: another user gets the same 404 and nothing is stored", func(t *testing.T) {
		rec := doAuthRequest(t, h, otherToken, http.MethodPut, path, body)
		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "NOT_FOUND", errorCode(t, rec.Body))
		assert.Equal(t, emptyBlock, fetch(t).Data.Dice)
	})

	t.Run("Error: a nonexistent sheet is 404", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, "/sheets/"+missingSheetID+"/dice", body)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Error: a malformed sheet id is 400", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, "/sheets/not-a-uuid/dice", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	invalid := []struct {
		name string
		body string
		code string
	}{
		{"an invalid expression", `{"dice":{"row":{"stats.dexterity":"d20 +"}}}`, "INVALID_REQUEST"},
		{"an empty expression", `{"dice":{"row":{"stats.dexterity":""}}}`, "INVALID_REQUEST"},
		{"an unknown section", `{"dice":{"section":{"ghost":null}}}`, "INVALID_REQUEST"},
		{"an unknown row", `{"dice":{"row":{"stats.ghost":null}}}`, "INVALID_REQUEST"},
		{"a row override that does not fit", `{"dice":{"row":{"identity.characterName":"1d20 + @"}}}`, "INVALID_REQUEST"},
		{"a section override that fits no row", `{"dice":{"section":{"stats":"1d20 + @.total"}}}`, "INVALID_REQUEST"},
		{"malformed JSON", `{"dice":`, "INVALID_JSON"},
		{"a number as expression", `{"dice":{"row":{"stats.dexterity":20}}}`, "INVALID_JSON"},
	}
	for _, tc := range invalid {
		t.Run("Error: "+tc.name+" is 400 and nothing is stored", func(t *testing.T) {
			rec := doAuthRequest(t, h, token, http.MethodPut, path, tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Equal(t, tc.code, errorCode(t, rec.Body))
			assert.Equal(t, emptyBlock, fetch(t).Data.Dice)
		})
	}

	t.Run("Happy path: PUT stores the block, null included, and GET returns it", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var put response.Success[struct {
			Dice sheet.Dice `json:"dice"`
		}]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &put))
		assert.Equal(t, want, put.Data.Dice)

		assert.Equal(t, want, fetch(t).Data.Dice)
	})

	t.Run("Edge: re-putting the same block is idempotent", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, body)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, want, fetch(t).Data.Dice)
	})

	t.Run("Edge: a PUT is a full replacement, omitted keys are removed", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPut, path, `{"dice":{"row":{"stats.strength":"1d20"}}}`)
		require.Equal(t, http.StatusOK, rec.Code)

		assert.Equal(t, sheet.Dice{
			Section: map[string]*string{},
			Row:     map[string]*string{"stats.strength": diceText("1d20")},
		}, fetch(t).Data.Dice)
	})

	t.Run("Edge: PUT keeps data, layout and updated_at, and a PATCH of data keeps the dice", func(t *testing.T) {
		require.Equal(t, http.StatusOK, doAuthRequest(t, h, token, http.MethodPut, "/sheets/"+sheetID+"/layout", `{"layout":{"pinned":["stats.dexterity"]}}`).Code)
		require.Equal(t, http.StatusOK, doAuthRequest(t, h, token, http.MethodPut, path, body).Code)
		before := fetch(t)

		patch := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":16}}}`)
		require.Equal(t, http.StatusOK, patch.Code)
		afterPatch := fetch(t)
		assert.Equal(t, want, afterPatch.Data.Dice)

		require.Equal(t, http.StatusOK, doAuthRequest(t, h, token, http.MethodPut, path, body).Code)
		afterPut := fetch(t)
		assert.Equal(t, afterPatch.Data.Data, afterPut.Data.Data, "a dice write must not change values or derived fields")
		assert.Equal(t, afterPatch.Data.UpdatedAt, afterPut.Data.UpdatedAt)
		assert.Equal(t, before.Data.Layout, afterPut.Data.Layout)
	})

	t.Run("Edge: a stale override for a row the template no longer has is returned on GET, not an error", func(t *testing.T) {
		_, err := mdb.Collection("sheets").UpdateOne(ctx,
			bson.D{{Key: "_id", Value: sheetID}},
			bson.M{"$set": bson.M{"dice": bson.M{
				"section": bson.M{"retired": "1d20"},
				"row":     bson.M{"stats.retired": "1d20 + stats.gone"},
			}}})
		require.NoError(t, err)

		rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"retired":"1d20"`)
	})

	t.Run("Edge: a sheet stored before the feature returns an empty block", func(t *testing.T) {
		_, err := mdb.Collection("sheets").UpdateOne(ctx,
			bson.D{{Key: "_id", Value: sheetID}},
			bson.M{"$unset": bson.M{"dice": ""}})
		require.NoError(t, err)

		assert.Equal(t, emptyBlock, fetch(t).Data.Dice)
	})

	t.Run("Edge: concurrent PUTs leave one whole valid block", func(t *testing.T) {
		bodies := []string{body, `{"dice":{"row":{"stats.strength":"1d20"}}}`}
		candidates := []sheet.Dice{
			want,
			{Section: map[string]*string{}, Row: map[string]*string{"stats.strength": diceText("1d20")}},
		}

		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rec := doAuthRequest(t, h, token, http.MethodPut, path, bodies[i%2])
				assert.Equal(t, http.StatusOK, rec.Code)
			}()
		}
		wg.Wait()

		assert.Contains(t, candidates, fetch(t).Data.Dice)
	})

	t.Run("Edge: an empty block clears everything and the stored field reads back as none", func(t *testing.T) {
		require.Equal(t, http.StatusOK, doAuthRequest(t, h, token, http.MethodPut, path, body).Code)

		rec := doAuthRequest(t, h, token, http.MethodPut, path, `{"dice":{}}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"data":{"dice":{"section":{},"row":{}}}}`, rec.Body.String())
		assert.Equal(t, emptyBlock, fetch(t).Data.Dice)

		var stored sheet.Sheet
		require.NoError(t, mdb.Collection("sheets").FindOne(ctx, bson.D{{Key: "_id", Value: sheetID}}).Decode(&stored))
		assert.Nil(t, stored.Dice)
	})

	t.Run("Edge: 64 entries are accepted and 65 are refused", func(t *testing.T) {
		rec64 := doAuthRequest(t, h, token, http.MethodPut, path, nullDiceBody(t, 64))
		assert.Equal(t, http.StatusOK, rec64.Code, rec64.Body.String())

		rec65 := doAuthRequest(t, h, token, http.MethodPut, path, nullDiceBody(t, 65))
		require.Equal(t, http.StatusBadRequest, rec65.Code)
		assert.Equal(t, "INVALID_REQUEST", errorCode(t, rec65.Body))
	})
}

// nullDiceBody builds a PUT body of n "no dice" entries over real fantasy_v1
// keys: every Section first, then Rows, so the keys are valid and only the cap
// can refuse it.
func nullDiceBody(t *testing.T, n int) string {
	t.Helper()

	var fantasy template.Template
	for _, candidate := range template.DefaultTemplates() {
		if candidate.ID == "fantasy_v1" {
			fantasy = candidate
		}
	}
	require.NotEmpty(t, fantasy.ID)

	block := sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}}
	var rows []string
	for _, section := range template.SectionRowKeys(fantasy) {
		block.Section[section.Section] = nil
		rows = append(rows, section.Rows...)
	}
	for _, row := range rows {
		if len(block.Section)+len(block.Row) == n {
			break
		}
		block.Row[row] = nil
	}
	require.Equal(t, n, len(block.Section)+len(block.Row), "fantasy_v1 must have enough Rows for the cap test")

	raw, err := json.Marshal(sheet.SetDiceCommand{Dice: &block})
	require.NoError(t, err)
	return string(raw)
}
