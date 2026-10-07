package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/internal/app"
	"legnaris/internal/auth"
	"legnaris/internal/engine"
	"legnaris/internal/platform/metrics"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"
	"legnaris/shared/maputils"
	"legnaris/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const testDatabaseName = "legnaris"

// newIntegrationRouter boots the real HTTP stack (in-memory template registry +
// MongoDB) in-process. It skips the test when MongoDB is unavailable.
func newIntegrationRouter(t *testing.T) (http.Handler, *mongo.Database) {
	t.Helper()

	t.Setenv("JWT_SECRET", "integration-test-secret")
	t.Setenv("AUTH_BCRYPT_COST", "12")

	log := logger.NewLogger(slog.LevelError)
	env := env.NewEnv(log)

	database, err := db.InitMongo(env)
	if err != nil {
		t.Skipf("mongo unavailable — skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), env.Variables.DatabaseCloseTimeout)
		defer cancel()
		_ = database.Close(closeCtx)
	})

	ctx := context.Background()
	mdb := database.Client.Database(testDatabaseName)

	// Clean slate so assertions are deterministic. The "template" collection is
	// dropped in case a pre-migration boot created it.
	_ = mdb.Collection("template").Drop(ctx)
	_ = mdb.Collection("sheets").Drop(ctx)
	_ = mdb.Collection("users").Drop(ctx)
	_ = mdb.Collection("refresh_tokens").Drop(ctx)
	t.Cleanup(func() {
		_ = mdb.Collection("sheets").Drop(context.Background())
		_ = mdb.Collection("template").Drop(context.Background())
		_ = mdb.Collection("users").Drop(context.Background())
		_ = mdb.Collection("refresh_tokens").Drop(context.Background())
	})

	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	modules, err := app.NewModules(database, registry, env, auth.NewNopObserver())
	require.NoError(t, err)

	return app.NewRouter(env, modules, metrics.New()), mdb
}

func doRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestIntegration_TemplatesList(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	rec := doRequest(t, h, http.MethodGet, "/templates", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var body response.Success[[]template.TemplateDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)

	tmpl := body.Data[0]
	assert.Equal(t, "fantasy_v1", tmpl.ID)
	assert.Equal(t, "Fantasy Template", tmpl.Name)
	assert.True(t, tmpl.Default)

	stats, ok := tmpl.Fields["stats"]
	require.True(t, ok)
	require.NotNil(t, stats.Children)

	dex := stats.Children["dexterity"]
	require.NotNil(t, dex.Schema)
	assert.Equal(t, "number", dex.Schema.Type)
	assert.True(t, dex.Schema.IsEditable)
	assert.Equal(t, 10.0, dex.Schema.Default)
	assert.Empty(t, dex.Schema.Formula)
}

func TestIntegration_TemplatesFetchByID(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	rec := doRequest(t, h, http.MethodGet, "/templates/fantasy_v1", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var body response.Success[template.TemplateDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "fantasy_v1", body.Data.ID)

	modifiers, ok := body.Data.Fields["modifiers"]
	require.True(t, ok)
	require.NotNil(t, modifiers.Children)

	dexMod := modifiers.Children["dexterityMod"]
	require.NotNil(t, dexMod.Schema)
	assert.False(t, dexMod.Schema.IsEditable)
	assert.Equal(t, "floor((stats_dexterity - rules_modifierBase) / rules_modifierStep)", dexMod.Schema.Formula)
	assert.Equal(t, [][]string{
		{"stats", "dexterity"},
		{"rules", "modifierBase"},
		{"rules", "modifierStep"},
	}, dexMod.Schema.DependsOn)

	// The sections must arrive in the template's declared display order —
	// the sheet renders them exactly as they come.
	assert.Equal(t, []string{
		"identity", "stats", "modifiers", "saves", "skills",
		"combat", "attacks", "spells", "derived", "rules", "config",
	}, templateFieldKeys(t, rec.Body.Bytes()))
}

// templateFieldKeys walks the response envelope down to the `fields` object of
// the template payload and returns its keys in arrival order — json.Unmarshal
// would flatten the object into a map and sort away the display-order contract.
func templateFieldKeys(t *testing.T, raw []byte) []string {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(raw))
	_, err := decoder.Token()
	require.NoError(t, err, "payload must open as an object")

	for decoder.More() {
		token, err := decoder.Token()
		require.NoError(t, err)
		key, ok := token.(string)
		require.True(t, ok, "object keys must be strings")
		if key != "data" {
			var skip json.RawMessage
			require.NoError(t, decoder.Decode(&skip))
			continue
		}

		_, err = decoder.Token()
		require.NoError(t, err, "data must open as an object")
		for decoder.More() {
			token, err := decoder.Token()
			require.NoError(t, err)
			key, ok := token.(string)
			require.True(t, ok, "object keys must be strings")
			if key != "fields" {
				var skip json.RawMessage
				require.NoError(t, decoder.Decode(&skip))
				continue
			}

			_, err = decoder.Token()
			require.NoError(t, err, "fields must open as an object")
			keys := []string{}
			for decoder.More() {
				token, err := decoder.Token()
				require.NoError(t, err)
				key, ok := token.(string)
				require.True(t, ok, "field keys must be strings")
				keys = append(keys, key)
				var skip json.RawMessage
				require.NoError(t, decoder.Decode(&skip))
			}
			return keys
		}
		t.Fatal("template payload has no fields object")
	}

	t.Fatal("response has no data envelope")
	return nil
}

func TestIntegration_TemplatesFetchByID_NotFound(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	rec := doRequest(t, h, http.MethodGet, "/templates/unknown", "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
	assert.Equal(t, "template not found", body.Error.Message)
}

func TestIntegration_BootDoesNotCreateTemplateCollection(t *testing.T) {
	_, mdb := newIntegrationRouter(t)

	names, err := mdb.ListCollectionNames(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.NotContains(t, names, "template")
}

func TestIntegration_Sheets_CreateFetchPatch(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	createRec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Test Sheet"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	sheetID := created.Data.ID
	require.NotEmpty(t, sheetID)
	assert.Equal(t, "fantasy_v1", created.Data.TemplateID)
	assert.Equal(t, "Test Sheet", created.Data.Name)

	stats, ok := created.Data.Data["stats"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 10.0, stats["dexterity"])
	assert.Equal(t, 10.0, stats["constitution"])

	// Derived fields must be computed on create (not left at their 0 default)
	derived, ok := created.Data.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 10.0, derived["armorClass"])
	assert.Equal(t, 10.0, derived["passiveAwareness"])

	// Text leaves seed as an empty string, never at the numeric default.
	identity, ok := created.Data.Data["identity"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "", identity["characterName"])

	// Skill totals compute on create: default dex 10 (+0) and no proficiency.
	skillsData, ok := created.Data.Data["skills"].(map[string]any)
	require.True(t, ok)
	stealth, ok := skillsData["stealth"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0.0, stealth["total"])

	fetchRec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, fetchRec.Code)

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(fetchRec.Body.Bytes(), &fetched))
	assert.Equal(t, sheetID, fetched.Data.ID)
	assert.Equal(t, "fantasy_v1", fetched.Data.TemplateID)

	// Derived fields must still be correct right after fetching
	fetchedDerived, ok := fetched.Data.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 10.0, fetchedDerived["armorClass"])

	patchRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":16}}}`)
	require.Equal(t, http.StatusOK, patchRec.Code)

	var patched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))

	modifiers, ok := patched.Data["modifiers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3.0, modifiers["dexterityMod"])

	patchedDerived, ok := patched.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 13.0, patchedDerived["armorClass"])

	patchedSkills, ok := patched.Data["skills"].(map[string]any)
	require.True(t, ok)
	patchedStealth, ok := patchedSkills["stealth"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3.0, patchedStealth["total"])
}

