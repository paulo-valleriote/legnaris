package app_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"legnaris/internal/app"
	"legnaris/internal/engine"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/mocks"
	"legnaris/internal/sheet/template"
	templatemocks "legnaris/internal/sheet/template/mocks"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// dagProvider adapts a plain map to the sheet.DAGProvider interface, so the
// error-mapping tests can wire a hand-built DAG without a whole
// template.Registry (LGN-54: the service takes a provider, not a mutable map).
type dagProvider map[string]*template.DAG

func (p dagProvider) DAG(id string) (*template.DAG, bool) {
	dag, ok := p[id]
	return dag, ok
}

// newSheetErrorRouter wires the real SheetController + HTTPErrorHandler over a
// real sheet.Service backed by a mocked repository, so the full translation
// chain (repository/DAG typed errors → apperr → HTTP status) is exercised
// without MongoDB.
func newSheetErrorRouter(t *testing.T, mockRepo *mocks.MockRepository, dagCache dagProvider) http.Handler {
	t.Helper()

	mockTemplateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	app.NewSheetController(svc).RegisterRoutes(e.Group("/sheets"))
	return e
}

func newErrorTestDAG(t *testing.T) dagProvider {
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
	return dagProvider{"fantasy_v1": dag}
}

// TestErrorMapping_SheetNotExistsReturns404 covers the card's headline case:
// GET /sheets/:id for a missing sheet must respond 404, not 500 (the
// repository's ErrSheetNotFound sentinel is translated by the service).
func TestErrorMapping_SheetNotExistsReturns404(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		FetchByID(mock.Anything, "missing").
		Return(nil, sheet.ErrSheetNotFound).
		Once()

	h := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

	rec := doRequest(t, h, http.MethodGet, "/sheets/missing", "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body response.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
	assert.Equal(t, "sheet not found", body.Error.Message)
}

// TestErrorMapping_UnknownTemplateReturns404: POST with a template that has no
// DAG in the registry must respond 404 (the service must not reach the repo).
func TestErrorMapping_UnknownTemplateReturns404(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

	h := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

	rec := doRequest(t, h, http.MethodPost, "/sheets", `{"template_id":"unknown_template","sheet_name":"x"}`)
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body response.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
	assert.Equal(t, "template not found", body.Error.Message)
}

// TestErrorMapping_PatchFieldErrors: unknown field → 400 UNKNOWN_FIELD,
// non-editable field → 400 INVALID_REQUEST.
func TestErrorMapping_PatchFieldErrors(t *testing.T) {
	storedSheet := func() *sheet.Sheet {
		return &sheet.Sheet{
			ID:         "s1",
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
				FetchByID(mock.Anything, "s1").
				Return(storedSheet(), nil).
				Once()

			h := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

			rec := doRequest(t, h, http.MethodPatch, "/sheets/s1", tc.payload)
			require.Equal(t, http.StatusBadRequest, rec.Code)

			var body response.Error
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

			h := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

			rec := doRequest(t, h, http.MethodPost, "/sheets", tc.payload)
			require.Equal(t, http.StatusBadRequest, rec.Code)

			var body response.Error
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
		})
	}
}
