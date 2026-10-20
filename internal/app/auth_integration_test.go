package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/internal/auth"
	"legnaris/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	authEmail    = "player@example.com"
	authUsername = "player"
	authPassword = "sup3r-secret"

	// integrationOrigin is the frontend origin in the default allowlist; the
	// origin guard on refresh/logout requires it (AUTH-13).
	integrationOrigin = "http://localhost:5173"
)

// authCookies carries the two cookies a session response sets, so a test can
// replay them on the next cookie-authenticated call.
type authCookies struct {
	refresh string
	csrf    string
}

func cookiesFrom(t *testing.T, rec *httptest.ResponseRecorder) authCookies {
	t.Helper()

	var found authCookies
	for _, cookie := range rec.Result().Cookies() {
		switch cookie.Name {
		case auth.RefreshCookieName:
			found.refresh = cookie.Value
		case auth.CSRFCookieName:
			found.csrf = cookie.Value
		}
	}
	return found
}

func decodeSession(t *testing.T, rec *httptest.ResponseRecorder) auth.SessionResponse {
	t.Helper()

	var body response.Success[auth.SessionResponse]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

// authPost drives a cookie-authenticated route with the allowlisted origin and
// the given cookies, echoing csrf (falling back to the cookie's value).
func authPost(t *testing.T, h http.Handler, path string, cookies authCookies, csrf string) *httptest.ResponseRecorder {
	t.Helper()

	req := newRequest(http.MethodPost, path, "")
	req.Header.Set("Origin", integrationOrigin)
	if cookies.refresh != "" {
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: cookies.refresh})
	}
	if csrf == "" {
		csrf = cookies.csrf
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	return serve(t, h, req)
}