// TestIntegration_Sheets_CalibrationAndTextUpdates covers the fantasy_v1
// mechanics end to end: the rules group retunes every dependent value
// through the DAG, config flags wire proficiency into a save, and a text
// field stores exactly what the client sends.
func TestIntegration_Sheets_CalibrationAndTextUpdates(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	createRec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Calibration"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID

	// Raising the modifier base to 12 turns every default score of 10 from
	// +0 into -1, cascading through saves, skills and derived values.
	rulesRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"rules":{"modifierBase":12}}}`)
	require.Equal(t, http.StatusOK, rulesRec.Code)

	var rulesPatched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(rulesRec.Body.Bytes(), &rulesPatched))

	modifiers, ok := rulesPatched.Data["modifiers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, -1.0, modifiers["dexterityMod"])

	saves, ok := rulesPatched.Data["saves"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, -1.0, saves["dexterity"])

	skillsData, ok := rulesPatched.Data["skills"].(map[string]any)
	require.True(t, ok)
	stealth, ok := skillsData["stealth"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, -1.0, stealth["total"])

	derived, ok := rulesPatched.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 9.0, derived["armorClass"])
	assert.Equal(t, 9.0, derived["passiveAwareness"])

	// Arming the dexterity save proficiency adds the +2 bonus on top.
	flagRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"saveDexterityProf":1}}}`)
	require.Equal(t, http.StatusOK, flagRec.Code)

	var flagPatched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(flagRec.Body.Bytes(), &flagPatched))
	flagSaves, ok := flagPatched.Data["saves"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1.0, flagSaves["dexterity"])

	// A text edit changes nothing derivable and persists verbatim.
	textRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"identity":{"characterName":"Aldric"}}}`)
	require.Equal(t, http.StatusOK, textRec.Code)

	fetchRec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, fetchRec.Code)

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(fetchRec.Body.Bytes(), &fetched))
	fetchedIdentity, ok := fetched.Data.Data["identity"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Aldric", fetchedIdentity["characterName"])

	fetchedRules, ok := fetched.Data.Data["rules"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 12.0, fetchedRules["modifierBase"])
}

// TestIntegration_Sheets_ExclusiveOptions covers the mutually exclusive
// armor-class pair end to end (EXC-04, EXC-05, EXC-07): a PATCH that turns one
// member on zeroes its sibling and recomputes the derived value, and a PATCH
// that turns both on is rejected with 400 INVALID_REQUEST.
func TestIntegration_Sheets_ExclusiveOptions(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	createRec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Exclusive"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID

	// Distinct modifiers make the armor-class recompute observably tied to the
	// active exclusive member.
	statsRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":14,"constitution":12}}}`)
	require.Equal(t, http.StatusOK, statsRec.Code)

	// The payload turns constitution on; normalization must zero the dexterity
	// sibling (default on) in the same request.
	conRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"acUsesCon":1}}}`)
	require.Equal(t, http.StatusOK, conRec.Code)

	var conPatched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(conRec.Body.Bytes(), &conPatched))
	conDerived, ok := conPatched.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 11.0, conDerived["armorClass"], "armor class must follow the active constitution option")

	fetchRec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, fetchRec.Code)

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(fetchRec.Body.Bytes(), &fetched))
	config, ok := fetched.Data.Data["config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0.0, config["acUsesDex"], "the dexterity sibling must be stored as zero")
	assert.Equal(t, 1.0, config["acUsesCon"])

	// Turning both members on in one payload is ambiguous intent: reject it.
	bothRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"acUsesDex":1,"acUsesCon":1}}}`)
	require.Equal(t, http.StatusBadRequest, bothRec.Code)

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(bothRec.Body.Bytes(), &body))
	assert.Equal(t, "INVALID_REQUEST", body.Error.Code)

	// A non-numeric member is not a 0/1 proxy: reject it instead of treating it
	// as "off" and letting the payload bypass the at-most-one rule.
	nonNumericRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"acUsesDex":"1"}}}`)
	require.Equal(t, http.StatusBadRequest, nonNumericRec.Code)

	var nonNumericBody response.ErrorResponse
	require.NoError(t, json.Unmarshal(nonNumericRec.Body.Bytes(), &nonNumericBody))
	assert.Equal(t, "INVALID_REQUEST", nonNumericBody.Error.Code)
}

// TestIntegration_Sheets_NestingDepthLimit covers the DoS guard end to end: a
// PATCH whose data nests deeper than maputils.MaxNestingDepth is rejected with
// 400 INVALID_REQUEST instead of driving the recursive flatten off a cliff.
func TestIntegration_Sheets_NestingDepthLimit(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	createRec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Deep"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID

	deep := any(1.0)
	for i := 0; i < maputils.MaxNestingDepth+1; i++ {
		deep = map[string]any{"level": deep}
	}
	deepBodyBytes, err := json.Marshal(map[string]any{"data": deep})
	require.NoError(t, err)

	deepRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, string(deepBodyBytes))
	require.Equal(t, http.StatusBadRequest, deepRec.Code)

	var deepBody response.ErrorResponse
	require.NoError(t, json.Unmarshal(deepRec.Body.Bytes(), &deepBody))
	assert.Equal(t, "INVALID_REQUEST", deepBody.Error.Code)
}

// TestIntegration_Sheets_Preview covers the dry-run route end to end: a valid
// delta returns the same cascade an equivalent PATCH returns, and the preview
// leaves the stored sheet untouched (no persistence).
func TestIntegration_Sheets_Preview(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	createRec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Preview"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID

	previewRec := doRequest(t, h, http.MethodPost, "/sheets/"+sheetID+"/preview", `{"data":{"stats":{"dexterity":16}}}`)
	require.Equal(t, http.StatusOK, previewRec.Code)

	var previewed response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(previewRec.Body.Bytes(), &previewed))

	previewModifiers, ok := previewed.Data["modifiers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3.0, previewModifiers["dexterityMod"])
	previewDerived, ok := previewed.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 13.0, previewDerived["armorClass"])

	// Preview must not persist: the stored sheet still holds the original value.
	fetchedRec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, fetchedRec.Code)

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(fetchedRec.Body.Bytes(), &fetched))
	fetchedStats, ok := fetched.Data.Data["stats"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 10.0, fetchedStats["dexterity"])
	fetchedDerived, ok := fetched.Data.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 10.0, fetchedDerived["armorClass"])

	// The equivalent PATCH returns the same cascade, proving the shared path.
	patchRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":16}}}`)
	require.Equal(t, http.StatusOK, patchRec.Code)

	var patched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))
	assert.Equal(t, patched.Data, previewed.Data)
}

