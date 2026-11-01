package app_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"
	"legnaris/backend/shared/maputils"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testLog = logger.NewLogger(slog.LevelError)

// errorTestOwnerID stands in for the authenticated user id the JWT middleware
// would place in the context; these unit tests mount the controller directly,
// so the harness injects it.
const errorTestOwnerID = "01900000-0000-7000-8000-0000000000aa"

// withOwner simulates auth.Middleware for tests that mount a controller group
// directly: it stamps the authenticated owner id the handlers read.
func withOwner(userID string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(auth.UserIDContextKey, userID)
			return next(c)
		}
	}
}

// newSheetErrorRouter wires the real SheetController + HTTPErrorHandler over a
// real sheet.Service backed by a mocked repository, so the full translation
// chain (repository/DAG typed errors → apperr → HTTP status) is exercised
// without MongoDB. The template-service mock is returned so tests whose flow
// reaches FetchTemplate (Create asks it first, LGN-86) can set expectations.
func newSheetErrorRouter(t *testing.T, mockRepo *mocks.MockRepository, dagCache sheettest.DAGMap) (http.Handler, *templatemocks.MockService) {
	t.Helper()

	mockTemplateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache, testLog)

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(withOwner(errorTestOwnerID))
	app.NewSheetController(svc).RegisterRoutes(e.Group("/sheets"))
	return e, mockTemplateSvc
}

func newErrorTestDAG(t *testing.T) sheettest.DAGMap {
	t.Helper()

	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("fantasy_v1", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return sheettest.DAGMap{"fantasy_v1": dag}
}

// Canonical UUID v7 fixtures (LGN-86): the sheet endpoints reject anything
// that is not a canonical UUID v7 before the repository, so tests exercising
// the repository need IDs in the exact shape idgen.New produces.
const (
	sheetStubID    = "01900000-0000-7000-8000-000000000001"
	missingSheetID = "01900000-0000-7000-8000-000000000404"
)

// TestErrorMapping_SheetNotExistsReturns404 covers the card's headline case:
// GET /sheets/:id for a missing sheet must respond 404, not 500 (the
// repository's ErrSheetNotFound sentinel is translated by the service). The
// ID is well-formed but unknown — format validation must let it through.
func TestErrorMapping_SheetNotExistsReturns404(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		FetchByID(mock.Anything, missingSheetID).
		Return(nil, sheet.ErrSheetNotFound).
		Once()

	h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

	rec := doRequest(t, h, http.MethodGet, "/sheets/"+missingSheetID, "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
	assert.Equal(t, "sheet not found", body.Error.Message)
}

// TestErrorMapping_UnknownTemplateReturns404: POST with a template that has no
// DAG in the registry must respond 404 (the service must not reach the repo).
// FetchTemplate runs first (LGN-86) and the well-formed unknown slug passes
// its checks, so the mock hands a template over and the DAG lookup misses.
func TestErrorMapping_UnknownTemplateReturns404(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

	h, mockTemplateSvc := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))
	mockTemplateSvc.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "unknown_template"}).
		Return(&template.Template{ID: "unknown_template", Name: "Unknown"}, nil).
		Once()

	rec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"unknown_template","sheet_name":"x"}`)
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
	assert.Equal(t, "template not found", body.Error.Message)
}

// TestErrorMapping_PatchFieldErrors: unknown field → 400 UNKNOWN_FIELD,
// non-editable field → 400 INVALID_REQUEST.
func TestErrorMapping_PatchFieldErrors(t *testing.T) {
	storedSheet := func() *sheet.Sheet {
		return &sheet.Sheet{
			ID:         sheetStubID,
			TemplateID: "fantasy_v1",
			OwnerID:    errorTestOwnerID,
			Name:       "Err Map",
			Data: map[string]any{
				"stats":     map[string]any{"dexterity": 10.0},
				"modifiers": map[string]any{"dexterityMod": 0.0},
			},
		}
	}

	tests := []struct {
		name     string
		payload  string
		wantCode string
	}{
		{
			name:     "unknown field returns 400 UNKNOWN_FIELD",
			payload:  `{"data":{"stats":{"not_exists":1}}}`,
			wantCode: "UNKNOWN_FIELD",
		},
		{
			name:     "non-editable field returns 400 INVALID_REQUEST",
			payload:  `{"data":{"modifiers":{"dexterityMod":5}}}`,
			wantCode: "INVALID_REQUEST",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().
				FetchByID(mock.Anything, sheetStubID).
				Return(storedSheet(), nil).
				Once()

			h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

			rec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetStubID, tc.payload)
			require.Equal(t, http.StatusBadRequest, rec.Code)

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
		})
	}
}

// TestErrorMapping_InvalidBodyReturns400: malformed JSON and unknown body
// fields are translated by the controller (decodeError) to 400 codes without
// reaching the service.
func TestErrorMapping_InvalidBodyReturns400(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantCode string
	}{
		{
			name:     "invalid JSON returns 400 INVALID_JSON",
			payload:  `{"template_id":`,
			wantCode: "INVALID_JSON",
		},
		{
			name:     "unknown field in body returns 400 INVALID_JSON",
			payload:  `{"template_id":"fantasy_v1","sheet_name":"x","bogus":1}`,
			wantCode: "INVALID_JSON",
		},
		{
			name:     "trailing JSON returns 400 UNKNOWN_FIELD",
			payload:  `{"template_id":"fantasy_v1","sheet_name":"x"} {}`,
			wantCode: "UNKNOWN_FIELD",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

			h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

			rec := doRequest(t, h, http.MethodPost, "/sheets", tc.payload)
			require.Equal(t, http.StatusBadRequest, rec.Code)

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
		})
	}
}

// TestErrorMapping_MalformedSheetIDReturns400: a junk path param must surface
// as 400 INVALID_REQUEST through the HTTP boundary on both sheet entry
// points, and the repository must never see it (LGN-86).
func TestErrorMapping_MalformedSheetIDReturns400(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		payload string
	}{
		{
			name:   "GET with garbage ID returns 400",
			method: http.MethodGet,
			path:   "/sheets/not-a-uuid",
		},
		{
			name:    "PATCH with UUID v4 ID returns 400",
			method:  http.MethodPatch,
			path:    "/sheets/9f2c8a1b-3d4e-4f60-8a1b-2c3d4e5f6071",
			payload: `{"data":{"stats":{"dexterity":12}}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

			h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

			rec := doRequest(t, h, tc.method, tc.path, tc.payload)
			require.Equal(t, http.StatusBadRequest, rec.Code)

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
			assert.Equal(t, sheet.MsgSheetIDInvalid, body.Error.Message)
		})
	}
}

