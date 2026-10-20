package app_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
	"legnaris/backend/shared/maputils"
	"legnaris/backend/shared/response"

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

	handler, mdb, _ := newIntegrationStack(t, nil)
	return handler, mdb
}

// newIntegrationStack is newIntegrationRouter with the wired modules exposed
// and optional environment overrides, so a suite can drive the service directly
// and shrink a configured bound without touching the production defaults.
func newIntegrationStack(t *testing.T, extra map[string]string) (http.Handler, *mongo.Database, *app.Modules) {
	t.Helper()

	t.Setenv("JWT_SECRET", "integration-test-secret")
	t.Setenv("AUTH_BCRYPT_COST", "12")
	t.Setenv("EMAIL_CODE_SECRET", "integration-test-code-secret-0123456789")
	for key, value := range extra {
		t.Setenv(key, value)
	}

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
	_ = mdb.Collection("email_outbox").Drop(ctx)
	_ = mdb.Collection("auth_challenges").Drop(ctx)
	t.Cleanup(func() {
		_ = mdb.Collection("sheets").Drop(context.Background())
		_ = mdb.Collection("template").Drop(context.Background())
		_ = mdb.Collection("users").Drop(context.Background())
		_ = mdb.Collection("refresh_tokens").Drop(context.Background())
		_ = mdb.Collection("email_outbox").Drop(context.Background())
		_ = mdb.Collection("auth_challenges").Drop(context.Background())
	})

	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	modules, err := app.NewModules(database, registry, env, auth.NewNopObserver(), auth.NewNopChallengeObserver())
	require.NoError(t, err)

	return app.NewRouter(env, modules, metrics.New()), mdb, modules
}

func doRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doAuthRequest(t, h, "", method, path, body)
}

// integrationPassword is the shared password every integration user registers
// with; it clears the service's length checks.
const integrationPassword = "sup3r-secret"

// newAuthUser registers a fresh user and returns the session's access token plus
// the user id parsed from its subject. Registration signs the browser in, so no
// separate login is needed (AUTH-02).
func newAuthUser(t *testing.T, h http.Handler, email, username string) (token string, userID string) {
	t.Helper()

	registerBody := fmt.Sprintf(`{"email":%q,"username":%q,"password":%q}`, email, username, integrationPassword)
	registerRec := doRequest(t, h, http.MethodPost, "/auth/register", registerBody)
	require.Equal(t, http.StatusCreated, registerRec.Code, registerRec.Body.String())

	var session response.Success[auth.SessionResponse]
	require.NoError(t, json.Unmarshal(registerRec.Body.Bytes(), &session))
	require.NotEmpty(t, session.Data.AccessToken)

	return session.Data.AccessToken, jwtSubject(t, session.Data.AccessToken)
}

