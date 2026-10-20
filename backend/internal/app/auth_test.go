package app_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	authmocks "legnaris/backend/internal/auth/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

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

// sessionOutcome is the service result of a legacy registration or sign-in:
// the established session and no code step (TFA-07, TFA-12).
func sessionOutcome() *auth.StartOutcome {
	return &auth.StartOutcome{Session: session()}
}

// challengeOutcome is the service result of a gated registration or sign-in:
// the started code step and no session (TFA-01, TFA-02).
func challengeOutcome() *auth.StartOutcome {
	return &auth.StartOutcome{Challenge: &auth.ChallengeIssued{
		ChallengeID: "01900000-0000-7000-8000-000000000009",
		ExpiresIn:   600,
		ResendAfter: 60,
	}}
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

// assertChallengeBody pins the gated answer: the challenge handle, its
// lifetime and the resend window, with no credential of any kind in the body
// and no cookie in the headers (TFA-01, TFA-02, SC-001).
func assertChallengeBody(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	var body response.Success[auth.ChallengeIssued]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "01900000-0000-7000-8000-000000000009", body.Data.ChallengeID)
	assert.Equal(t, 600, body.Data.ExpiresIn)
	assert.Equal(t, 60, body.Data.ResendAfter)

	raw := rec.Body.String()
	assert.NotContains(t, raw, "access_token", "the body must carry no access credential")
	assert.NotContains(t, raw, "refresh_token", "the body must carry no refresh credential")
	assert.NotContains(t, raw, "csrf", "the CSRF token travels only in the cookie")
	assert.Empty(t, rec.Header().Get("Set-Cookie"), "a code-step answer sets no cookie")
}

func TestAuthController_Register(t *testing.T) {
	t.Run("Happy path: 201 with the session body and both cookies", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, auth.RegisterCommand{Email: "player@example.com", Username: "player", Password: "sup3r-secret"}).
			Return(sessionOutcome(), nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusCreated, rec.Code)

		assertSessionCookies(t, rec)
		assertSessionBody(t, rec)
	})

	t.Run("Happy path: the gated registration answers 201 with the challenge and no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, mock.Anything).
			Return(challengeOutcome(), nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusCreated, rec.Code)

		assertChallengeBody(t, rec)
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

	t.Run("Error: the spent daily allowance answers 503 with a retry hint and no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeEmailUnavailable, auth.MsgEmailUnavailable, nil).WithRetryAfter(90*time.Second)).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "90", rec.Header().Get("Retry-After"))
		assert.Empty(t, rec.Header().Get("Set-Cookie"))

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, string(apperr.CodeEmailUnavailable), body.Error.Code)
	})

	t.Run("Error: a store failure while starting the code step answers 500", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Register(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeInternal, auth.MsgInternal, errors.New("mongo down"))).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, string(apperr.CodeInternal), body.Error.Code)
	})
}

