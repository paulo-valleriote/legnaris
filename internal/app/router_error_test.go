package app_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"legnaris/internal/app"
	"legnaris/shared/apperr"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newErrorHandlerApp mounts a single route over the production HTTPErrorHandler,
// so a handler can return an arbitrary error and the boundary translation is
// observable without routing or middleware in the way.
func newErrorHandlerApp(h echo.HandlerFunc) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.GET("/boom", h)
	return e
}

// TestHTTPErrorHandler_RoutingErrors covers LGN-48 at the router boundary: Echo
// reports an unmatched route and a known path reached with the wrong method as
// typed HTTP-status errors rather than apperr, so untranslated they answered the
// 500 fallback. These requests take the real router, because the status comes
// from route resolution, not from a handler.
func TestHTTPErrorHandler_RoutingErrors(t *testing.T) {
	h, _ := newWiringRouter(t)

	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "Edge: an unmatched route returns 404",
			method:      http.MethodGet,
			path:        "/no-such-route",
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "not found",
		},
		{
			name:        "Edge: the API root has no handler and returns 404",
			method:      http.MethodGet,
			path:        "/",
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "not found",
		},
		{
			name:        "Edge: a nested unmatched route returns 404",
			method:      http.MethodGet,
			path:        "/no/such/route",
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "not found",
		},
		{
			name:        "Edge: an unmatched sub-path of a public route returns 404",
			method:      http.MethodGet,
			path:        "/health/extra",
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "not found",
		},
		{
			name:        "Edge: a known path with the wrong method returns 405",
			method:      http.MethodPost,
			path:        "/health",
			wantStatus:  http.StatusMethodNotAllowed,
			wantCode:    "METHOD_NOT_ALLOWED",
			wantMessage: "method not allowed",
		},
		{
			name:        "Edge: another known path with the wrong method returns 405",
			method:      http.MethodDelete,
			path:        "/metrics",
			wantStatus:  http.StatusMethodNotAllowed,
			wantCode:    "METHOD_NOT_ALLOWED",
			wantMessage: "method not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, h, tc.method, tc.path, "")
			require.Equal(t, tc.wantStatus, rec.Code, "%s %s", tc.method, tc.path)
			require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
			assert.Equal(t, tc.wantMessage, body.Error.Message)
		})
	}
}

// TestHTTPErrorHandler_Translations pins how asAppErr classifies a handler
// error: an apperr keeps its own status, a status-coded error is translated to
// the matching apperr code, and everything else stays on the 500 safety net
// (mirroring apperr's TestHttpStatus_UnmappedCodeFallsBackTo500) so an
// unmapped status is never silently promoted to a client error.
func TestHTTPErrorHandler_Translations(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "Happy path: an apperr keeps its status and message",
			err:         apperr.NewError(apperr.CodeConflict, "sheet limit reached", nil),
			wantStatus:  http.StatusConflict,
			wantCode:    "CONFLICT",
			wantMessage: "sheet limit reached",
		},
		{
			name:        "Edge: a status-coded error is translated to its apperr code",
			err:         echo.ErrForbidden,
			wantStatus:  http.StatusForbidden,
			wantCode:    "FORBIDDEN",
			wantMessage: "forbidden",
		},
		{
			name:        "Edge: a bare error returns 500 without leaking the cause",
			err:         errors.New("connection reset by peer"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "INTERNAL_ERROR",
			wantMessage: "internal server error",
		},
		{
			name:        "Edge: a status without an apperr counterpart keeps the 500 fallback",
			err:         echo.ErrUnsupportedMediaType,
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "INTERNAL_ERROR",
			wantMessage: "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newErrorHandlerApp(func(c *echo.Context) error {
				return tc.err
			})

			rec := doRequest(t, h, http.MethodGet, "/boom", "")
			require.Equal(t, tc.wantStatus, rec.Code)

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
			assert.Equal(t, tc.wantMessage, body.Error.Message)
			assert.NotContains(t, rec.Body.String(), "connection reset by peer")
		})
	}
}
