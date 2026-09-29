package app_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/internal/app"
	"legnaris/internal/engine"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"
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
	t.Cleanup(func() {
		_ = mdb.Collection("sheets").Drop(context.Background())
		_ = mdb.Collection("template").Drop(context.Background())
	})

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	modules := app.NewModules(database, registry, testDatabaseName, log)
	return app.NewRouter(env, modules), mdb
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
	assert.Equal(t, "floor((stats_dexterity - 10) / 2)", dexMod.Schema.Formula)
	assert.Equal(t, [][]string{{"stats", "dexterity"}}, dexMod.Schema.DependsOn)
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
	assert.Equal(t, 0.0, derived["stealthTotal"])

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
	assert.Equal(t, 3.0, patchedDerived["stealthTotal"])
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
