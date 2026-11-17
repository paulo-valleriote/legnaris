package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const genUserID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"

type fakeGenerationService struct {
	create func(cmd generation.CreateCommand) (string, error)
	fetch  func(userID, id string) (generation.View, error)
	accept func(cmd generation.AcceptCommand) (generation.AcceptResult, error)
}

func (f fakeGenerationService) Create(_ context.Context, cmd generation.CreateCommand) (string, error) {
	return f.create(cmd)
}

func (f fakeGenerationService) Fetch(_ context.Context, userID, id string) (generation.View, error) {
	return f.fetch(userID, id)
}

func (f fakeGenerationService) Accept(_ context.Context, cmd generation.AcceptCommand) (generation.AcceptResult, error) {
	return f.accept(cmd)
}

func generationEngine(svc app.GenerationService, userID string, create, read ratelimit.Allower) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if userID != "" {
				c.Set(auth.UserIDContextKey, userID)
			}
			return next(c)
		}
	}
	app.NewGenerationController(svc).RegisterRoutes(e.Group("/generations", inject), create, read)
	return e
}

func unlimited() ratelimit.Allower {
	return ratelimit.New(1000, time.Minute, time.Now)
}

func doJSON(t *testing.T, e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func dataOf(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

func errorCodeOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Error.Code
}

const validCreateBody = `{"template_id":"fantasy_v1","kind":"character","concept":"A grizzled dwarf smith"}`

func TestGenerationController_Create(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		body       string
		service    fakeGenerationService
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name:   "Happy path: the queued generation is answered 202 with its id",
			userID: genUserID,
			body:   validCreateBody,
			service: fakeGenerationService{create: func(cmd generation.CreateCommand) (string, error) {
				assert.Equal(t, genUserID, cmd.UserID)
				assert.Equal(t, "fantasy_v1", cmd.TemplateID)
				assert.Equal(t, "character", cmd.Kind)
				return "0196e4f2-0000-7000-8000-0000000000aa", nil
			}},
			wantStatus: http.StatusAccepted,
			wantID:     "0196e4f2-0000-7000-8000-0000000000aa",
		},
		{
			name:   "Happy path: the requested NPC kind reaches the service",
			userID: genUserID,
			body:   `{"template_id":"fantasy_v1","kind":"npc","concept":"A grizzled dwarf smith"}`,
			service: fakeGenerationService{create: func(cmd generation.CreateCommand) (string, error) {
				assert.Equal(t, "npc", cmd.Kind)
				return "0196e4f2-0000-7000-8000-0000000000aa", nil
			}},
			wantStatus: http.StatusAccepted,
			wantID:     "0196e4f2-0000-7000-8000-0000000000aa",
		},
		{
			name:   "Error: an unknown kind reaches the service and is answered 400",
			userID: genUserID,
			body:   `{"template_id":"fantasy_v1","kind":"monster","concept":"A grizzled dwarf smith"}`,
			service: fakeGenerationService{create: func(cmd generation.CreateCommand) (string, error) {
				assert.Equal(t, "monster", cmd.Kind)
				return "", apperr.NewError(apperr.CodeInvalidRequest, generation.MsgKindInvalid, nil)
			}},
			wantStatus: http.StatusBadRequest,
			wantCode:   string(apperr.CodeInvalidRequest),
		},
		{
			name:   "Error: a missing kind reaches the service and is answered 400",
			userID: genUserID,
			body:   `{"template_id":"fantasy_v1","concept":"A grizzled dwarf smith"}`,
			service: fakeGenerationService{create: func(cmd generation.CreateCommand) (string, error) {
				assert.Equal(t, "", cmd.Kind)
				return "", apperr.NewError(apperr.CodeInvalidRequest, generation.MsgKindInvalid, nil)
			}},
			wantStatus: http.StatusBadRequest,
			wantCode:   string(apperr.CodeInvalidRequest),
		},
		{
			name:       "Error: an unauthenticated request is 401",
			userID:     "",
			body:       validCreateBody,
			service:    fakeGenerationService{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeUnauthorized),
		},
		{
			name:       "Error: malformed JSON is 400 before any gate",
			userID:     genUserID,
			body:       `{"template_id":`,
			service:    fakeGenerationService{},
			wantStatus: http.StatusBadRequest,
			wantCode:   string(apperr.CodeInvalidJSON),
		},
		{
			name:   "Error: the kill-switch answers 503 PROVIDER_UNAVAILABLE",
			userID: genUserID,
			body:   validCreateBody,
			service: fakeGenerationService{create: func(generation.CreateCommand) (string, error) {
				return "", apperr.NewError(apperr.CodeProviderUnavailable, generation.MsgDisabled, nil)
			}},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   string(apperr.CodeProviderUnavailable),
		},
		{
			name:   "Error: a consent refusal is 403 LLM_CONSENT_REQUIRED",
			userID: genUserID,
			body:   validCreateBody,
			service: fakeGenerationService{create: func(generation.CreateCommand) (string, error) {
				return "", apperr.NewError(apperr.CodeLLMConsentRequired, "llm consent is required", nil)
			}},
			wantStatus: http.StatusForbidden,
			wantCode:   string(apperr.CodeLLMConsentRequired),
		},
		{
			name:   "Error: a job in progress is 409 GENERATION_IN_PROGRESS",
			userID: genUserID,
			body:   validCreateBody,
			service: fakeGenerationService{create: func(generation.CreateCommand) (string, error) {
				return "", apperr.NewError(apperr.CodeGenerationInProgress, generation.MsgInProgress, nil)
			}},
			wantStatus: http.StatusConflict,
			wantCode:   string(apperr.CodeGenerationInProgress),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := generationEngine(tc.service, tc.userID, unlimited(), unlimited())
			rec := doJSON(t, e, http.MethodPost, "/generations", tc.body)

			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantID != "" {
				assert.Equal(t, tc.wantID, dataOf(t, rec)["id"])
			}
			if tc.wantCode != "" {
				assert.Equal(t, tc.wantCode, errorCodeOf(t, rec))
			}
		})
	}
}

