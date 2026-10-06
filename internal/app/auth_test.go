package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/internal/app"
	"legnaris/internal/auth"
	authmocks "legnaris/internal/auth/mocks"
	"legnaris/internal/user"
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

func newAuthRouter(t *testing.T, service *authmocks.MockService) http.Handler {
	t.Helper()

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	app.NewAuthController(service).RegisterRoutes(e.Group("/auth"))
	return e
}

func TestAuthController_Register(t *testing.T) {
	t.Run("Happy path: 201 with the created user and no password", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, auth.RegisterCommand{Email: "player@example.com", Username: "player", Password: "sup3r-secret"}).
			Return(&user.User{ID: "user-1", Email: "player@example.com", Username: "player", CreatedAt: time.Unix(0, 0).UTC()}, nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusCreated, rec.Code)

		var body response.Success[user.UserDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "user-1", body.Data.ID)
		assert.Equal(t, "player@example.com", body.Data.Email)
		assert.NotContains(t, rec.Body.String(), "password")
	})

	t.Run("Error: invalid body returns 400", func(t *testing.T) {
		service := authmocks.NewMockService(t) // no expectations: the body never reaches the service

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_JSON", body.Error.Code)
	})

	t.Run("Error: duplicate email maps to 409", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeConflict, auth.MsgEmailAlreadyExists, nil)).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestAuthController_Login(t *testing.T) {
	t.Run("Happy path: 200 with the token pair and no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Login(mock.Anything, auth.LoginCommand{Email: "player@example.com", Password: "sup3r-secret"}).
			Return(&auth.TokenPair{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 900}, nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"), "native clients must not receive a refresh cookie")

		var body response.Success[auth.TokenPair]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "access", body.Data.AccessToken)
		assert.Equal(t, "refresh", body.Data.RefreshToken)
	})

	t.Run("Error: invalid credentials map to 401", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Login(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgInvalidCredentials, nil)).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"wrong"}`)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAuthController_Refresh(t *testing.T) {
	t.Run("Happy path: reads the X-Refresh-Token header and returns a new pair", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, auth.RefreshCommand{RefreshToken: "old-refresh"}).
			Return(&auth.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh", TokenType: "Bearer", ExpiresIn: 900}, nil).
			Once()

		h := newAuthRouter(t, service)

		req := newRequest(http.MethodPost, "/auth/refresh", "")
		req.Header.Set(app.RefreshTokenHeader, "old-refresh")
		rec := serve(t, h, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))

		var body response.Success[auth.TokenPair]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "new-refresh", body.Data.RefreshToken)
	})

	t.Run("Error: missing header reaches the service empty and maps to 401", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, auth.RefreshCommand{RefreshToken: ""}).
			Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgRefreshTokenRequired, nil)).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/refresh", "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAuthController_Logout(t *testing.T) {
	t.Run("Happy path: revokes via the header and returns 204", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Logout(mock.Anything, auth.LogoutCommand{RefreshToken: "refresh"}).
			Return(nil).
			Once()

		h := newAuthRouter(t, service)

		req := newRequest(http.MethodPost, "/auth/logout", "")
		req.Header.Set(app.RefreshTokenHeader, "refresh")
		rec := serve(t, h, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})

	t.Run("Edge: missing header is still 204", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Logout(mock.Anything, auth.LogoutCommand{RefreshToken: ""}).
			Return(nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/logout", "")
		require.Equal(t, http.StatusNoContent, rec.Code)
	})
}