// TestIntegration_Templates_Preview covers the template dry-run route end to
// end over the real registry: a valid delta returns the cascade seeded from the
// template, a malformed id is 400 and an unknown slug is 404.
func TestIntegration_Templates_Preview(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	previewRec := doRequest(t, h, http.MethodPost, "/templates/fantasy_v1/preview", `{"data":{"stats":{"dexterity":16}}}`)
	require.Equal(t, http.StatusOK, previewRec.Code)

	var previewed response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(previewRec.Body.Bytes(), &previewed))

	modifiers, ok := previewed.Data["modifiers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3.0, modifiers["dexterityMod"])
	derived, ok := previewed.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 13.0, derived["armorClass"])

	malformedRec := doRequest(t, h, http.MethodPost, "/templates/Fantasy-V1/preview", `{"data":{"stats":{"dexterity":12}}}`)
	require.Equal(t, http.StatusBadRequest, malformedRec.Code)

	var malformedBody response.ErrorResponse
	require.NoError(t, json.Unmarshal(malformedRec.Body.Bytes(), &malformedBody))
	assert.Equal(t, "INVALID_REQUEST", malformedBody.Error.Code)

	unknownRec := doRequest(t, h, http.MethodPost, "/templates/unknown_template/preview", `{"data":{"stats":{"dexterity":12}}}`)
	require.Equal(t, http.StatusNotFound, unknownRec.Code)

	var unknownBody response.ErrorResponse
	require.NoError(t, json.Unmarshal(unknownRec.Body.Bytes(), &unknownBody))
	assert.Equal(t, "NOT_FOUND", unknownBody.Error.Code)
}