func TestGenerationController_CreateCapCarriesRetryAfter(t *testing.T) {
	svc := fakeGenerationService{create: func(generation.CreateCommand) (string, error) {
		return "", apperr.NewError(apperr.CodeGenerationCapReached, generation.MsgCapReached, nil).WithRetryAfter(90 * time.Second)
	}}
	rec := doJSON(t, generationEngine(svc, genUserID, unlimited(), unlimited()), http.MethodPost, "/generations", validCreateBody)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "90", rec.Header().Get(echo.HeaderRetryAfter))
	assert.Equal(t, string(apperr.CodeGenerationCapReached), errorCodeOf(t, rec))
}

func TestGenerationController_CreateRateLimit(t *testing.T) {
	calls := 0
	svc := fakeGenerationService{create: func(generation.CreateCommand) (string, error) {
		calls++
		return "0196e4f2-0000-7000-8000-0000000000aa", nil
	}}
	e := generationEngine(svc, genUserID, ratelimit.New(1, time.Minute, time.Now), unlimited())

	first := doJSON(t, e, http.MethodPost, "/generations", validCreateBody)
	second := doJSON(t, e, http.MethodPost, "/generations", validCreateBody)

	assert.Equal(t, http.StatusAccepted, first.Code)
	assert.Equal(t, http.StatusTooManyRequests, second.Code)
	assert.Equal(t, string(apperr.CodeRateLimited), errorCodeOf(t, second))
	assert.Equal(t, 1, calls, "a refused request must not reach the service")
}

