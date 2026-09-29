package app_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"legnaris/internal/app"
	"legnaris/internal/engine"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/mocks"
	"legnaris/internal/sheet/sheettest"
	"legnaris/internal/sheet/template"
	templatemocks "legnaris/internal/sheet/template/mocks"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newSheetErrorRouter wires the real SheetController + HTTPErrorHandler over a
// real sheet.Service backed by a mocked repository, so the full translation
// chain (repository/DAG typed errors → apperr → HTTP status) is exercised
// without MongoDB. The template-service mock is returned so tests whose flow
// reaches FetchTemplate (Create asks it first, LGN-86) can set expectations.
func newSheetErrorRouter(t *testing.T, mockRepo *mocks.MockRepository, dagCache sheettest.DAGMap) (http.Handler, *templatemocks.MockService) {
	t.Helper()

	mockTemplateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
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
