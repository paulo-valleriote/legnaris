package app_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/internal/app"
	"legnaris/internal/auth"
	"legnaris/internal/user"
	"legnaris/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	authEmail    = "player@example.com"
	authUsername = "player"
	authPassword = "sup3r-secret"
)

func registerAuthUser(t *testing.T, h http.Handler) {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"username":%q,"password":%q}`, authEmail, authUsername, authPassword)
	rec := doRequest(t, h, http.MethodPost, "/auth/register", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created response.Success[user.UserDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, authEmail, created.Data.Email)
	assert.NotContains(t, rec.Body.String(), "password")
}

func decodeTokenPair(t *testing.T, rec *httptest.ResponseRecorder) auth.TokenPair {
	t.Helper()

	var pair response.Success[auth.TokenPair]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pair))
	return pair.Data
}

func loginAuthUser(t *testing.T, h http.Handler) auth.TokenPair {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"password":%q}`, authEmail, authPassword)
	rec := doRequest(t, h, http.MethodPost, "/auth/login", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	pair := decodeTokenPair(t, rec)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Empty(t, rec.Header().Get("Set-Cookie"), "native clients must not receive a refresh cookie")

	return pair
}

func refreshWith(t *testing.T, h http.Handler, refreshToken string) *httptest.ResponseRecorder {
	t.Helper()

	req := newRequest(http.MethodPost, "/auth/refresh", "")
	if refreshToken != "" {
		req.Header.Set(app.RefreshTokenHeader, refreshToken)
	}
	return serve(t, h, req)
}

func TestIntegration_AuthRegister(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	t.Run("Happy path: unique registration returns 201 without the password", func(t *testing.T) {
		registerAuthUser(t, h)
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
	registerAuthUser(t, h)

	t.Run("Happy path: valid credentials return a parseable access token", func(t *testing.T) {
		pair := loginAuthUser(t, h)

		manager, err := auth.NewTokenManager("integration-test-secret", 15*time.Minute)
		require.NoError(t, err)

		userID, err := manager.Parse(pair.AccessToken)
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
	})

	t.Run("Error: unknown email returns the same generic 401", func(t *testing.T) {
		body := `{"email":"nobody@example.com","password":"sup3r-secret"}`
		rec := doRequest(t, h, http.MethodPost, "/auth/login", body)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestIntegration_AuthRefreshRotation(t *testing.T) {
	h, _ := newIntegrationRouter(t)
	registerAuthUser(t, h)
	pair := loginAuthUser(t, h)

	t.Run("Happy path: refresh rotates the token", func(t *testing.T) {
		rec := refreshWith(t, h, pair.RefreshToken)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		rotated := decodeTokenPair(t, rec)
		require.NotEmpty(t, rotated.AccessToken)
		require.NotEmpty(t, rotated.RefreshToken)
		assert.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)

		t.Run("Error: reusing the rotated token revokes the whole family", func(t *testing.T) {
			reuse := refreshWith(t, h, pair.RefreshToken)
			require.Equal(t, http.StatusUnauthorized, reuse.Code)

			// The family is gone, so the token minted by the rotation is dead too.
			afterReuse := refreshWith(t, h, rotated.RefreshToken)
			require.Equal(t, http.StatusUnauthorized, afterReuse.Code)
		})
	})

	t.Run("Error: missing header returns 401", func(t *testing.T) {
		rec := refreshWith(t, h, "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestIntegration_AuthLogout(t *testing.T) {
	h, _ := newIntegrationRouter(t)
	registerAuthUser(t, h)
	pair := loginAuthUser(t, h)

	t.Run("Happy path: logout revokes the refresh token", func(t *testing.T) {
		req := newRequest(http.MethodPost, "/auth/logout", "")
		req.Header.Set(app.RefreshTokenHeader, pair.RefreshToken)
		rec := serve(t, h, req)
		require.Equal(t, http.StatusNoContent, rec.Code)

		afterLogout := refreshWith(t, h, pair.RefreshToken)
		require.Equal(t, http.StatusUnauthorized, afterLogout.Code)
	})

	t.Run("Edge: logout without a token is still 204", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/auth/logout", "")
		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("Edge: logout with an already rotated token still kills the family", func(t *testing.T) {
		current := loginAuthUser(t, h)

		rotatedRec := refreshWith(t, h, current.RefreshToken)
		require.Equal(t, http.StatusOK, rotatedRec.Code, rotatedRec.Body.String())
		rotated := decodeTokenPair(t, rotatedRec)

		req := newRequest(http.MethodPost, "/auth/logout", "")
		req.Header.Set(app.RefreshTokenHeader, current.RefreshToken)
		rec := serve(t, h, req)
		require.Equal(t, http.StatusNoContent, rec.Code)

		afterLogout := refreshWith(t, h, rotated.RefreshToken)
		require.Equal(t, http.StatusUnauthorized, afterLogout.Code)
	})
}