func TestGenerationController_Get(t *testing.T) {
	const id = "0196e4f2-0000-7000-8000-0000000000aa"
	tests := []struct {
		name       string
		view       generation.View
		err        error
		wantStatus int
		wantCode   string
		wantValues bool
		wantError  string
		wantName   any
	}{
		{
			name:       "Happy path: a succeeded generation exposes its values and suggested name",
			view:       generation.View{ID: id, Status: generation.StatusSucceeded, Kind: "character", TemplateID: "fantasy_v1", Values: map[string]any{"name": "Brom"}, SuggestedName: ptrString("Brom")},
			wantStatus: http.StatusOK,
			wantValues: true,
			wantName:   "Brom",
		},
		{
			name:       "Edge: a succeeded generation without a name suggests null",
			view:       generation.View{ID: id, Status: generation.StatusSucceeded, Kind: "character", TemplateID: "fantasy_v1", Values: map[string]any{"name": ""}},
			wantStatus: http.StatusOK,
			wantValues: true,
			wantName:   nil,
		},
		{
			name:       "Edge: a queued generation carries no values key and a null suggested name",
			view:       generation.View{ID: id, Status: generation.StatusQueued, Kind: "character"},
			wantStatus: http.StatusOK,
			wantName:   nil,
		},
		{
			name:       "Happy path: an NPC generation returns its kind",
			view:       generation.View{ID: id, Status: generation.StatusQueued, Kind: "npc"},
			wantStatus: http.StatusOK,
			wantName:   nil,
		},
		{
			name:       "Edge: a failed generation carries its error code",
			view:       generation.View{ID: id, Status: generation.StatusFailed, Kind: "character", ErrorCode: "provider_unavailable"},
			wantStatus: http.StatusOK,
			wantError:  "provider_unavailable",
		},
		{
			name:       "Error: a foreign or expired generation is 404",
			err:        apperr.NewError(apperr.CodeNotFound, generation.MsgNotFound, nil),
			wantStatus: http.StatusNotFound,
			wantCode:   string(apperr.CodeNotFound),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := fakeGenerationService{fetch: func(userID, got string) (generation.View, error) {
				assert.Equal(t, genUserID, userID)
				assert.Equal(t, id, got)
				return tc.view, tc.err
			}}
			rec := doJSON(t, generationEngine(svc, genUserID, unlimited(), unlimited()), http.MethodGet, "/generations/"+id, "")

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantCode != "" {
				assert.Equal(t, tc.wantCode, errorCodeOf(t, rec))
				return
			}
			data := dataOf(t, rec)
			assert.Equal(t, id, data["id"])
			assert.Equal(t, tc.view.Kind, data["kind"])
			_, hasValues := data["values"]
			assert.Equal(t, tc.wantValues, hasValues)
			nameValue, hasName := data["suggested_name"]
			assert.True(t, hasName, "suggested_name is always present")
			assert.Equal(t, tc.wantName, nameValue)
			if tc.wantError != "" {
				assert.Equal(t, tc.wantError, data["error_code"])
			} else {
				_, hasError := data["error_code"]
				assert.False(t, hasError)
			}
		})
	}
}

func TestGenerationController_Accept(t *testing.T) {
	const id = "0196e4f2-0000-7000-8000-0000000000aa"
	const sheetID = "0196e4f2-0000-7000-8000-0000000000bb"
	tests := []struct {
		name       string
		body       string
		result     generation.AcceptResult
		err        error
		wantStatus int
		wantCode   string
		wantSheet  string
	}{
		{name: "Happy path: the first accept is 201 with the new sheet id", body: `{"sheet_name":"Brom","values":{"name":"Brom"}}`, result: generation.AcceptResult{SheetID: sheetID, Created: true}, wantStatus: http.StatusCreated, wantSheet: sheetID},
		{name: "Edge: a repeat accept is 200 with the same sheet id", body: `{"sheet_name":"Brom"}`, result: generation.AcceptResult{SheetID: sheetID}, wantStatus: http.StatusOK, wantSheet: sheetID},
		{name: "Error: a generation that is not ready is 409", body: `{"sheet_name":"Brom","values":{"name":"Brom"}}`, err: apperr.NewError(apperr.CodeConflict, generation.MsgNotAcceptable, nil), wantStatus: http.StatusConflict, wantCode: string(apperr.CodeConflict)},
		{name: "Error: the Sheet limit refuses the accept with 409", body: `{"sheet_name":"Brom","values":{"name":"Brom"}}`, err: apperr.NewError(apperr.CodeSheetLimitReached, "sheet limit reached for this kind", nil), wantStatus: http.StatusConflict, wantCode: string(apperr.CodeSheetLimitReached)},
		{name: "Error: malformed JSON is 400", body: `{"sheet_name":`, wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeInvalidJSON)},
		{name: "Error: missing values is 400", body: `{"sheet_name":"Brom"}`, err: apperr.NewError(apperr.CodeInvalidRequest, generation.MsgValuesRequired, nil), wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeInvalidRequest)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := fakeGenerationService{accept: func(cmd generation.AcceptCommand) (generation.AcceptResult, error) {
				assert.Equal(t, id, cmd.ID)
				assert.Equal(t, genUserID, cmd.UserID)
				return tc.result, tc.err
			}}
			rec := doJSON(t, generationEngine(svc, genUserID, unlimited(), unlimited()), http.MethodPost, "/generations/"+id+"/accept", tc.body)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantCode != "" {
				assert.Equal(t, tc.wantCode, errorCodeOf(t, rec))
				return
			}
			assert.Equal(t, tc.wantSheet, dataOf(t, rec)["sheet_id"])
		})
	}
}

func TestGenerationController_UnauthenticatedGetRejected(t *testing.T) {
	svc := fakeGenerationService{fetch: func(string, string) (generation.View, error) {
		return generation.View{}, errors.New("must not be called")
	}}
	rec := doJSON(t, generationEngine(svc, "", unlimited(), unlimited()), http.MethodGet, "/generations/0196e4f2-0000-7000-8000-0000000000aa", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func ptrString(s string) *string {
	return &s
}
