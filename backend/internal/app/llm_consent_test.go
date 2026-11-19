package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/llmconsent/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newLLMConsentHandler(t *testing.T, service llmconsent.Service, withUser bool) http.Handler {
	t.Helper()

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	stub := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if withUser {
				c.Set(auth.UserIDContextKey, "user-1")
			}
			return next(c)
		}
	}
	app.NewLLMConsentController(service).RegisterRoutes(e.Group("/me/llm-consent", stub))
	return e
}

func llmConsentRequest(t *testing.T, h http.Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, "/me/llm-consent", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLLMConsentController_Put(t *testing.T) {
	tests := []struct {
		name       string
		withUser   bool
		body       string
		wantCall   bool
		callVer    string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{name: "Error: unauthenticated request is 401", withUser: false, body: `{"version":"1"}`, wantStatus: http.StatusUnauthorized, wantCode: string(apperr.CodeUnauthorized)},
		{name: "Edge: empty body is 400 INVALID_JSON", withUser: true, body: "", wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeInvalidJSON)},
		{name: "Edge: malformed JSON is 400 INVALID_JSON", withUser: true, body: `{"version":`, wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeInvalidJSON)},
		{name: "Edge: unknown field is 400 INVALID_JSON", withUser: true, body: `{"version":"1","extra":true}`, wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeInvalidJSON)},
		{name: "Edge: non-string version is 400 INVALID_JSON", withUser: true, body: `{"version":1}`, wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeInvalidJSON)},
		{name: "Edge: missing version reaches the service as empty and is 400 VALIDATION", withUser: true, body: `{}`, wantCall: true, callVer: "", serviceErr: apperr.NewError(apperr.CodeValidation, llmconsent.MsgVersionMismatch, nil), wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeValidation)},
		{name: "Edge: outdated version is 400 VALIDATION", withUser: true, body: `{"version":"0"}`, wantCall: true, callVer: "0", serviceErr: apperr.NewError(apperr.CodeValidation, llmconsent.MsgVersionMismatch, nil), wantStatus: http.StatusBadRequest, wantCode: string(apperr.CodeValidation)},
		{name: "Error: missing user is 404", withUser: true, body: `{"version":"1"}`, wantCall: true, callVer: "1", serviceErr: apperr.NewError(apperr.CodeNotFound, "user not found", nil), wantStatus: http.StatusNotFound, wantCode: string(apperr.CodeNotFound)},
		{name: "Error: unexpected failure is 500", withUser: true, body: `{"version":"1"}`, wantCall: true, callVer: "1", serviceErr: assertErr{}, wantStatus: http.StatusInternalServerError, wantCode: string(apperr.CodeInternal)},
		{name: "Happy path: current version is recorded with 204", withUser: true, body: `{"version":"1"}`, wantCall: true, callVer: "1", wantStatus: http.StatusNoContent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := mocks.NewMockService(t)
			if tc.wantCall {
				service.EXPECT().Grant(mock.Anything, "user-1", tc.callVer).Return(tc.serviceErr).Once()
			}
			rec := llmConsentRequest(t, newLLMConsentHandler(t, service, tc.withUser), http.MethodPut, tc.body)

			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			if tc.wantCode != "" {
				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tc.wantCode, body.Error.Code)
			}
		})
	}
}

func TestLLMConsentController_Revoke(t *testing.T) {
	tests := []struct {
		name       string
		withUser   bool
		serviceErr error
		wantCall   bool
		wantStatus int
	}{
		{name: "Error: unauthenticated request is 401", withUser: false, wantStatus: http.StatusUnauthorized},
		{name: "Edge: revoke without consent still answers 204", withUser: true, wantCall: true, wantStatus: http.StatusNoContent},
		{name: "Error: unexpected failure is 500", withUser: true, wantCall: true, serviceErr: assertErr{}, wantStatus: http.StatusInternalServerError},
		{name: "Happy path: revoke answers 204", withUser: true, wantCall: true, wantStatus: http.StatusNoContent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := mocks.NewMockService(t)
			if tc.wantCall {
				service.EXPECT().Revoke(mock.Anything, "user-1").Return(tc.serviceErr).Once()
			}
			rec := llmConsentRequest(t, newLLMConsentHandler(t, service, tc.withUser), http.MethodDelete, "")

			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
		})
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "store unavailable" }

func TestRouterWiring_LLMConsentRequiresBearer(t *testing.T) {
	h, _ := newWiringRouter(t)

	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "Edge: PUT without a credential is 401", method: http.MethodPut, body: `{"version":"1"}`},
		{name: "Edge: DELETE without a credential is 401", method: http.MethodDelete},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doAuthRequest(t, h, "", tc.method, "/me/llm-consent", tc.body)
			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		})
	}
}
