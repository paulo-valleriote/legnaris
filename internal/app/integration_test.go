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
	t.Cleanup(func() { _ = database.Close(context.Background()) })

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

	modules := app.NewModules(database, registry)
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

	dexMod := modifiers.Children["dexterity_mod"]
	require.NotNil(t, dexMod.Schema)
	assert.False(t, dexMod.Schema.IsEditable)
	assert.Equal(t, "floor((stats_dexterity - 10) / 2)", dexMod.Schema.Formula)
	assert.Equal(t, [][]string{{"stats", "dexterity"}}, dexMod.Schema.DependsOn)
}

func TestIntegration_TemplatesFetchByID_NotFound(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	rec := doRequest(t, h, http.MethodGet, "/templates/unknown", "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body response.Error
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

	// Create
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
	assert.Equal(t, 10.0, derived["armor_class"])
	assert.Equal(t, 0.0, derived["stealth_total"])

	// Fetch
	fetchRec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, fetchRec.Code)

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(fetchRec.Body.Bytes(), &fetched))
	assert.Equal(t, sheetID, fetched.Data.ID)
	assert.Equal(t, "fantasy_v1", fetched.Data.TemplateID)

	// Derived fields must still be correct right after fetching
	fetchedDerived, ok := fetched.Data.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 10.0, fetchedDerived["armor_class"])

	// Patch (cascading recalc without any template query)
	patchRec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":16}}}`)
	require.Equal(t, http.StatusOK, patchRec.Code)

	var patched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))

	modifiers, ok := patched.Data["modifiers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3.0, modifiers["dexterity_mod"])

	patchedDerived, ok := patched.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 13.0, patchedDerived["armor_class"])
	assert.Equal(t, 3.0, patchedDerived["stealth_total"])
}
