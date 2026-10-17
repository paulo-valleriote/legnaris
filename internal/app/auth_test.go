package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/internal/app"
	"legnaris/internal/auth"
	authmocks "legnaris/internal/auth/mocks"
	"legnaris/shared/apperr"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func serve(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// newAuthRouter mounts the four auth handlers without the guards: guard
// behaviour is proven white-box in origin_test.go and ratelimit_test.go, so the
// controller test isolates the cookie transport and the session body.
func newAuthRouter(t *testing.T, service *authmocks.MockService) http.Handler {
	t.Helper()

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler

	controller := app.NewAuthController(service)
	group := e.Group("/auth")
	group.POST("/register", controller.Register)
	group.POST("/login", controller.Login)
	group.POST("/refresh", controller.Refresh)
	group.POST("/logout", controller.Logout)
	return e
}

// findCookie returns the named Set-Cookie from a response, or nil.
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// session is a representative service result carrying a credential and a CSRF
// token the controller must move to cookies, never to the body.
func session() *auth.Session {
	return &auth.Session{
		AccessToken:   "access-token",
		ExpiresIn:     900,
		RefreshToken:  "raw-refresh",
		RefreshMaxAge: 720,
		CSRFToken:     "csrf-token",
		UserID:        "user-1",
	}
}

func assertSessionCookies(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	refresh := findCookie(rec, auth.RefreshCookieName)
	require.NotNil(t, refresh, "the refresh cookie must be set")
	assert.Equal(t, "raw-refresh", refresh.Value)
	assert.True(t, refresh.HttpOnly)
	assert.True(t, refresh.Secure)
	assert.Equal(t, http.SameSiteStrictMode, refresh.SameSite)
	assert.Equal(t, "/", refresh.Path)
	assert.Empty(t, refresh.Domain)
	assert.Equal(t, 720, refresh.MaxAge)

	csrf := findCookie(rec, auth.CSRFCookieName)
	require.NotNil(t, csrf, "the CSRF cookie must be set")
	assert.Equal(t, "csrf-token", csrf.Value)
	assert.False(t, csrf.HttpOnly, "the page must be able to read and echo the CSRF token")
	assert.True(t, csrf.Secure)
	assert.Equal(t, http.SameSiteStrictMode, csrf.SameSite)
	assert.Equal(t, "/", csrf.Path)
	assert.Equal(t, 720, csrf.MaxAge)
}

func assertSessionBody(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	var body response.Success[auth.SessionResponse]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "access-token", body.Data.AccessToken)
	assert.Equal(t, "Bearer", body.Data.TokenType)
	assert.Equal(t, 900, body.Data.ExpiresIn)

	raw := rec.Body.String()
	assert.NotContains(t, raw, "refresh_token", "the body must carry no refresh field")
	assert.NotContains(t, raw, "raw-refresh", "the credential must never reach the body")
	assert.NotContains(t, raw, "csrf-token", "the CSRF token travels only in the cookie")
}

func TestAuthController_Register(t *testing.T) {
	t.Run("Happy path: 201 with the session body and both cookies", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, auth.RegisterCommand{Email: "player@example.com", Username: "player", Password: "sup3r-secret"}).
			Return(session(), nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusCreated, rec.Code)

		assertSessionCookies(t, rec)
		assertSessionBody(t, rec)
	})

	t.Run("Error: invalid body returns 400", func(t *testing.T) {
		service := authmocks.NewMockService(t) // no expectations: the body never reaches the service

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_JSON", body.Error.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"), "a rejected request sets no cookie")
	})

	t.Run("Error: duplicate email maps to 409 and sets no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeConflict, auth.MsgEmailAlreadyExists, nil)).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})
}

func TestAuthController_Login(t *testing.T) {
	t.Run("Happy path: 200 with the session body and both cookies", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Login(mock.Anything, auth.LoginCommand{Email: "player@example.com", Password: "sup3r-secret"}).
			Return(session(), nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		assertSessionCookies(t, rec)
		assertSessionBody(t, rec)
	})

	t.Run("Error: invalid credentials map to 401 and set no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Login(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgInvalidCredentials, nil)).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"wrong"}`)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
		assert.NotContains(t, rec.Body.String(), "raw-refresh")
	})
}

