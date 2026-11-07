package app_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func diceText(s string) *string { return &s }

func TestErrorMapping_SetDice(t *testing.T) {
	path := "/sheets/" + sheetStubID + "/dice"
	validBody := `{"dice":{"section":{"stats":"1d20 + @","config":null},"row":{"stats.strength":null,"config#armorClassAbility":"2d6"}}}`
	validDice := &sheet.Dice{
		Section: map[string]*string{"stats": diceText("1d20 + @"), "config": nil},
		Row:     map[string]*string{"stats.strength": nil, "config#armorClassAbility": diceText("2d6")},
	}
	tooMany := func() string {
		entries := make([]string, 0, 65)
		for i := range 65 {
			entries = append(entries, `"r`+strings.Repeat("x", i)+`":null`)
		}
		return `{"dice":{"row":{` + strings.Join(entries, ",") + `}}}`
	}()

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		stored      *sheet.Sheet
		fetchErr    error
		wantFetch   bool
		wantUpdate  bool
		wantStored  *sheet.Dice
		wantStatus  int
		wantCode    string
		wantMessage string
		wantBody    string
	}{
		{
			name: "Error: malformed JSON is 400 INVALID_JSON", method: http.MethodPut, path: path, body: `{"dice":`,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_JSON", wantMessage: "request body contains invalid JSON",
		},
		{
			name: "Error: empty body is 400 INVALID_JSON", method: http.MethodPut, path: path, body: ``,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_JSON", wantMessage: "request body contains invalid JSON",
		},
		{
			name: "Error: a number where an expression belongs is 400 INVALID_JSON", method: http.MethodPut, path: path,
			body:       `{"dice":{"row":{"stats.strength":20}}}`,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_JSON", wantMessage: "request body contains invalid JSON",
		},
		{
			name: "Error: an array where a map belongs is 400 INVALID_JSON", method: http.MethodPut, path: path,
			body:       `{"dice":{"section":["stats"]}}`,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_JSON", wantMessage: "request body contains invalid JSON",
		},
		{
			name: "Error: unknown key inside dice is 400 INVALID_JSON", method: http.MethodPut, path: path,
			body:       `{"dice":{"rows":{}}}`,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_JSON", wantMessage: "request body contains invalid JSON",
		},
		{
			name: "Error: unknown top-level field is 400 INVALID_JSON", method: http.MethodPut, path: path,
			body:       `{"dice":{},"data":{}}`,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_JSON", wantMessage: "request body contains invalid JSON",
		},
		{
			name: "Error: malformed sheet id is 400 and never reaches the repository", method: http.MethodPut,
			path: "/sheets/not-a-uuid/dice", body: validBody,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST", wantMessage: sheet.MsgSheetIDInvalid,
		},
		{
			name: "Error: missing sheet is 404", method: http.MethodPut, path: path, body: validBody,
			fetchErr: sheet.ErrSheetNotFound, wantFetch: true,
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND", wantMessage: "sheet not found",
		},
		{
			name: "Error: foreign sheet is the same 404 as a missing one", method: http.MethodPut, path: path, body: validBody,
			stored: layoutRouteSheet("someone-else"), wantFetch: true,
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND", wantMessage: "sheet not found",
		},
		{
			name: "Error: unknown section is 400 INVALID_REQUEST", method: http.MethodPut, path: path,
			body:   `{"dice":{"section":{"ghost":null}}}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
			wantMessage: "invalid dice: dice section 'ghost' is not a section of the template",
		},
		{
			name: "Error: unknown row is 400 INVALID_REQUEST", method: http.MethodPut, path: path,
			body:   `{"dice":{"row":{"stats.charisma":"1d20"}}}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
			wantMessage: "invalid dice: dice row 'stats.charisma' is not a row of any section",
		},
		{
			name: "Error: invalid expression is 400 INVALID_REQUEST", method: http.MethodPut, path: path,
			body:   `{"dice":{"row":{"stats.strength":"banana"}}}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
		{
			name: "Error: row override that does not fit is 400 INVALID_REQUEST", method: http.MethodPut, path: path,
			body:   `{"dice":{"row":{"config#armorClassAbility":"1d20 + stats.ghost"}}}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
		{
			name: "Error: section override fitting no row is 400 INVALID_REQUEST", method: http.MethodPut, path: path,
			body:   `{"dice":{"section":{"stats":"1d20 + @.total"}}}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
		{
			name: "Error: 65 entries is 400 INVALID_REQUEST", method: http.MethodPut, path: path, body: tooMany,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
			wantMessage: "invalid dice: 65 dice overrides exceed the maximum of 64",
		},
		{
			name: "Error: PATCH on the dice path is 405", method: http.MethodPatch, path: path, body: validBody,
			wantStatus: http.StatusMethodNotAllowed, wantCode: "METHOD_NOT_ALLOWED", wantMessage: "method not allowed",
		},
		{
			name: "Edge: absent dice clears and answers an empty block", method: http.MethodPut, path: path, body: `{}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true, wantUpdate: true,
			wantStatus: http.StatusOK, wantBody: `{"data":{"dice":{"section":{},"row":{}}}}`,
		},
		{
			name: "Edge: null dice clears", method: http.MethodPut, path: path, body: `{"dice":null}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true, wantUpdate: true,
			wantStatus: http.StatusOK, wantBody: `{"data":{"dice":{"section":{},"row":{}}}}`,
		},
		{
			name: "Edge: empty dice object clears", method: http.MethodPut, path: path, body: `{"dice":{}}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true, wantUpdate: true,
			wantStatus: http.StatusOK, wantBody: `{"data":{"dice":{"section":{},"row":{}}}}`,
		},
		{
			name: "Edge: dice of null maps clears", method: http.MethodPut, path: path,
			body:   `{"dice":{"section":null,"row":null}}`,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true, wantUpdate: true,
			wantStatus: http.StatusOK, wantBody: `{"data":{"dice":{"section":{},"row":{}}}}`,
		},
		{
			name: "Happy path: valid block keeps null distinct from absent and is echoed", method: http.MethodPut, path: path,
			body:   validBody,
			stored: layoutRouteSheet(errorTestOwnerID), wantFetch: true, wantUpdate: true, wantStored: validDice,
			wantStatus: http.StatusOK,
			wantBody:   `{"data":{"dice":{"section":{"stats":"1d20 + @","config":null},"row":{"stats.strength":null,"config#armorClassAbility":"2d6"}}}}`,
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
			clears := tc.wantUpdate && tc.wantStored == nil
			if tc.stored != nil && tc.stored.OwnerID == errorTestOwnerID && !clears {
				templateSvc.EXPECT().
					FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
					Return(layoutRouteTemplate(), nil).
					Once()
			}
			if tc.wantUpdate {
				mockRepo.EXPECT().UpdateDice(mock.Anything, sheetStubID, tc.wantStored).Return(nil).Once()
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
			if tc.wantMessage != "" {
				assert.Equal(t, tc.wantMessage, body.Error.Message)
			}
		})
	}
}