// TestIntegration_ErrorStatusMapping covers the HTTP status each typed error
// produces through HTTPErrorHandler (LGN-48): domain errors must never fall
// through as 500 INTERNAL_ERROR.
func TestIntegration_ErrorStatusMapping(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	assertErrorBody := func(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		require.Equal(t, status, rec.Code)
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, code, body.Error.Code)
		assert.NotEmpty(t, body.Error.Message)
	}

	t.Run("GET /sheets/:id missing returns 404", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodGet, "/sheets/"+missingSheetID, "")
		assertErrorBody(t, rec, http.StatusNotFound, "NOT_FOUND")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "sheet not found", body.Error.Message)
	})

	t.Run("GET with malformed sheet ID returns 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodGet, "/sheets/not-a-uuid", "")
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, sheet.MsgSheetIDInvalid, body.Error.Message)
	})

	t.Run("GET /templates/:id with malformed ID returns 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodGet, "/templates/Fantasy-V1", "")
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "template id is not a valid slug", body.Error.Message)
	})

	t.Run("POST /sheets with malformed template_id returns 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"Fantasy-V1","sheet_name":"x"}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "template id is not a valid slug", body.Error.Message)
	})

	t.Run("POST /sheets with unknown template returns 404", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"unknown_template","sheet_name":"x"}`)
		assertErrorBody(t, rec, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("PATCH /sheets/:id missing returns 404", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPatch, "/sheets/"+missingSheetID, `{"data":{"stats":{"dexterity":12}}}`)
		assertErrorBody(t, rec, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("PATCH with unknown field returns 400 UNKNOWN_FIELD", func(t *testing.T) {
		createRec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Err Map"}`)
		require.Equal(t, http.StatusCreated, createRec.Code)
		var created response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

		rec := doRequest(t, h, http.MethodPatch, "/sheets/"+created.Data.ID, `{"data":{"stats":{"not_exists":1}}}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "UNKNOWN_FIELD")
	})

	t.Run("PATCH on non-editable field returns 400 INVALID_REQUEST", func(t *testing.T) {
		createRec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Err Map 2"}`)
		require.Equal(t, http.StatusCreated, createRec.Code)
		var created response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

		rec := doRequest(t, h, http.MethodPatch, "/sheets/"+created.Data.ID, `{"data":{"modifiers":{"dexterityMod":99}}}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("POST with invalid JSON body returns 400 INVALID_JSON", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_JSON")
	})

	t.Run("POST with unknown field in body returns 400 INVALID_JSON", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"x","bogus":1}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_JSON")
	})

	t.Run("POST with trailing JSON returns 400 UNKNOWN_FIELD", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"x"} {}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "UNKNOWN_FIELD")
	})
}

// TestIntegration_MetricsRecordsErrorStatus pins the metric status label for
// erroring routes end-to-end. The RequestLogger (HandleError=true) commits the
// mapped 404 before the metrics middleware resumes, so /metrics must show
// status="404" (not 200 or a bare 500) for GET /sheets/:id that returns
// ErrSheetNotFound. This guards the middleware ordering the metrics status
// label depends on.
func TestIntegration_MetricsRecordsErrorStatus(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	rec := doRequest(t, h, http.MethodGet, "/sheets/"+missingSheetID, "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	mrec := doRequest(t, h, http.MethodGet, "/metrics", "")
	require.Equal(t, http.StatusOK, mrec.Code)

	assert.Contains(
		t,
		mrec.Body.String(),
		`legnaris_http_requests_total{method="GET",path="/sheets/:id",status="404"}`,
	)
}