// newPreviewErrorDAG builds the DAG the preview route tests share: a base field,
// a derived leaf and an exclusive group, so every reject path PATCH maps can be
// exercised over HTTP.
func newPreviewErrorDAG(t *testing.T) sheettest.DAGMap {
	t.Helper()

	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
		}},
		"config": {Children: template.FieldGroup{
			"acUsesDex": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 1, ExclusiveGroup: "armorClassAbility"}},
			"acUsesCon": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 0, ExclusiveGroup: "armorClassAbility"}},
		}},
	}

	dag, err := template.BuildDAG("fantasy_v1", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return sheettest.DAGMap{"fantasy_v1": dag}
}

// TestErrorMapping_PreviewSheet pins POST /sheets/:id/preview end to end over
// the HTTP boundary: the cascade envelope on 200, no persistence, and the exact
// status codes PATCH maps for a malformed id, a missing sheet, an unknown field,
// a derived field, an exclusive conflict, over-deep nesting and a bad body.
func TestErrorMapping_PreviewSheet(t *testing.T) {
	storedSheet := func() *sheet.Sheet {
		return &sheet.Sheet{
			ID:         sheetStubID,
			TemplateID: "fantasy_v1",
			OwnerID:    errorTestOwnerID,
			Name:       "Preview",
			Data: map[string]any{
				"stats":     map[string]any{"dexterity": 14.0},
				"modifiers": map[string]any{"dexterityMod": 2.0},
				"config":    map[string]any{"acUsesDex": 1.0, "acUsesCon": 0.0},
			},
		}
	}

	deep := any(1.0)
	for i := 0; i < maputils.MaxNestingDepth+1; i++ {
		deep = map[string]any{"level": deep}
	}
	deepBody, err := json.Marshal(map[string]any{"data": deep})
	require.NoError(t, err)

	t.Run("Happy path: 200 with the cascade and no write", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(storedSheet(), nil).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newPreviewErrorDAG(t))

		rec := doRequest(t, h, http.MethodPost, "/sheets/"+sheetStubID+"/preview", `{"data":{"stats":{"dexterity":18}}}`)
		require.Equal(t, http.StatusOK, rec.Code)

		var body response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

		modifiers, ok := body.Data["modifiers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 4.0, modifiers["dexterityMod"])
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: malformed sheet id returns 400 INVALID_REQUEST", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

		h, _ := newSheetErrorRouter(t, mockRepo, newPreviewErrorDAG(t))

		rec := doRequest(t, h, http.MethodPost, "/sheets/not-a-uuid/preview", `{"data":{"stats":{"dexterity":12}}}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
		assert.Equal(t, sheet.MsgSheetIDInvalid, body.Error.Message)
	})

	t.Run("Error: missing sheet returns 404 NOT_FOUND", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(mock.Anything, missingSheetID).Return(nil, sheet.ErrSheetNotFound).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newPreviewErrorDAG(t))

		rec := doRequest(t, h, http.MethodPost, "/sheets/"+missingSheetID+"/preview", `{"data":{"stats":{"dexterity":12}}}`)
		require.Equal(t, http.StatusNotFound, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "NOT_FOUND", body.Error.Code)
		assert.Equal(t, "sheet not found", body.Error.Message)
	})

	t.Run("Error: invalid body returns 400 INVALID_JSON", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t) // no expectations: the body never reaches the service

		h, _ := newSheetErrorRouter(t, mockRepo, newPreviewErrorDAG(t))

		rec := doRequest(t, h, http.MethodPost, "/sheets/"+sheetStubID+"/preview", `{"data":`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_JSON", body.Error.Code)
	})

	t.Run("Error: DAG rejections return the same codes PATCH maps", func(t *testing.T) {
		tests := []struct {
			name     string
			payload  string
			wantCode string
		}{
			{
				name:     "unknown field returns 400 UNKNOWN_FIELD",
				payload:  `{"data":{"stats":{"not_exists":1}}}`,
				wantCode: "UNKNOWN_FIELD",
			},
			{
				name:     "derived field returns 400 INVALID_REQUEST",
				payload:  `{"data":{"modifiers":{"dexterityMod":5}}}`,
				wantCode: "INVALID_REQUEST",
			},
			{
				name:     "exclusive group conflict returns 400 INVALID_REQUEST",
				payload:  `{"data":{"config":{"acUsesDex":1,"acUsesCon":1}}}`,
				wantCode: "INVALID_REQUEST",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockRepo := mocks.NewMockRepository(t)
				mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(storedSheet(), nil).Once()

				h, _ := newSheetErrorRouter(t, mockRepo, newPreviewErrorDAG(t))

				rec := doRequest(t, h, http.MethodPost, "/sheets/"+sheetStubID+"/preview", tc.payload)
				require.Equal(t, http.StatusBadRequest, rec.Code)

				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tc.wantCode, body.Error.Code)
				mockRepo.AssertNotCalled(t, "PartialUpdate")
			})
		}
	})

	t.Run("Error: over-deep nesting returns 400 INVALID_REQUEST", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(storedSheet(), nil).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newPreviewErrorDAG(t))

		rec := doRequest(t, h, http.MethodPost, "/sheets/"+sheetStubID+"/preview", string(deepBody))
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})
}

// newTemplatePreviewDAG builds the template and DAG the template preview route
// tests share: a base field and one derived leaf.
func newTemplatePreviewDAG(t *testing.T) (sheettest.DAGMap, *template.Template) {
	t.Helper()

	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"mod": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("test", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return sheettest.DAGMap{"test": dag}, &template.Template{ID: "test", Name: "Test Template", Fields: fields}
}

// newTemplatePreviewRouter wires the sheet controller's /templates preview route
// over a real sheet service backed by a mocked repository, so the full
// translation chain is exercised without MongoDB. The template-service mock is
// returned so the tests own its expectations.
func newTemplatePreviewRouter(t *testing.T, mockRepo *mocks.MockRepository, dagCache sheettest.DAGMap) (http.Handler, *templatemocks.MockService) {
	t.Helper()

	mockTemplateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache, testLog)

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	app.NewSheetController(svc).RegisterTemplatePreviewRoute(e.Group("/templates"))
	return e, mockTemplateSvc
}

// TestErrorMapping_PreviewTemplate pins POST /templates/:id/preview over the
// HTTP boundary: the cascade envelope on 200 with no stored sheet, 400 for a
// missing or malformed template id, 404 for an unknown slug, and the same DAG
// codes the sheet preview maps. A body template_id is rejected: the id comes
// from the path only.
func TestErrorMapping_PreviewTemplate(t *testing.T) {
	dagCache, tmpl := newTemplatePreviewDAG(t)

	t.Run("Happy path: 200 with the cascade and no stored sheet", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t) // no expectations: the template preview never touches the repository

		h, mockTemplateSvc := newTemplatePreviewRouter(t, mockRepo, dagCache)
		mockTemplateSvc.EXPECT().
			FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "test"}).
			Return(tmpl, nil).
			Once()

		rec := doRequest(t, h, http.MethodPost, "/templates/test/preview", `{"data":{"stats":{"dexterity":18}}}`)
		require.Equal(t, http.StatusOK, rec.Code)

		var body response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

		mod, ok := body.Data["mod"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 4.0, mod["dexterityMod"])
		mockRepo.AssertNotCalled(t, "FetchByID")
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: malformed template id returns 400 INVALID_REQUEST", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)

		h, mockTemplateSvc := newTemplatePreviewRouter(t, mockRepo, dagCache)
		mockTemplateSvc.EXPECT().
			FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "Fantasy-V1"}).
			Return(nil, template.ErrTemplateIDInvalid).
			Once()

		rec := doRequest(t, h, http.MethodPost, "/templates/Fantasy-V1/preview", `{"data":{"stats":{"dexterity":12}}}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
		assert.Equal(t, "template id is not a valid slug", body.Error.Message)
	})

	t.Run("Error: empty template id in the path returns 400 INVALID_REQUEST", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)

		h, mockTemplateSvc := newTemplatePreviewRouter(t, mockRepo, dagCache)
		mockTemplateSvc.EXPECT().
			FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: ""}).
			Return(nil, template.ErrTemplateIDNotProvided).
			Once()

		rec := doRequest(t, h, http.MethodPost, "/templates//preview", `{"data":{"stats":{"dexterity":12}}}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_REQUEST", body.Error.Code)
		assert.Equal(t, "template id not provided", body.Error.Message)
	})

	t.Run("Error: unknown template slug returns 404 NOT_FOUND", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)

		h, mockTemplateSvc := newTemplatePreviewRouter(t, mockRepo, dagCache)
		mockTemplateSvc.EXPECT().
			FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "unknown"}).
			Return(nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)).
			Once()

		rec := doRequest(t, h, http.MethodPost, "/templates/unknown/preview", `{"data":{"stats":{"dexterity":12}}}`)
		require.Equal(t, http.StatusNotFound, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "NOT_FOUND", body.Error.Code)
		assert.Equal(t, "template not found", body.Error.Message)
	})

	t.Run("Error: a body template_id is rejected", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)

		h, _ := newTemplatePreviewRouter(t, mockRepo, dagCache)

		rec := doRequest(t, h, http.MethodPost, "/templates/test/preview", `{"template_id":"test","data":{"stats":{"dexterity":12}}}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_JSON", body.Error.Code)
	})

	t.Run("Error: DAG rejections return the same codes as the sheet preview", func(t *testing.T) {
		tests := []struct {
			name     string
			payload  string
			wantCode string
		}{
			{
				name:     "unknown field returns 400 UNKNOWN_FIELD",
				payload:  `{"data":{"stats":{"not_exists":1}}}`,
				wantCode: "UNKNOWN_FIELD",
			},
			{
				name:     "derived field returns 400 INVALID_REQUEST",
				payload:  `{"data":{"mod":{"dexterityMod":5}}}`,
				wantCode: "INVALID_REQUEST",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockRepo := mocks.NewMockRepository(t)

				h, mockTemplateSvc := newTemplatePreviewRouter(t, mockRepo, dagCache)
				mockTemplateSvc.EXPECT().
					FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "test"}).
					Return(tmpl, nil).
					Once()

				rec := doRequest(t, h, http.MethodPost, "/templates/test/preview", tc.payload)
				require.Equal(t, http.StatusBadRequest, rec.Code)

				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tc.wantCode, body.Error.Code)
				mockRepo.AssertNotCalled(t, "FetchByID")
			})
		}
	})
}