func TestErrorMapping_FetchSheetIncludesDice(t *testing.T) {
	t.Run("Happy path: a stored block, null included, is part of the sheet response", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		stored := layoutRouteSheet(errorTestOwnerID)
		stored.Dice = &sheet.Dice{
			Section: map[string]*string{"stats": nil},
			Row:     map[string]*string{"stats.strength": diceText("1d20 + @")},
		}
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(stored, nil).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))
		rec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetStubID, "")

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"dice":{"section":{"stats":null},"row":{"stats.strength":"1d20 + @"}}`)
	})

	t.Run("Edge: a sheet created before dice returns an empty block", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(layoutRouteSheet(errorTestOwnerID), nil).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))
		rec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetStubID, "")

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"dice":{"section":{},"row":{}}`)
	})

	t.Run("Edge: a stale override for a row the template no longer has is returned, not an error", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		stored := layoutRouteSheet(errorTestOwnerID)
		stored.Dice = &sheet.Dice{
			Section: map[string]*string{"retired": diceText("1d20")},
			Row:     map[string]*string{"stats.retired": diceText("1d20 + stats.gone")},
		}
		mockRepo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(stored, nil).Once()

		h, _ := newSheetErrorRouter(t, mockRepo, newErrorTestDAG(t))
		rec := doRequest(t, h, http.MethodGet, "/sheets/"+sheetStubID, "")

		require.Equal(t, http.StatusOK, rec.Code)
		var body response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, diceText("1d20"), body.Data.Dice.Section["retired"])
	})
}
