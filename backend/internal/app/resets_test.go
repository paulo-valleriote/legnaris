package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/internal/sheet/sheettest"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	resetRouteRequestID  = "01900000-0000-7000-8000-0000000000c1"
	resetRouteRequestID2 = "01900000-0000-7000-8000-0000000000c2"
)

func newResetRouter(t *testing.T, repo sheet.Repository) http.Handler {
	t.Helper()

	dag := sheettest.ListDAG(t)
	svc := sheet.NewService(
		repo,
		templatemocks.NewMockService(t),
		sheettest.DAGMap{sheettest.ListTemplateID: dag},
		testLog,
		events.NoOp{},
	)

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(withOwner(errorTestOwnerID))
	app.NewSheetController(svc).RegisterRoutes(e.Group("/sheets"))
	return e
}

func resetRouteSheet(version int64, ownerID, lastRequestID string, level float64) *sheet.Sheet {
	return &sheet.Sheet{
		ID:                 sheetStubID,
		TemplateID:         sheettest.ListTemplateID,
		OwnerID:            ownerID,
		Version:            version,
		LastResetRequestID: lastRequestID,
		Data: map[string]any{
			"level":          level,
			"inventory":      map[string]any{"items": map[string]any{}},
			"inventoryValue": 0.0,
			"inventoryCount": 0.0,
		},
	}
}

func TestResets_Endpoint(t *testing.T) {
	path := "/sheets/" + sheetStubID + "/resets/" + sheettest.ListActionConsume
	body := `{"request_id":"` + resetRouteRequestID + `"}`

	t.Run("Happy path: 200 with the assigned cascade and no version in the body", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, sheetStubID).
			Return(resetRouteSheet(5, errorTestOwnerID, "", 5), nil).Once()
		repo.EXPECT().PartialUpdate(mock.Anything, sheetStubID, int64(5), mock.Anything).
			Return(int64(6), nil).Once()

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var decoded response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
		assert.Equal(t, map[string]any{"level": 4.0}, decoded.Data)
		assert.NotContains(t, rec.Body.String(), "version")
	})

	t.Run("Edge: a replay of the same request id is 200 with no write", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, sheetStubID).
			Return(resetRouteSheet(6, errorTestOwnerID, resetRouteRequestID, 4), nil).Once()

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, body)
		require.Equal(t, http.StatusOK, rec.Code)

		var decoded response.Success[map[string]any]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
		assert.Empty(t, decoded.Data)
		repo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: an unknown action is 404 NOT_FOUND", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, sheetStubID).
			Return(resetRouteSheet(1, errorTestOwnerID, "", 1), nil).Once()

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost,
			"/sheets/"+sheetStubID+"/resets/ghost", body)
		assertResetError(t, rec, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("Error: an empty request id is 400 INVALID_REQUEST", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, `{}`)
		assertResetError(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var decoded response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
		assert.Equal(t, sheet.MsgRequestIDMissing, decoded.Error.Message)
	})

	t.Run("Error: a malformed request id is 400 INVALID_REQUEST", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, `{"request_id":"not-a-uuid"}`)
		assertResetError(t, rec, http.StatusBadRequest, "INVALID_REQUEST")

		var decoded response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
		assert.Equal(t, sheet.MsgRequestIDInvalid, decoded.Error.Message)
	})

	t.Run("Error: malformed JSON is 400 INVALID_JSON", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, `{"request_id":`)
		assertResetError(t, rec, http.StatusBadRequest, "INVALID_JSON")
	})

	t.Run("Error: an unknown body key is 400 INVALID_JSON", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, `{"request_id":"`+resetRouteRequestID+`","data":{}}`)
		assertResetError(t, rec, http.StatusBadRequest, "INVALID_JSON")
	})

	t.Run("Error: a malformed sheet id is 400 INVALID_REQUEST beyond INVALID_JSON", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost,
			"/sheets/not-a-uuid/resets/"+sheettest.ListActionConsume, body)
		assertResetError(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("Error: a foreign sheet is 404 NOT_FOUND", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, sheetStubID).
			Return(resetRouteSheet(1, "01900000-0000-7000-8000-0000000000fe", "", 1), nil).Once()

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, body)
		assertResetError(t, rec, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("Error: a missing sheet is 404 NOT_FOUND", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, sheetStubID).Return(nil, sheeterr.ErrSheetNotFound).Once()

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, body)
		assertResetError(t, rec, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("Error: a lost race is 409 CONFLICT with Retry-After 1", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, sheetStubID).
			Return(resetRouteSheet(3, errorTestOwnerID, "", 1), nil).Once()
		repo.EXPECT().PartialUpdate(mock.Anything, sheetStubID, int64(3), mock.Anything).
			Return(int64(0), sheeterr.ErrVersionConflict).Once()

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path, body)
		assertResetError(t, rec, http.StatusConflict, "CONFLICT")
		assert.Equal(t, "1", rec.Header().Get(echo.HeaderRetryAfter))
	})

	t.Run("Edge: a new request id restarts even after a replay id was stored", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, sheetStubID).
			Return(resetRouteSheet(6, errorTestOwnerID, resetRouteRequestID, 4), nil).Once()
		repo.EXPECT().PartialUpdate(mock.Anything, sheetStubID, int64(6), mock.Anything).
			Return(int64(7), nil).Once()

		rec := doRequest(t, newResetRouter(t, repo), http.MethodPost, path,
			`{"request_id":"`+resetRouteRequestID2+`"}`)
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func assertResetError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())

	var decoded response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
	assert.Equal(t, code, decoded.Error.Code)
}
