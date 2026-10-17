package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"legnaris/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const allowedOrigin = "https://app.legnaris.com.br"

// newOriginRouter mounts the origin guard on the two cookie-authenticated routes
// and an unguarded bearer route, mirroring router.go's scoping (AUTH-26).
func newOriginRouter() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler

	reached := func(c *echo.Context) error { return c.String(http.StatusOK, "reached") }
	guard := requireAllowedOrigin([]string{allowedOrigin})

	e.POST("/auth/refresh", reached, guard)
	e.POST("/auth/logout", reached, guard)
	e.GET("/sheets", reached)
	return e
}

func TestRequireAllowedOrigin(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		origin   string
		referer  string
		wantCode int
	}{
		{
			name:     "Error: no Origin and no Referer is 403",
			method:   http.MethodPost,
			path:     "/auth/refresh",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "Error: logout with no Origin and no Referer is 403",
			method:   http.MethodPost,
			path:     "/auth/logout",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "Error: an unlisted Origin is 403",
			method:   http.MethodPost,
			path:     "/auth/refresh",
			origin:   "https://evil.example.com",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "Error: an unlisted Referer is 403",
			method:   http.MethodPost,
			path:     "/auth/refresh",
			referer:  "https://evil.example.com/page",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "Error: a malformed Referer without a scheme is 403",
			method:   http.MethodPost,
			path:     "/auth/logout",
			referer:  "not-a-url",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "Happy path: the allowlisted Origin passes",
			method:   http.MethodPost,
			path:     "/auth/refresh",
			origin:   allowedOrigin,
			wantCode: http.StatusOK,
		},
		{
			name:     "Happy path: an Origin match is case-insensitive",
			method:   http.MethodPost,
			path:     "/auth/refresh",
			origin:   "HTTPS://APP.LEGNARIS.COM.BR",
			wantCode: http.StatusOK,
		},
		{
			name:     "Happy path: the allowlisted origin of a Referer passes",
			method:   http.MethodPost,
			path:     "/auth/logout",
			referer:  allowedOrigin + "/dashboard",
			wantCode: http.StatusOK,
		},
		{
			name:     "Edge: a bearer route with no Origin is not rejected",
			method:   http.MethodGet,
			path:     "/sheets",
			wantCode: http.StatusOK,
		},
		{
			name:     "Edge: a bearer route with an unlisted Origin is not rejected",
			method:   http.MethodGet,
			path:     "/sheets",
			origin:   "https://evil.example.com",
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newOriginRouter()

			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.origin != "" {
				req.Header.Set(echo.HeaderOrigin, tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tc.wantCode, rec.Code, rec.Body.String())

			if tc.wantCode == http.StatusForbidden {
				var body struct {
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, string(apperr.CodeForbidden), body.Error.Code)
				assert.Equal(t, MsgOriginNotAllowed, body.Error.Message)
			}
		})
	}
}