// jwtSubject decodes the `sub` claim from an access token's payload without
// re-verifying the signature; the server just issued the token.
func jwtSubject(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "a JWT has three segments")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var claims struct {
		Subject string `json:"sub"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	require.NotEmpty(t, claims.Subject)
	return claims.Subject
}

// doAuthRequest issues a request with the bearer token attached (empty token
// leaves the header off, for the unauthenticated cases).
func doAuthRequest(t *testing.T, h http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestIntegration_ProtectedRoutesRequireJWT covers OWN-15…OWN-18: every /sheets*
// and /templates* route rejects a tokenless request with 401 UNAUTHORIZED and
// never reaches a handler, while health, metrics, and the auth endpoints stay
// public and a valid token reaches the normal handler.
func TestIntegration_ProtectedRoutesRequireJWT(t *testing.T) {
	h, mdb := newIntegrationRouter(t)

	protected := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"x"}`},
		{http.MethodGet, "/sheets", ""},
		{http.MethodGet, "/sheets/" + missingSheetID, ""},
		{http.MethodPatch, "/sheets/" + missingSheetID, `{"data":{"stats":{"dexterity":12}}}`},
		{http.MethodDelete, "/sheets/" + missingSheetID, ""},
		{http.MethodPost, "/sheets/" + missingSheetID + "/preview", `{"data":{"stats":{"dexterity":12}}}`},
		{http.MethodGet, "/templates", ""},
		{http.MethodGet, "/templates/fantasy_v1", ""},
		{http.MethodPost, "/templates/fantasy_v1/preview", `{"data":{"stats":{"dexterity":12}}}`},
	}

	for _, tc := range protected {
		rec := doAuthRequest(t, h, "", tc.method, tc.path, tc.body)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s must be 401 without a token", tc.method, tc.path)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
	}

	// The unauthorized calls must not have written anything.
	count, err := mdb.Collection("sheets").CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.Zero(t, count, "no sheet may be created without a token")

	t.Run("Edge: public routes stay reachable without a token", func(t *testing.T) {
		health := doRequest(t, h, http.MethodGet, "/health", "")
		require.True(t, health.Code == http.StatusOK || health.Code == http.StatusNoContent, "health must stay public")

		mrec := doRequest(t, h, http.MethodGet, "/metrics", "")
		require.Equal(t, http.StatusOK, mrec.Code)

		// register/login stay public: a fresh user can sign in with no token.
		token, _ := newAuthUser(t, h, "public@example.com", "public")
		require.NotEmpty(t, token)
	})

	t.Run("Edge: an invalid token is 401", func(t *testing.T) {
		rec := doAuthRequest(t, h, "not-a-jwt", http.MethodGet, "/sheets", "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
	})

	t.Run("Happy path: a valid token reaches the normal handler", func(t *testing.T) {
		token, _ := newAuthUser(t, h, "valid@example.com", "valid")
		rec := doAuthRequest(t, h, token, http.MethodGet, "/templates", "")
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestIntegration_TemplatesList(t *testing.T) {
	h, _ := newIntegrationRouter(t)
	token, _ := newAuthUser(t, h, "templates-list@example.com", "templates-list")

	rec := doAuthRequest(t, h, token, http.MethodGet, "/templates", "")
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
	token, _ := newAuthUser(t, h, "templates-fetch@example.com", "templates-fetch")

	rec := doAuthRequest(t, h, token, http.MethodGet, "/templates/fantasy_v1", "")
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
	token, _ := newAuthUser(t, h, "templates-404@example.com", "templates-404")

	rec := doAuthRequest(t, h, token, http.MethodGet, "/templates/unknown", "")
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
	token, _ := newAuthUser(t, h, "sheets-cfp@example.com", "sheets-cfp")

	createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Test Sheet"}`)
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

	fetchRec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, fetchRec.Code)

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(fetchRec.Body.Bytes(), &fetched))
	assert.Equal(t, sheetID, fetched.Data.ID)
	assert.Equal(t, "fantasy_v1", fetched.Data.TemplateID)

	// Derived fields must still be correct right after fetching
	fetchedDerived, ok := fetched.Data.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 10.0, fetchedDerived["armorClass"])

	patchRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":16}}}`)
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
	token, _ := newAuthUser(t, h, "sheets-calib@example.com", "sheets-calib")

	createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Calibration"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID

	// Raising the modifier base to 12 turns every default score of 10 from
	// +0 into -1, cascading through saves, skills and derived values.
	rulesRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"rules":{"modifierBase":12}}}`)
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
	flagRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"saveDexterityProf":1}}}`)
	require.Equal(t, http.StatusOK, flagRec.Code)

	var flagPatched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(flagRec.Body.Bytes(), &flagPatched))
	flagSaves, ok := flagPatched.Data["saves"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1.0, flagSaves["dexterity"])

	// A text edit changes nothing derivable and persists verbatim.
	textRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"identity":{"characterName":"Aldric"}}}`)
	require.Equal(t, http.StatusOK, textRec.Code)

	fetchRec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
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
	token, _ := newAuthUser(t, h, "sheets-exclusive@example.com", "sheets-exclusive")

	createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Exclusive"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID

	// Distinct modifiers make the armor-class recompute observably tied to the
	// active exclusive member.
	statsRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":14,"constitution":12}}}`)
	require.Equal(t, http.StatusOK, statsRec.Code)

	// The payload turns constitution on; normalization must zero the dexterity
	// sibling (default on) in the same request.
	conRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"acUsesCon":1}}}`)
	require.Equal(t, http.StatusOK, conRec.Code)

	var conPatched response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(conRec.Body.Bytes(), &conPatched))
	conDerived, ok := conPatched.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 11.0, conDerived["armorClass"], "armor class must follow the active constitution option")

	fetchRec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
	require.Equal(t, http.StatusOK, fetchRec.Code)

	var fetched response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(fetchRec.Body.Bytes(), &fetched))
	config, ok := fetched.Data.Data["config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0.0, config["acUsesDex"], "the dexterity sibling must be stored as zero")
	assert.Equal(t, 1.0, config["acUsesCon"])

	// Turning both members on in one payload is ambiguous intent: reject it.
	bothRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"acUsesDex":1,"acUsesCon":1}}}`)
	require.Equal(t, http.StatusBadRequest, bothRec.Code)

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(bothRec.Body.Bytes(), &body))
	assert.Equal(t, "INVALID_REQUEST", body.Error.Code)

	// A non-numeric member is not a 0/1 proxy: reject it instead of treating it
	// as "off" and letting the payload bypass the at-most-one rule.
	nonNumericRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"config":{"acUsesDex":"1"}}}`)
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
	token, _ := newAuthUser(t, h, "sheets-depth@example.com", "sheets-depth")

	createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Deep"}`)
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

	deepRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, string(deepBodyBytes))
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
	token, _ := newAuthUser(t, h, "sheets-preview@example.com", "sheets-preview")

	createRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Preview"}`)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID

	previewRec := doAuthRequest(t, h, token, http.MethodPost, "/sheets/"+sheetID+"/preview", `{"data":{"stats":{"dexterity":16}}}`)
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
	fetchedRec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+sheetID, "")
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
	patchRec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+sheetID, `{"data":{"stats":{"dexterity":16}}}`)
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
	token, _ := newAuthUser(t, h, "templates-preview@example.com", "templates-preview")

	previewRec := doAuthRequest(t, h, token, http.MethodPost, "/templates/fantasy_v1/preview", `{"data":{"stats":{"dexterity":16}}}`)
	require.Equal(t, http.StatusOK, previewRec.Code)

	var previewed response.Success[map[string]any]
	require.NoError(t, json.Unmarshal(previewRec.Body.Bytes(), &previewed))

	modifiers, ok := previewed.Data["modifiers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3.0, modifiers["dexterityMod"])
	derived, ok := previewed.Data["derived"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 13.0, derived["armorClass"])

	malformedRec := doAuthRequest(t, h, token, http.MethodPost, "/templates/Fantasy-V1/preview", `{"data":{"stats":{"dexterity":12}}}`)
	require.Equal(t, http.StatusBadRequest, malformedRec.Code)

	var malformedBody response.ErrorResponse
	require.NoError(t, json.Unmarshal(malformedRec.Body.Bytes(), &malformedBody))
	assert.Equal(t, "INVALID_REQUEST", malformedBody.Error.Code)

	unknownRec := doAuthRequest(t, h, token, http.MethodPost, "/templates/unknown_template/preview", `{"data":{"stats":{"dexterity":12}}}`)
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

	// A token for the read-only subtests; the create subtests mint their own so
	// the one-sheet rule never turns a first create into a 409.
	token, _ := newAuthUser(t, h, "err-map@example.com", "err-map")

	assertErrorBody := func(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		require.Equal(t, status, rec.Code)
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, code, body.Error.Code)
		assert.NotEmpty(t, body.Error.Message)
	}

	t.Run("GET /sheets/:id missing returns 404", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+missingSheetID, "")
		assertErrorBody(t, rec, http.StatusNotFound, "NOT_FOUND")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "sheet not found", body.Error.Message)
	})

	t.Run("GET with malformed sheet ID returns 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/not-a-uuid", "")
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, sheet.MsgSheetIDInvalid, body.Error.Message)
	})

	t.Run("GET /templates/:id with malformed ID returns 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodGet, "/templates/Fantasy-V1", "")
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "template id is not a valid slug", body.Error.Message)
	})

	t.Run("POST /sheets with malformed template_id returns 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"Fantasy-V1","sheet_name":"x"}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "template id is not a valid slug", body.Error.Message)
	})

	t.Run("POST /sheets with unknown template returns 404", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"unknown_template","sheet_name":"x"}`)
		assertErrorBody(t, rec, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("PATCH /sheets/:id missing returns 404", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPatch, "/sheets/"+missingSheetID, `{"data":{"stats":{"dexterity":12}}}`)
		assertErrorBody(t, rec, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("PATCH with unknown field returns 400 UNKNOWN_FIELD", func(t *testing.T) {
		ownerToken, _ := newAuthUser(t, h, "err-map-unknown@example.com", "err-map-unknown")
		createRec := doAuthRequest(t, h, ownerToken, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Err Map"}`)
		require.Equal(t, http.StatusCreated, createRec.Code)
		var created response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

		rec := doAuthRequest(t, h, ownerToken, http.MethodPatch, "/sheets/"+created.Data.ID, `{"data":{"stats":{"not_exists":1}}}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "UNKNOWN_FIELD")
	})

	t.Run("PATCH on non-editable field returns 400 INVALID_REQUEST", func(t *testing.T) {
		ownerToken, _ := newAuthUser(t, h, "err-map-nonedit@example.com", "err-map-nonedit")
		createRec := doAuthRequest(t, h, ownerToken, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Err Map 2"}`)
		require.Equal(t, http.StatusCreated, createRec.Code)
		var created response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

		rec := doAuthRequest(t, h, ownerToken, http.MethodPatch, "/sheets/"+created.Data.ID, `{"data":{"modifiers":{"dexterityMod":99}}}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("POST with invalid JSON body returns 400 INVALID_JSON", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_JSON")
	})

	t.Run("POST with unknown field in body returns 400 INVALID_JSON", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"x","bogus":1}`)
		assertErrorBody(t, rec, http.StatusBadRequest, "INVALID_JSON")
	})

	t.Run("POST with trailing JSON returns 400 UNKNOWN_FIELD", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"x"} {}`)
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
	token, _ := newAuthUser(t, h, "metrics-status@example.com", "metrics-status")

	rec := doAuthRequest(t, h, token, http.MethodGet, "/sheets/"+missingSheetID, "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	mrec := doRequest(t, h, http.MethodGet, "/metrics", "")
	require.Equal(t, http.StatusOK, mrec.Code)

	assert.Contains(
		t,
		mrec.Body.String(),
		`legnaris_http_requests_total{method="GET",path="/sheets/:id",status="404"}`,
	)
}

// TestIntegration_Sheets_Ownership covers the owner-scoped surface end to end
// (OWN-05…OWN-14, OWN-23): creation binds the sheet to its owner, GET /sheets
// returns only the caller's sheet (404 when none), a second create is 409, and
// every operation on another user's sheet is the same 404 as a missing one and
// mutates nothing, while the owner's delete returns 204 and flips to 404.
func TestIntegration_Sheets_Ownership(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	ctx := context.Background()

	tokenA, userA := newAuthUser(t, h, "owner-a@example.com", "owner-a")
	tokenB, userB := newAuthUser(t, h, "owner-b@example.com", "owner-b")
	require.NotEqual(t, userA, userB)

	// A creates their one sheet.
	createRec := doAuthRequest(t, h, tokenA, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"A sheet"}`)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	sheetID := created.Data.ID
	require.NotEmpty(t, sheetID)

	// The owner id is never exposed on the wire.
	assert.NotContains(t, createRec.Body.String(), "owner_id")
	assert.NotContains(t, createRec.Body.String(), userA)

	// Baseline the stored data so foreign mutation can be detected.
	var storedBefore sheet.Sheet
	require.NoError(t, mdb.Collection("sheets").
		FindOne(ctx, bson.D{{Key: "_id", Value: sheetID}}).
		Decode(&storedBefore))

	t.Run("Happy path: GET /sheets returns the owner's sheet and 404 for another user", func(t *testing.T) {
		mineRec := doAuthRequest(t, h, tokenA, http.MethodGet, "/sheets", "")
		require.Equal(t, http.StatusOK, mineRec.Code)

		var mine response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(mineRec.Body.Bytes(), &mine))
		assert.Equal(t, sheetID, mine.Data.ID)
		assert.Equal(t, "A sheet", mine.Data.Name)

		noneRec := doAuthRequest(t, h, tokenB, http.MethodGet, "/sheets", "")
		require.Equal(t, http.StatusNotFound, noneRec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(noneRec.Body.Bytes(), &body))
		assert.Equal(t, "NOT_FOUND", body.Error.Code)
	})

	t.Run("Error: a second create for the same owner is 409 SHEET_LIMIT_REACHED", func(t *testing.T) {
		rec := doAuthRequest(t, h, tokenA, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"Second"}`)
		require.Equal(t, http.StatusConflict, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "SHEET_LIMIT_REACHED", body.Error.Code)

		count, err := mdb.Collection("sheets").CountDocuments(ctx, bson.D{{Key: "owner_id", Value: userA}})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "the limit must leave exactly one sheet for the owner")
	})

	t.Run("Error: B's operations on A's sheet are 404 and mutate nothing", func(t *testing.T) {
		ops := []struct {
			method string
			path   string
			body   string
		}{
			{http.MethodGet, "/sheets/" + sheetID, ""},
			{http.MethodPatch, "/sheets/" + sheetID, `{"data":{"identity":{"characterName":"Hijacked"}}}`},
			{http.MethodDelete, "/sheets/" + sheetID, ""},
			{http.MethodPost, "/sheets/" + sheetID + "/preview", `{"data":{"stats":{"dexterity":20}}}`},
		}

		for _, op := range ops {
			rec := doAuthRequest(t, h, tokenB, op.method, op.path, op.body)
			require.Equal(t, http.StatusNotFound, rec.Code, "%s %s must be 404 for a foreign owner", op.method, op.path)

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "NOT_FOUND", body.Error.Code)
		}

		var storedAfter sheet.Sheet
		require.NoError(t, mdb.Collection("sheets").
			FindOne(ctx, bson.D{{Key: "_id", Value: sheetID}}).
			Decode(&storedAfter))
		assert.Equal(t, storedBefore.Data, storedAfter.Data, "a foreign request must not change the stored data")
	})

	t.Run("Happy path: the owner's delete returns 204 then every read is 404", func(t *testing.T) {
		rec := doAuthRequest(t, h, tokenA, http.MethodDelete, "/sheets/"+sheetID, "")
		require.Equal(t, http.StatusNoContent, rec.Code)

		getRec := doAuthRequest(t, h, tokenA, http.MethodGet, "/sheets/"+sheetID, "")
		require.Equal(t, http.StatusNotFound, getRec.Code)

		mineRec := doAuthRequest(t, h, tokenA, http.MethodGet, "/sheets", "")
		require.Equal(t, http.StatusNotFound, mineRec.Code)

		secondDelete := doAuthRequest(t, h, tokenA, http.MethodDelete, "/sheets/"+sheetID, "")
		require.Equal(t, http.StatusNotFound, secondDelete.Code)
	})
}

// TestIntegration_CanonicalEmail covers the mailbox-identity journey end to end
// (PRQ-08, PRQ-10, PRQ-15, SC-004, SC-005): a display-name registration stores
// the bare lowercase address with no verification timestamp, another spelling of
// the same mailbox is a 409 with one row, sign-in accepts a third spelling for
// the same account, and the boot path leaves a legacy document untouched.
func TestIntegration_CanonicalEmail(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	ctx := context.Background()

	const canonical = "player@example.com"
	registerBody := fmt.Sprintf(
		`{"email":%q,"username":"player","password":%q}`,
		"  Player One <Player@Example.COM>  ",
		integrationPassword,
	)

	registerRec := doRequest(t, h, http.MethodPost, "/auth/register", registerBody)
	require.Equal(t, http.StatusCreated, registerRec.Code, registerRec.Body.String())

	var session response.Success[auth.SessionResponse]
	require.NoError(t, json.Unmarshal(registerRec.Body.Bytes(), &session))
	registeredID := jwtSubject(t, session.Data.AccessToken)

	var stored bson.M
	require.NoError(t, mdb.Collection("users").FindOne(ctx, bson.D{{Key: "email", Value: canonical}}).Decode(&stored))
	assert.Equal(t, registeredID, stored["_id"])
	assert.NotContains(t, stored, "email_verified_at", "registration must not backfill a verification timestamp")

	duplicateRec := doRequest(
		t,
		h,
		http.MethodPost,
		"/auth/register",
		fmt.Sprintf(`{"email":%q,"username":"other","password":%q}`, "PLAYER@example.com", integrationPassword),
	)
	require.Equal(t, http.StatusConflict, duplicateRec.Code, duplicateRec.Body.String())

	var duplicateBody response.ErrorResponse
	require.NoError(t, json.Unmarshal(duplicateRec.Body.Bytes(), &duplicateBody))
	assert.Equal(t, "CONFLICT", duplicateBody.Error.Code)
	assert.Equal(t, auth.MsgEmailAlreadyExists, duplicateBody.Error.Message)

	count, err := mdb.Collection("users").CountDocuments(ctx, bson.D{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "one mailbox must stay one account")

	loginRec := doRequest(
		t,
		h,
		http.MethodPost,
		"/auth/login",
		fmt.Sprintf(`{"email":%q,"password":%q}`, "Another Name <player@example.com>", integrationPassword),
	)
	require.Equal(t, http.StatusOK, loginRec.Code, loginRec.Body.String())

	var loginSession response.Success[auth.SessionResponse]
	require.NoError(t, json.Unmarshal(loginRec.Body.Bytes(), &loginSession))
	assert.Equal(t, registeredID, jwtSubject(t, loginSession.Data.AccessToken))

	legacyID := "01900000-0000-7000-8000-0000000000aa"
	_, err = mdb.Collection("users").InsertOne(ctx, bson.M{
		"_id":           legacyID,
		"email":         "Legacy@Example.COM",
		"username":      "legacy",
		"password_hash": "bcrypt-hash",
	})
	require.NoError(t, err)

	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	var legacy bson.M
	require.NoError(t, mdb.Collection("users").FindOne(ctx, bson.D{{Key: "_id", Value: legacyID}}).Decode(&legacy))
	assert.NotContains(t, legacy, "email_verified_at", "the boot path must not backfill a legacy document")
	assert.Equal(t, "Legacy@Example.COM", legacy["email"], "the boot path must not rewrite a legacy address")
}