func TestAuthController_Refresh(t *testing.T) {
	t.Run("Happy path: reads the cookie and returns a rotated session", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, auth.RefreshCommand{RefreshToken: "cookie-refresh", CSRFToken: "csrf-token"}).
			Return(session(), nil).
			Once()

		h := newAuthRouter(t, service)
		req := newRequest(http.MethodPost, "/auth/refresh", "")
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: "cookie-refresh"})
		req.Header.Set("X-CSRF-Token", "csrf-token")

		rec := serve(t, h, req)
		require.Equal(t, http.StatusOK, rec.Code)
		assertSessionCookies(t, rec)
		assertSessionBody(t, rec)
	})

	t.Run("Edge: a credential supplied in X-Refresh-Token is ignored", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, auth.RefreshCommand{RefreshToken: "", CSRFToken: ""}).
			Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgRefreshTokenRequired, nil)).
			Once()

		h := newAuthRouter(t, service)
		req := newRequest(http.MethodPost, "/auth/refresh", `{"refresh_token":"body-refresh"}`)
		req.Header.Set("X-Refresh-Token", "header-refresh")

		rec := serve(t, h, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Error: no cookie is 401", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, auth.RefreshCommand{RefreshToken: ""}).
			Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgRefreshTokenRequired, nil)).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/refresh", "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})

	t.Run("Error: a refused CSRF token propagates the typed 403 untouched", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, auth.RefreshCommand{RefreshToken: "cookie-refresh", CSRFToken: "foreign-token"}).
			Return(nil, apperr.NewError(apperr.CodeForbidden, auth.MsgCSRFInvalid, nil)).
			Once()

		h := newAuthRouter(t, service)
		req := newRequest(http.MethodPost, "/auth/refresh", "")
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: "cookie-refresh"})
		req.Header.Set("X-CSRF-Token", "foreign-token")

		rec := serve(t, h, req)
		require.Equal(t, http.StatusForbidden, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, string(apperr.CodeForbidden), body.Error.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"), "a refused call changes no cookie")
	})

	t.Run("Edge: a CSRF header with no cookie is 401, not 403 (presence before CSRF)", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, auth.RefreshCommand{RefreshToken: "", CSRFToken: "csrf-token"}).
			Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgRefreshTokenRequired, nil)).
			Once()

		h := newAuthRouter(t, service)
		req := newRequest(http.MethodPost, "/auth/refresh", "")
		req.Header.Set("X-CSRF-Token", "csrf-token")

		rec := serve(t, h, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAuthController_Logout(t *testing.T) {
	t.Run("Happy path: revokes via the cookie, clears both cookies and returns 204", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Logout(mock.Anything, auth.LogoutCommand{RefreshToken: "cookie-refresh", CSRFToken: "csrf-token"}).
			Return(nil).
			Once()

		h := newAuthRouter(t, service)
		req := newRequest(http.MethodPost, "/auth/logout", "")
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: "cookie-refresh"})
		req.Header.Set("X-CSRF-Token", "csrf-token")

		rec := serve(t, h, req)
		require.Equal(t, http.StatusNoContent, rec.Code)

		clearedRefresh := findCookie(rec, auth.RefreshCookieName)
		require.NotNil(t, clearedRefresh)
		assert.Empty(t, clearedRefresh.Value)
		// Max-Age=0 round-trips to MaxAge=-1 through http.ReadSetCookies.
		assert.Equal(t, -1, clearedRefresh.MaxAge)
		assert.True(t, clearedRefresh.HttpOnly)
		assert.Contains(t, rec.Header().Values("Set-Cookie")[0], "Max-Age=0")

		clearedCSRF := findCookie(rec, auth.CSRFCookieName)
		require.NotNil(t, clearedCSRF)
		assert.Empty(t, clearedCSRF.Value)
		assert.Equal(t, -1, clearedCSRF.MaxAge)
	})

	t.Run("Edge: no cookie is a 204 no-op that sets nothing", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Logout(mock.Anything, auth.LogoutCommand{RefreshToken: ""}).
			Return(nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/logout", "")
		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})

	t.Run("Edge: a credential supplied in X-Refresh-Token is ignored", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Logout(mock.Anything, auth.LogoutCommand{RefreshToken: "", CSRFToken: ""}).
			Return(nil).
			Once()

		h := newAuthRouter(t, service)
		req := newRequest(http.MethodPost, "/auth/logout", "")
		req.Header.Set("X-Refresh-Token", "header-refresh")

		rec := serve(t, h, req)
		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"), "an inert header changes no outcome")
	})

	t.Run("Error: a refused CSRF token propagates the typed 403 and clears nothing", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Logout(mock.Anything, auth.LogoutCommand{RefreshToken: "cookie-refresh"}).
			Return(apperr.NewError(apperr.CodeForbidden, auth.MsgCSRFInvalid, nil)).
			Once()

		h := newAuthRouter(t, service)
		req := newRequest(http.MethodPost, "/auth/logout", "")
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: "cookie-refresh"})

		rec := serve(t, h, req)
		require.Equal(t, http.StatusForbidden, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})
}
