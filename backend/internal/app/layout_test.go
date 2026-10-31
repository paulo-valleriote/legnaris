package app_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func layoutRouteTemplate() *template.Template {
	return &template.Template{
		ID: "fantasy_v1",
		Fields: template.FieldGroup{
			"stats": {Children: template.FieldGroup{
				"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true}},
				"strength":  {Schema: &template.FieldSchema{Type: "number", IsEditable: true}},
			}},
			"config": {Children: template.FieldGroup{
				"acUsesCon": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility"}},
				"acUsesDex": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility"}},
			}},
		},
		SectionOrder: []string{"stats", "config"},
	}
}

func layoutRouteSheet(ownerID string) *sheet.Sheet {
	return &sheet.Sheet{
		ID:         sheetStubID,
		TemplateID: "fantasy_v1",
		OwnerID:    ownerID,
		Name:       "Layout",
		Data:       map[string]any{"stats": map[string]any{"dexterity": 10.0}},
	}
}

func TestErrorMapping_SetLayout(t *testing.T) {
	validBody := `{"layout":{"section_order":["config","stats"],"pinned":["config#armorClassAbility"]}}`
	validBodyWithRowOrder := `{"layout":{"section_order":["config","stats"],"row_order":{"stats":["stats.strength"]},"pinned":["config#armorClassAbility"]}}`
	validLayout := &sheet.Layout{
		SectionOrder: []string{"config", "stats"},
		Pinned:       []string{"config#armorClassAbility"},
	}

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		stored      *sheet.Sheet
		fetchErr    error
		wantFetch   bool
		wantUpdate  bool
		wantLayout  *sheet.Layout
		wantStatus  int
		wantCode    string
		wantMessage string
		wantBody    string
	}{
		{
			name:        "Error: malformed JSON is 400 INVALID_JSON",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_JSON",
			wantMessage: "request body contains invalid JSON",
		},
		{
			name:        "Error: empty body is 400 INVALID_JSON",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        ``,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_JSON",
			wantMessage: "request body contains invalid JSON",
		},
		{
			name:        "Error: layout of the wrong shape is 400 INVALID_JSON",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":{"section_order":"stats"}}`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_JSON",
			wantMessage: "request body contains invalid JSON",
		},
		{
			name:        "Error: unknown top-level field is rejected as 400 INVALID_JSON",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":{},"data":{}}`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_JSON",
			wantMessage: "request body contains invalid JSON",
		},
		{
			name:        "Error: unknown field inside layout is rejected as 400 INVALID_JSON",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":{"pinned_rows":["stats.strength"]}}`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_JSON",
			wantMessage: "request body contains invalid JSON",
		},
		{
			name:        "Error: malformed sheet id is 400 and never reaches the repository",
			method:      http.MethodPut,
			path:        "/sheets/not-a-uuid/layout",
			body:        validBody,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_REQUEST",
			wantMessage: sheet.MsgSheetIDInvalid,
		},
		{
			name:        "Error: missing sheet is 404",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        validBody,
			fetchErr:    sheet.ErrSheetNotFound,
			wantFetch:   true,
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "sheet not found",
		},
		{
			name:        "Error: foreign sheet is the same 404 as a missing one",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        validBody,
			stored:      layoutRouteSheet("someone-else"),
			wantFetch:   true,
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "sheet not found",
		},
		{
			name:        "Error: unknown section is 400 INVALID_REQUEST",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":{"section_order":["stats","ghost"]}}`,
			stored:      layoutRouteSheet(errorTestOwnerID),
			wantFetch:   true,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_REQUEST",
			wantMessage: "invalid layout: unknown layout section 'ghost'",
		},
		{
			name:        "Error: pin that is no row of any section is 400 INVALID_REQUEST",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":{"pinned":["stats.charisma"]}}`,
			stored:      layoutRouteSheet(errorTestOwnerID),
			wantFetch:   true,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_REQUEST",
			wantMessage: "invalid layout: unknown pinned row 'stats.charisma'",
		},
		{
			name:        "Error: removed row_order field is rejected as 400 INVALID_JSON",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":{"row_order":{"stats":["stats.strength"]}}}`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_JSON",
			wantMessage: "request body contains invalid JSON",
		},
		{
			name:        "Error: row_order next to valid fields is still rejected as 400 INVALID_JSON",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        validBodyWithRowOrder,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_JSON",
			wantMessage: "request body contains invalid JSON",
		},
		{
			name:        "Error: duplicate pin is 400 INVALID_REQUEST",
			method:      http.MethodPut,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        `{"layout":{"pinned":["stats.strength","stats.strength"]}}`,
			stored:      layoutRouteSheet(errorTestOwnerID),
			wantFetch:   true,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "INVALID_REQUEST",
			wantMessage: "invalid layout: duplicate key 'stats.strength' in pinned",
		},
		{
			name:        "Error: PATCH on the layout path is 405",
			method:      http.MethodPatch,
			path:        "/sheets/" + sheetStubID + "/layout",
			body:        validBody,
			wantStatus:  http.StatusMethodNotAllowed,
			wantCode:    "METHOD_NOT_ALLOWED",
			wantMessage: "method not allowed",
		},
		{
			name:       "Edge: absent layout resets to null",
			method:     http.MethodPut,
			path:       "/sheets/" + sheetStubID + "/layout",
			body:       `{}`,
			stored:     layoutRouteSheet(errorTestOwnerID),
			wantFetch:  true,
			wantUpdate: true,
			wantStatus: http.StatusOK,
			wantBody:   `{"data":{"layout":null}}`,
		},
		{
			name:       "Edge: null layout resets to null",
			method:     http.MethodPut,
			path:       "/sheets/" + sheetStubID + "/layout",
			body:       `{"layout":null}`,
			stored:     layoutRouteSheet(errorTestOwnerID),
			wantFetch:  true,
			wantUpdate: true,
			wantStatus: http.StatusOK,
			wantBody:   `{"data":{"layout":null}}`,
		},
		{
			name:       "Edge: empty layout object resets to null",
			method:     http.MethodPut,
			path:       "/sheets/" + sheetStubID + "/layout",
			body:       `{"layout":{}}`,
			stored:     layoutRouteSheet(errorTestOwnerID),
			wantFetch:  true,
			wantUpdate: true,
			wantStatus: http.StatusOK,
			wantBody:   `{"data":{"layout":null}}`,
		},
		{
			name:       "Happy path: valid layout is stored and echoed",
			method:     http.MethodPut,
			path:       "/sheets/" + sheetStubID + "/layout",
			body:       validBody,
			stored:     layoutRouteSheet(errorTestOwnerID),
			wantFetch:  true,
			wantUpdate: true,
			wantLayout: validLayout,
			wantStatus: http.StatusOK,
			wantBody:   `{"data":{"layout":{"section_order":["config","stats"],"pinned":["config#armorClassAbility"]}}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			h, templateSvc := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))

			if tc.wantFetch {
				call := mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID)
				if tc.stored != nil {
					call.Return(tc.stored, nil).Once()
				} else {
					call.Return(nil, tc.fetchErr).Once()
				}
			}
			if tc.stored != nil && tc.stored.OwnerID == errorTestOwnerID && tc.body != `{}` &&
				tc.body != `{"layout":null}` && tc.body != `{"layout":{}}` {
				templateSvc.EXPECT().
					FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
					Return(layoutRouteTemplate(), nil).
					Once()
			}
			if tc.wantUpdate {
				mockRepo.EXPECT().UpdateLayout(mock.Anything, sheetStubID, tc.wantLayout).Return(nil).Once()
			}

			rec := doRequest(t, h, tc.method, tc.path, tc.body)
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())

			if tc.wantBody != "" {
				assert.JSONEq(t, tc.wantBody, rec.Body.String())
				return
			}
			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
			assert.Equal(t, tc.wantMessage, body.Error.Message)
		})
	}
}

func TestErrorMapping_FetchSheetIncludesLayout(t *testing.T) {
	t.Run("Happy path: a stored layout is part of the sheet response", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		stored := layoutRouteSheet(errorTestOwnerID)
		stored.Layout = &sheet.Layout{Pinned: []string{"stats.strength"}}
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(stored, nil).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))
		rec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetStubID, "")
		require.Equal(t, http.StatusOK, rec.Code)

		var body response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, &sheet.Layout{Pinned: []string{"stats.strength"}}, body.Data.Layout)
	})

	t.Run("Edge: a sheet without layout has no layout key", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(layoutRouteSheet(errorTestOwnerID), nil).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))
		rec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetStubID, "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.NotContains(t, rec.Body.String(), "layout")
	})
}