// registerAuthUser signs a new account in and returns its cookies. The caller
// passes the email so a single router can host several distinct users (each test
// drops the collections once, then registrations accumulate within it).
func registerAuthUser(t *testing.T, h http.Handler, email, username string) authCookies {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"username":%q,"password":%q}`, email, username, authPassword)
	rec := doRequest(t, h, http.MethodPost, "/auth/register", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	session := decodeSession(t, rec)
	assert.NotEmpty(t, session.AccessToken)
	assert.NotContains(t, rec.Body.String(), "refresh_token", "the credential must never reach the body")

	cookies := cookiesFrom(t, rec)
	require.NotEmpty(t, cookies.refresh, "registration must set the refresh cookie")
	require.NotEmpty(t, cookies.csrf, "registration must set the CSRF cookie")
	return cookies
}

func TestIntegration_AuthRegister(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	t.Run("Happy path: registration signs in with a session body and both cookies", func(t *testing.T) {
		cookies := registerAuthUser(t, h, authEmail, authUsername)
		assert.NotEqual(t, cookies.refresh, cookies.csrf)
	})

	t.Run("Error: duplicate email returns 409", func(t *testing.T) {
		const dupEmail = "dup@example.com"

		first := fmt.Sprintf(`{"email":%q,"username":"dup-one","password":%q}`, dupEmail, authPassword)
		require.Equal(t, http.StatusCreated, doRequest(t, h, http.MethodPost, "/auth/register", first).Code)

		second := fmt.Sprintf(`{"email":%q,"username":"dup-two","password":%q}`, dupEmail, authPassword)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", second)
		require.Equal(t, http.StatusConflict, rec.Code)

		var errBody response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
		assert.Equal(t, "CONFLICT", errBody.Error.Code)
	})

	t.Run("Error: short password returns 400", func(t *testing.T) {
		body := fmt.Sprintf(`{"email":%q,"username":%q,"password":"short"}`, authEmail, authUsername)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", body)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Error: password over the 72-byte bcrypt limit returns 400", func(t *testing.T) {
		body := fmt.Sprintf(
			`{"email":"overlimit@example.com","username":"overlimit","password":%q}`,
			strings.Repeat("a", 73),
		)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

		var errBody response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
		assert.Equal(t, auth.MsgPasswordTooLong, errBody.Error.Message)
	})
}

func TestIntegration_AuthLogin(t *testing.T) {
	h, _ := newIntegrationRouter(t)
	registerAuthUser(t, h, authEmail, authUsername)

	t.Run("Happy path: valid credentials set the cookies and return a parseable token", func(t *testing.T) {
		body := fmt.Sprintf(`{"email":%q,"password":%q}`, authEmail, authPassword)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		session := decodeSession(t, rec)
		require.NotEmpty(t, session.AccessToken)

		cookies := cookiesFrom(t, rec)
		require.NotEmpty(t, cookies.refresh)
		require.NotEmpty(t, cookies.csrf)

		manager, err := auth.NewTokenManager("integration-test-secret", 15*time.Minute, time.Now)
		require.NoError(t, err)

		userID, err := manager.Parse(session.AccessToken)
		require.NoError(t, err)
		assert.NotEmpty(t, userID)
	})

	t.Run("Error: wrong password returns a generic 401", func(t *testing.T) {
		body := fmt.Sprintf(`{"email":%q,"password":"wrong-password"}`, authEmail)
		rec := doRequest(t, h, http.MethodPost, "/auth/login", body)
		require.Equal(t, http.StatusUnauthorized, rec.Code)

		var errBody response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
		assert.Equal(t, auth.MsgInvalidCredentials, errBody.Error.Message)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})

	t.Run("Error: unknown email returns the same generic 401", func(t *testing.T) {
		body := `{"email":"nobody@example.com","password":"sup3r-secret"}`
		rec := doRequest(t, h, http.MethodPost, "/auth/login", body)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestIntegration_AuthRefreshRotation(t *testing.T) {
	h, _ := newIntegrationRouter(t)
	cookies := registerAuthUser(t, h, authEmail, authUsername)

	t.Run("Happy path: refresh rotates both the refresh and CSRF values", func(t *testing.T) {
		rec := authPost(t, h, "/auth/refresh", cookies, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		session := decodeSession(t, rec)
		require.NotEmpty(t, session.AccessToken)
		assert.NotContains(t, rec.Body.String(), "refresh_token")

		rotated := cookiesFrom(t, rec)
		require.NotEmpty(t, rotated.refresh)
		assert.NotEqual(t, cookies.refresh, rotated.refresh, "the credential must rotate")
		assert.NotEqual(t, cookies.csrf, rotated.csrf, "the CSRF token must rotate with the session")

		t.Run("Error: reusing the rotated credential revokes the whole family", func(t *testing.T) {
			reuse := authPost(t, h, "/auth/refresh", cookies, "")
			require.Equal(t, http.StatusUnauthorized, reuse.Code)

			// The family is gone, so the successor minted by the rotation is dead too.
			afterReuse := authPost(t, h, "/auth/refresh", rotated, "")
			require.Equal(t, http.StatusUnauthorized, afterReuse.Code)
		})
	})

	t.Run("Error: refresh with no cookie returns 401", func(t *testing.T) {
		rec := authPost(t, h, "/auth/refresh", authCookies{}, "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Error: refresh with no CSRF token returns 403 and rotates nothing", func(t *testing.T) {
		fresh := registerAuthUser(t, h, "fresh-user@example.com", "fresh-user")

		req := newRequest(http.MethodPost, "/auth/refresh", "")
		req.Header.Set("Origin", integrationOrigin)
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: fresh.refresh})
		rec := serve(t, h, req)

		require.Equal(t, http.StatusForbidden, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})
}

func TestIntegration_AuthLogout(t *testing.T) {
	h, _ := newIntegrationRouter(t)
	cookies := registerAuthUser(t, h, authEmail, authUsername)

	t.Run("Happy path: logout clears both cookies and revokes the family", func(t *testing.T) {
		rec := authPost(t, h, "/auth/logout", cookies, "")
		require.Equal(t, http.StatusNoContent, rec.Code)

		cleared := cookiesFrom(t, rec)
		assert.Empty(t, cleared.refresh, "the refresh cookie must be cleared")
		assert.Empty(t, cleared.csrf, "the CSRF cookie must be cleared")
		assert.NotEmpty(t, rec.Header().Get("Set-Cookie"))

		afterLogout := authPost(t, h, "/auth/refresh", cookies, "")
		require.Equal(t, http.StatusUnauthorized, afterLogout.Code)
	})

	t.Run("Edge: logout without a cookie is a 204 no-op", func(t *testing.T) {
		rec := authPost(t, h, "/auth/logout", authCookies{}, "")
		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})

	t.Run("Edge: logout with an already rotated credential still kills the family", func(t *testing.T) {
		current := registerAuthUser(t, h, "rotated-user@example.com", "rotated-user")

		rotatedRec := authPost(t, h, "/auth/refresh", current, "")
		require.Equal(t, http.StatusOK, rotatedRec.Code, rotatedRec.Body.String())
		rotated := cookiesFrom(t, rotatedRec)

		logoutRec := authPost(t, h, "/auth/logout", current, "")
		require.Equal(t, http.StatusNoContent, logoutRec.Code)

		afterLogout := authPost(t, h, "/auth/refresh", rotated, "")
		require.Equal(t, http.StatusUnauthorized, afterLogout.Code)
	})
}

// TestIntegration_AuthRefreshRequiresAccount proves renewal resolves the account
// end to end: a deleted account answers 401 with no cookie and a dead family,
// while a present account yields a token naming that account (PRQ-05, PRQ-06,
// SC-003).
func TestIntegration_AuthRefreshRequiresAccount(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	ctx := context.Background()

	t.Run("Happy path: a present account refreshes and the access token names it", func(t *testing.T) {
		const email = "present-account@example.com"
		cookies := registerAuthUser(t, h, email, "present-account")

		var stored struct {
			ID string `bson:"_id"`
		}
		require.NoError(t, mdb.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&stored))
		require.NotEmpty(t, stored.ID)

		rec := authPost(t, h, "/auth/refresh", cookies, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		session := decodeSession(t, rec)
		manager, err := auth.NewTokenManager("integration-test-secret", 15*time.Minute, time.Now)
		require.NoError(t, err)

		subject, err := manager.Parse(session.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, stored.ID, subject, "the renewed token must name the loaded account")
	})

	t.Run("Error: a deleted account is 401, sets no cookie and revokes the family", func(t *testing.T) {
		const email = "deleted-account@example.com"
		cookies := registerAuthUser(t, h, email, "deleted-account")

		_, err := mdb.Collection("users").DeleteOne(ctx, bson.M{"email": email})
		require.NoError(t, err)

		rec := authPost(t, h, "/auth/refresh", cookies, "")
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		assert.Empty(t, rec.Header().Get("Set-Cookie"))

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
		assert.Equal(t, auth.MsgInvalidRefreshToken, body.Error.Message)

		replay := authPost(t, h, "/auth/refresh", cookies, "")
		require.Equal(t, http.StatusUnauthorized, replay.Code, "the family must be revoked")
		assert.Empty(t, replay.Header().Get("Set-Cookie"))
	})
}
