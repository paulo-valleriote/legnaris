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

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
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

// registerAuthUser registers a new account and verifies its address with the
// code its own challenge carries, so the call returns the cookies of the
// account's first verified session. The caller passes the email so a single
// router can host several distinct users.
func registerAuthUser(t *testing.T, h http.Handler, mdb *mongo.Database, email, username string) authCookies {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"username":%q,"password":%q}`, email, username, authPassword)
	rec := doRequest(t, h, http.MethodPost, "/auth/register", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Set-Cookie"), "registration sets no cookie")

	var issued response.Success[auth.ChallengeIssued]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &issued))

	verifyRec := doRequest(t, h, http.MethodPost, verifyRoute,
		`{"challenge_id":"`+issued.Data.ChallengeID+`","code":"`+codeForChallenge(t, mdb, issued.Data.ChallengeID)+`"}`)
	require.Equal(t, http.StatusOK, verifyRec.Code, verifyRec.Body.String())

	cookies := cookiesFrom(t, verifyRec)
	require.NotEmpty(t, cookies.refresh, "the verified session must set the refresh cookie")
	require.NotEmpty(t, cookies.csrf, "the verified session must set the CSRF cookie")
	return cookies
}

// TestIntegration_RegistrationVerification drives the registration journey end
// to end: the registration answers a challenge and no session, and the code the
// recorded intention carries verifies it into the account's first session and
// records the verification timestamp (TFA-01, TFA-04, TFA-11, TFA-13).
func TestIntegration_RegistrationVerification(t *testing.T) {
	h, mdb, _ := newIntegrationStack(t, nil)
	ctx := context.Background()

	const email = "gated-register@example.com"
	registerBody := fmt.Sprintf(`{"email":%q,"username":"gated","password":%q}`, email, authPassword)

	registerRec := doRequest(t, h, http.MethodPost, "/auth/register", registerBody)
	require.Equal(t, http.StatusCreated, registerRec.Code, registerRec.Body.String())
	assert.Empty(t, registerRec.Header().Get("Set-Cookie"), "the registration sets no cookie")
	assert.NotContains(t, registerRec.Body.String(), "access_token")

	var issued response.Success[auth.ChallengeIssued]
	require.NoError(t, json.Unmarshal(registerRec.Body.Bytes(), &issued))
	require.NotEmpty(t, issued.Data.ChallengeID)
	assert.Equal(t, 600, issued.Data.ExpiresIn)
	assert.Equal(t, 60, issued.Data.ResendAfter)

	userID := userIDByEmail(t, mdb, email)
	var stored struct {
		VerifiedAt *time.Time `bson:"email_verified_at"`
	}
	require.NoError(t, mdb.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&stored))
	assert.Nil(t, stored.VerifiedAt, "the registration leaves the account unverified")

	code := pendingCode(t, mdb)

	t.Run("Happy path: verifying the code establishes the account's first session", func(t *testing.T) {
		verifyRec := doRequest(t, h, http.MethodPost, verifyRoute,
			`{"challenge_id":"`+issued.Data.ChallengeID+`","code":"`+code+`"}`)
		require.Equal(t, http.StatusOK, verifyRec.Code, verifyRec.Body.String())

		session := decodeSession(t, verifyRec)
		require.NotEmpty(t, session.AccessToken)

		cookies := cookiesFrom(t, verifyRec)
		require.NotEmpty(t, cookies.refresh, "the verified session sets the refresh cookie")
		require.NotEmpty(t, cookies.csrf, "the verified session sets the CSRF cookie")

		require.NoError(t, mdb.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&stored))
		assert.NotNil(t, stored.VerifiedAt, "the verification records the timestamp")
	})

	t.Run("Happy path: a verified account signs in with a session", func(t *testing.T) {
		loginRec := doRequest(t, h, http.MethodPost, "/auth/login",
			fmt.Sprintf(`{"email":%q,"password":%q}`, email, authPassword))
		require.Equal(t, http.StatusOK, loginRec.Code, loginRec.Body.String())
		assert.NotEmpty(t, cookiesFrom(t, loginRec).refresh)
	})

	t.Run("Edge: a registration against the verified address answers 201 and only notifies the owner, once", func(t *testing.T) {
		before := outboxCount(t, mdb)

		dupRec := doRequest(t, h, http.MethodPost, "/auth/register", registerBody)
		require.Equal(t, http.StatusCreated, dupRec.Code, dupRec.Body.String())
		assert.Empty(t, dupRec.Header().Get("Set-Cookie"))

		var dup response.Success[auth.ChallengeIssued]
		require.NoError(t, json.Unmarshal(dupRec.Body.Bytes(), &dup))
		assert.NotEmpty(t, dup.Data.ChallengeID)
		assert.NotEqual(t, issued.Data.ChallengeID, dup.Data.ChallengeID)
		assert.Equal(t, before+1, outboxCount(t, mdb), "the owner gets one notice")

		againRec := doRequest(t, h, http.MethodPost, "/auth/register", registerBody)
		require.Equal(t, http.StatusCreated, againRec.Code, againRec.Body.String())
		assert.Equal(t, before+1, outboxCount(t, mdb), "a repeat inside the window sends nothing more")

		verifyRec := doRequest(t, h, http.MethodPost, verifyRoute,
			`{"challenge_id":"`+dup.Data.ChallengeID+`","code":"000000"}`)
		assert.GreaterOrEqual(t, verifyRec.Code, http.StatusBadRequest, "the decoy identity can never be redeemed")
	})
}

// TestIntegration_IdentityRead covers the identity read end to end: a verified
// account reports its verified state, and the read carries the profile fields
// the sign-in established (TFA-08, SC-006).
func TestIntegration_IdentityRead(t *testing.T) {
	h, mdb, _ := newIntegrationStack(t, nil)

	const email = "identity-read@example.com"
	cookies := registerAuthUser(t, h, mdb, email, "identity")

	rec := authPost(t, h, "/auth/refresh", cookies, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	session := decodeSession(t, rec)
	require.NotEmpty(t, session.AccessToken)

	identity := readIdentity(t, h, session.AccessToken)
	assert.True(t, identity.EmailVerified, "a verified account reports the verified state")
	assert.Equal(t, email, identity.Email)
	assert.Equal(t, "identity", identity.Username)
}

// readIdentity reads the signed-in account and returns the projected profile.
func readIdentity(t *testing.T, h http.Handler, token string) user.UserDTO {
	t.Helper()

	rec := doAuthRequest(t, h, token, http.MethodGet, "/auth/me", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body response.Success[user.UserDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotContains(t, rec.Body.String(), "email_verified_at", "the instant never reaches the wire")
	assert.NotContains(t, rec.Body.String(), "password", "the hash never reaches the wire")
	return body.Data
}

// TestIntegration_LoginUnverifiedAccount drives the sign-in of an account that
// registered but never verified: the password earns a challenge, not a session,
// and the code of that challenge verifies the account into its first session
// (D2, TFA-06).
func TestIntegration_LoginUnverifiedAccount(t *testing.T) {
	h, mdb, _ := newIntegrationStack(t, nil)

	const email = "unverified-login@example.com"
	registerBody := fmt.Sprintf(`{"email":%q,"username":"unverified","password":%q}`, email, authPassword)
	registerRec := doRequest(t, h, http.MethodPost, "/auth/register", registerBody)
	require.Equal(t, http.StatusCreated, registerRec.Code, registerRec.Body.String())

	loginBody := fmt.Sprintf(`{"email":%q,"password":%q}`, email, authPassword)
	loginRec := doRequest(t, h, http.MethodPost, "/auth/login", loginBody)
	require.Equal(t, http.StatusAccepted, loginRec.Code, loginRec.Body.String())
	assert.Empty(t, loginRec.Header().Get("Set-Cookie"), "an unverified sign-in sets no cookie")
	assert.NotContains(t, loginRec.Body.String(), "access_token")

	var issued response.Success[auth.ChallengeIssued]
	require.NoError(t, json.Unmarshal(loginRec.Body.Bytes(), &issued))
	require.NotEmpty(t, issued.Data.ChallengeID)

	verifyRec := doRequest(t, h, http.MethodPost, verifyRoute,
		`{"challenge_id":"`+issued.Data.ChallengeID+`","code":"`+codeForChallenge(t, mdb, issued.Data.ChallengeID)+`"}`)
	require.Equal(t, http.StatusOK, verifyRec.Code, verifyRec.Body.String())
	assert.NotEmpty(t, cookiesFrom(t, verifyRec).refresh)

	session := decodeSession(t, verifyRec)
	assert.True(t, readIdentity(t, h, session.AccessToken).EmailVerified)
}

// codeForChallenge reads the clear code of the send intention the challenge
// itself carries, so a test verifies the challenge it owns (OTP-12).
func codeForChallenge(t *testing.T, mdb *mongo.Database, challengeID string) string {
	t.Helper()

	ctx := context.Background()
	var challenge struct {
		LastIntentID string `bson:"last_intent_id"`
	}
	require.NoError(t, mdb.Collection("auth_challenges").FindOne(ctx, bson.M{"_id": challengeID}).Decode(&challenge))

	var intent struct {
		Payload struct {
			Code string `bson:"code"`
		} `bson:"payload"`
	}
	require.NoError(t, mdb.Collection("email_outbox").FindOne(ctx, bson.M{"_id": challenge.LastIntentID}).Decode(&intent))
	require.Regexp(t, codePattern, intent.Payload.Code)
	return intent.Payload.Code
}

func TestIntegration_AuthRegister(t *testing.T) {
	h, _ := newIntegrationRouter(t)

	t.Run("Happy path: registration answers a challenge, no session body and no cookie", func(t *testing.T) {
		body := fmt.Sprintf(`{"email":%q,"username":%q,"password":%q}`, authEmail, authUsername, authPassword)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
		assert.NotContains(t, rec.Body.String(), "access_token")

		var issued response.Success[auth.ChallengeIssued]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &issued))
		assert.NotEmpty(t, issued.Data.ChallengeID)
	})

	t.Run("Edge: re-registering a pending address replaces the account and answers 201", func(t *testing.T) {
		const dupEmail = "dup@example.com"

		first := fmt.Sprintf(`{"email":%q,"username":"dup-one","password":%q}`, dupEmail, authPassword)
		require.Equal(t, http.StatusCreated, doRequest(t, h, http.MethodPost, "/auth/register", first).Code)

		second := fmt.Sprintf(`{"email":%q,"username":"dup-two","password":%q}`, dupEmail, authPassword)
		require.Equal(t, http.StatusCreated, doRequest(t, h, http.MethodPost, "/auth/register", second).Code)
	})

	t.Run("Error: a username held by another account is 409 with the neutral message", func(t *testing.T) {
		first := fmt.Sprintf(`{"email":%q,"username":"shared-name","password":%q}`, "owner@example.com", authPassword)
		require.Equal(t, http.StatusCreated, doRequest(t, h, http.MethodPost, "/auth/register", first).Code)

		second := fmt.Sprintf(`{"email":%q,"username":"shared-name","password":%q}`, "other@example.com", authPassword)
		rec := doRequest(t, h, http.MethodPost, "/auth/register", second)
		require.Equal(t, http.StatusConflict, rec.Code)

		var errBody response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
		assert.Equal(t, "CONFLICT", errBody.Error.Code)
		assert.Equal(t, auth.MsgRegistrationConflict, errBody.Error.Message)
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
	h, mdb := newIntegrationRouter(t)
	registerAuthUser(t, h, mdb, authEmail, authUsername)

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
	h, mdb := newIntegrationRouter(t)
	cookies := registerAuthUser(t, h, mdb, authEmail, authUsername)

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
		fresh := registerAuthUser(t, h, mdb, "fresh-user@example.com", "fresh-user")

		req := newRequest(http.MethodPost, "/auth/refresh", "")
		req.Header.Set("Origin", integrationOrigin)
		req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: fresh.refresh})
		rec := serve(t, h, req)

		require.Equal(t, http.StatusForbidden, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
	})
}

func TestIntegration_AuthLogout(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	cookies := registerAuthUser(t, h, mdb, authEmail, authUsername)

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
		current := registerAuthUser(t, h, mdb, "rotated-user@example.com", "rotated-user")

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
		cookies := registerAuthUser(t, h, mdb, email, "present-account")

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
		cookies := registerAuthUser(t, h, mdb, email, "deleted-account")

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