// TestErrorMapping_PatchVersionConflictReturns409: a PATCH that loses the
// compare-and-swap reaches the client as 409 CONFLICT with a Retry-After header
// after a single read and write; the retry is the client's job.
func TestErrorMapping_PatchVersionConflictReturns409(t *testing.T) {
	storedSheet := func(version int64) *sheet.Sheet {
		return &sheet.Sheet{
			ID:         sheetStubID,
			TemplateID: "fantasy_v1",
			OwnerID:    errorTestOwnerID,
			Version:    version,
			Data: map[string]any{
				"stats":     map[string]any{"dexterity": 10.0},
				"modifiers": map[string]any{"dexterityMod": 0.0},
			},
		}
	}
	const payload = `{"data":{"stats":{"dexterity":12}}}`

	t.Run("Error: a lost race returns 409 CONFLICT with Retry-After and no internal retry", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(storedSheet(0), nil).Once()
		mockRepo.EXPECT().PartialUpdate(mock.Anything, sheetStubID, int64(0), mock.Anything).Return(0, sheet.ErrVersionConflict).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

		rec := doRequest(t, h, http.MethodPatch, "/sheets/"+sheetStubID, payload)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "1", rec.Header().Get("Retry-After"))

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "CONFLICT", body.Error.Code)
	})
}