func TestAuthController_Login(t *testing.T) {
	t.Run("Happy path: 200 with the session body and both cookies", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Login(mock.Anything, auth.LoginCommand{Email: "player@example.com", Password: "sup3r-secret"}).
			Return(sessionOutcome(), nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusOK, rec.Code)

		assertSessionCookies(t, rec)
		assertSessionBody(t, rec)
	})

	t.Run("Happy path: the gated sign-in answers 202 with the challenge and no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Login(mock.Anything, mock.Anything).
			Return(challengeOutcome(), nil).
			Once()

		h := newAuthRouter(t, service)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"sup3r-secret"}`)
		require.Equal(t, http.StatusAccepted, rec.Code)

		assertChallengeBody(t, rec)
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

// TestAuthController_CredentialFailuresAreUniform pins the one uniform
// credential failure: the four ways a sign-in can fail answer the same status
// and the same body, so no caller can tell them apart (TFA-05, SC-002).
func TestAuthController_CredentialFailuresAreUniform(t *testing.T) {
	failures := []struct {
		name string
		body string
	}{
		{name: "unknown address", body: `{"email":"nobody@example.com","password":"sup3r-secret"}`},
		{name: "wrong password", body: `{"email":"player@example.com","password":"wrong"}`},
		{name: "unparseable address", body: `{"email":"not-an-email","password":"sup3r-secret"}`},
		{name: "over-long address", body: `{"email":"` + strings.Repeat("a", 250) + `@example.com","password":"sup3r-secret"}`},
	}

	var bodies []string
	for _, tc := range failures {
		t.Run("Edge: "+tc.name+" answers the one uniform 401", func(t *testing.T) {
			service := authmocks.NewMockService(t)
			service.EXPECT().
				Login(mock.Anything, mock.Anything).
				Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgInvalidCredentials, nil)).
				Once()

			h := newAuthRouter(t, service)
			rec := doRequest(t, h, http.MethodPost, "/auth/login", tc.body)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Empty(t, rec.Header().Get("Set-Cookie"))

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, string(apperr.CodeUnauthorized), body.Error.Code)
			assert.Equal(t, auth.MsgInvalidCredentials, body.Error.Message)

			bodies = append(bodies, rec.Body.String())
		})
	}

	t.Run("Edge: every credential failure answered byte-identically", func(t *testing.T) {
		require.Len(t, bodies, len(failures))
		for i := 1; i < len(bodies); i++ {
			assert.Equal(t, bodies[0], bodies[i],
				"failure %q must be indistinguishable from %q", failures[i].name, failures[0].name)
		}
	})
}

// TestAuthController_GatedAnswersDiscloseNothing sweeps the bodies and headers
// of the two gated answers: no six-digit code and no address may appear in
// either (TFA-16, SC-001).
func TestAuthController_GatedAnswersDiscloseNothing(t *testing.T) {
	for _, route := range []string{"/auth/register", "/auth/login"} {
		t.Run("Edge: "+route+" discloses neither a code nor an address", func(t *testing.T) {
			service := authmocks.NewMockService(t)
			service.EXPECT().
				Register(mock.Anything, mock.Anything).
				Return(challengeOutcome(), nil).
				Maybe()
			service.EXPECT().
				Login(mock.Anything, mock.Anything).
				Return(challengeOutcome(), nil).
				Maybe()

			h := newAuthRouter(t, service)
			rec := doRequest(t, h, http.MethodPost, route, `{"email":"player@example.com","username":"player","password":"sup3r-secret"}`)
			require.NotEqual(t, http.StatusUnauthorized, rec.Code)

			assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), rec.Body.String())
			assert.NotContains(t, rec.Body.String(), "player@example.com")
			for key, values := range rec.Header() {
				for _, value := range values {
					assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), value, "header %s must carry no code", key)
					assert.NotContains(t, value, "@", "header %s must carry no address", key)
				}
			}
		})
	}
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

// TestAuthController_RefreshAccountFailures pins the HTTP mapping of the new
// renewal failures: a missing account is 401 with no cookie, a store failure is
// 500 with no cookie, and neither changes the response body shape (PRQ-07,
// PRQ-14, PRQ-17).
func TestAuthController_RefreshAccountFailures(t *testing.T) {
	refreshRequest := func(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
		t.Helper()

		req := newRequest(http.MethodPost, "/auth/refresh", "")
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: "cookie-refresh"})
		req.Header.Set("X-CSRF-Token", "csrf-token")
		return serve(t, h, req)
	}

	t.Run("Error: a missing account is 401 and sets no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeUnauthorized, auth.MsgInvalidRefreshToken, nil)).
			Once()

		rec := refreshRequest(t, newAuthRouter(t, service))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, string(apperr.CodeUnauthorized), body.Error.Code)
		assert.Equal(t, auth.MsgInvalidRefreshToken, body.Error.Message)
		assert.NotContains(t, rec.Body.String(), "access_token")
	})

	t.Run("Error: an account-store failure is 500 and sets no cookie", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			Refresh(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeInternal, auth.MsgInternal, errors.New("mongo down"))).
			Once()

		rec := refreshRequest(t, newAuthRouter(t, service))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, string(apperr.CodeInternal), body.Error.Code)
		assert.Equal(t, auth.MsgInternal, body.Error.Message)
		assert.NotContains(t, rec.Body.String(), "access_token")
	})
}
