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

	"legnaris/internal/app"
	"legnaris/internal/auth"
	authmocks "legnaris/internal/auth/mocks"
	usermocks "legnaris/internal/user/mocks"
	"legnaris/shared/apperr"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// challengeRoute is the request path a verify/resend test drives.
const (
	verifyRoute = "/auth/challenge/verify"
	resendRoute = "/auth/challenge/resend"
)

// challengeID is a canonical UUID v7 the service mock accepts as an identity.
const challengeID = "01900000-0000-7000-8000-000000000001"

// countingObserver records the durable bound's refusals in the existing auth
// metric (OTP-14).
type countingObserver struct{ blocked int }

func (o *countingObserver) RateLimitBlocked() { o.blocked++ }

// newChallengeRouter mounts the two challenge handlers without the guards, so
// the controller test isolates the status mapping and the cookie transport.
func newChallengeRouter(t *testing.T, service auth.Service, observer app.Observer) *echo.Echo {
	t.Helper()

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler

	controller := app.NewChallengeController(service, observer)
	group := e.Group("/auth")
	group.POST("/challenge/verify", controller.Verify)
	group.POST("/challenge/resend", controller.Resend)
	return e
}

func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) response.ErrorResponse {
	t.Helper()

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

// TestChallengeController_Verify pins the verify endpoint: 200 with the session
// body and both cookies on success, the uniform 401 for every service failure,
// 400 for a malformed body and 500 for a store failure (OTP-07, OTP-08,
// OTP-17).
func TestChallengeController_Verify(t *testing.T) {
	t.Run("Happy path: 200 with the session body and both cookies", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			VerifyChallenge(mock.Anything, auth.VerifyChallengeCommand{ChallengeID: challengeID, Code: "042917"}).
			Return(session(), nil).
			Once()

		rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, verifyRoute,
			`{"challenge_id":"`+challengeID+`","code":"042917"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assertSessionCookies(t, rec)
		assertSessionBody(t, rec)
	})

	t.Run("Error: the uniform 401 for an invalid code", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			VerifyChallenge(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeOTPInvalid, auth.MsgOTPInvalid, nil)).
			Once()

		rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, verifyRoute,
			`{"challenge_id":"`+challengeID+`","code":"000000"}`)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))
		body := decodeErrorBody(t, rec)
		assert.Equal(t, string(apperr.CodeOTPInvalid), body.Error.Code)
		assert.Equal(t, auth.MsgOTPInvalid, body.Error.Message)
		assert.NotContains(t, rec.Body.String(), "000000")
	})

	t.Run("Error: a malformed body, unknown field or missing field is 400", func(t *testing.T) {
		service := authmocks.NewMockService(t) // no expectation: the service is never reached

		tests := []struct {
			name string
			body string
			code apperr.Code
		}{
			{name: "invalid JSON", body: `{"challenge_id":`, code: apperr.CodeInvalidJSON},
			// The decoder rejects an undeclared field as invalid JSON; a
			// trailing second value is what yields UNKNOWN_FIELD.
			{name: "unknown field", body: `{"challenge_id":"` + challengeID + `","code":"042917","purpose":"login_otp"}`, code: apperr.CodeInvalidJSON},
			{name: "trailing value", body: `{"challenge_id":"` + challengeID + `","code":"042917"}{"extra":1}`, code: apperr.CodeUnknownField},
			{name: "missing code", body: `{"challenge_id":"` + challengeID + `"}`, code: apperr.CodeInvalidRequest},
			{name: "missing identity", body: `{"code":"042917"}`, code: apperr.CodeInvalidRequest},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, verifyRoute, tc.body)
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				assert.Equal(t, string(tc.code), decodeErrorBody(t, rec).Error.Code)
			})
		}
	})

	t.Run("Error: a store failure is 500, never a code failure", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			VerifyChallenge(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeInternal, auth.MsgInternal, errors.New("mongo down"))).
			Once()

		rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, verifyRoute,
			`{"challenge_id":"`+challengeID+`","code":"042917"}`)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, string(apperr.CodeInternal), decodeErrorBody(t, rec).Error.Code)
	})
}

// TestChallengeController_Resend pins the resend endpoint: the accepted 202
// with the retry window for every outcome, 400 for a malformed body and the
// typed 503 with a retry hint at the daily cap (OTP-10, OTP-11, OTP-13).
func TestChallengeController_Resend(t *testing.T) {
	t.Run("Happy path: 202 with the retry window", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			ResendChallenge(mock.Anything, auth.ResendChallengeCommand{ChallengeID: challengeID}).
			Return(&auth.ResendResult{ResendAfter: 60}, nil).
			Once()

		rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, resendRoute,
			`{"challenge_id":"`+challengeID+`"}`)

		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		var body response.Success[auth.ResendResult]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, 60, body.Data.ResendAfter)
		assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), rec.Body.String())
	})

	t.Run("Error: a malformed body or missing identity is 400", func(t *testing.T) {
		service := authmocks.NewMockService(t)

		for _, tc := range []struct {
			name string
			body string
			code apperr.Code
		}{
			{name: "invalid JSON", body: `{"challenge_id":`, code: apperr.CodeInvalidJSON},
			{name: "unknown field", body: `{"challenge_id":"` + challengeID + `","purpose":"x"}`, code: apperr.CodeInvalidJSON},
			{name: "missing identity", body: `{}`, code: apperr.CodeInvalidRequest},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, resendRoute, tc.body)
				require.Equal(t, http.StatusBadRequest, rec.Code)
				assert.Equal(t, string(tc.code), decodeErrorBody(t, rec).Error.Code)
			})
		}
	})

	t.Run("Error: the daily cap answers 503 with a retry hint", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			ResendChallenge(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeEmailUnavailable, auth.MsgEmailUnavailable, nil).WithRetryAfter(90*time.Second)).
			Once()

		rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, resendRoute,
			`{"challenge_id":"`+challengeID+`"}`)

		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "90", rec.Header().Get("Retry-After"))
		assert.Equal(t, string(apperr.CodeEmailUnavailable), decodeErrorBody(t, rec).Error.Code)
	})
}

// TestChallengeController_Bounds pins the durable per-account refusal: a 429
// with a retry hint, recorded in the existing auth metric (OTP-14).
func TestChallengeController_Bounds(t *testing.T) {
	t.Run("Edge: the account bound answers 429 with a retry hint and a metric", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().
			VerifyChallenge(mock.Anything, mock.Anything).
			Return(nil, apperr.NewError(apperr.CodeRateLimited, auth.MsgRateLimited, nil).WithRetryAfter(time.Hour)).
			Once()

		observer := &countingObserver{}
		rec := doRequest(t, newChallengeRouter(t, service, observer), http.MethodPost, verifyRoute,
			`{"challenge_id":"`+challengeID+`","code":"042917"}`)

		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Equal(t, "3600", rec.Header().Get("Retry-After"))
		assert.Equal(t, 1, observer.blocked, "the refusal is recorded in the existing auth metric")
		body := decodeErrorBody(t, rec)
		assert.Equal(t, string(apperr.CodeRateLimited), body.Error.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"), "a refused verify issues no session")
	})
}

// TestChallengeEndpoints_AddressLimiter proves both routes sit behind the
// per-address limiter: once its budget is spent, the request is refused 429
// with a retry hint before any handler work (OTP-14, OTP-19).
func TestChallengeEndpoints_AddressLimiter(t *testing.T) {
	for _, route := range []string{verifyRoute, resendRoute} {
		t.Run("Edge: "+route+" is refused after the address budget", func(t *testing.T) {
			// A fresh router per route, so the shared per-address budget is
			// spent by this route's own first request.
			h, _ := newWiringRouterWithEnv(t, map[string]string{"AUTH_RATE_LIMIT_ATTEMPTS": "1"})

			// The first request passes the limiter and fails body validation
			// (400) before any database access.
			first := doRequest(t, h, http.MethodPost, route, `{"challenge_id":`)
			require.Equal(t, http.StatusBadRequest, first.Code, first.Body.String())

			second := doRequest(t, h, http.MethodPost, route, `{"challenge_id":`)
			require.Equal(t, http.StatusTooManyRequests, second.Code, second.Body.String())
			assert.NotEmpty(t, second.Header().Get("Retry-After"))
			assert.Equal(t, string(apperr.CodeRateLimited), decodeErrorBody(t, second).Error.Code)
		})
	}
}

// TestChallengeEndpoints_DisclosureSweep drives every verification failure
// through the real service and asserts the six answers are byte-identical, that
// no six-digit code or unmasked address appears in any response body or header,
// and that the service's log lines carry no code (OTP-08, OTP-15, SC-002,
// SC-003).
func TestChallengeEndpoints_DisclosureSweep(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	const (
		code   = "042917"
		userID = "01900000-0000-7000-8000-0000000000ab"
	)
	secret := strings.Repeat("k", 32)

	digest, err := auth.NewCodeDigest(secret)
	require.NoError(t, err)

	row := func() *auth.Challenge {
		return &auth.Challenge{
			ID:           challengeID,
			UserID:       userID,
			Purpose:      auth.PurposeLoginOTP,
			CodeHash:     digest.Digest(userID, auth.PurposeLoginOTP, code),
			ExpiresAt:    now.Add(10 * time.Minute),
			CeilingAt:    now.Add(30 * time.Minute),
			Attempts:     1,
			CreatedAt:    now,
			LastSentAt:   now,
			LastIntentID: "01900000-0000-7000-8000-0000000000ac",
		}
	}

	tests := []struct {
		name       string
		body       string
		wire       func(repo *authmocks.MockChallengeRepository)
		wantStatus int
	}{
		{
			name: "unknown identity",
			body: `{"challenge_id":"` + challengeID + `","code":"` + code + `"}`,
			wire: func(repo *authmocks.MockChallengeRepository) {
				repo.EXPECT().ClaimAttempt(mock.Anything, challengeID, mock.Anything, mock.Anything).Return(nil, auth.ErrChallengeUnusable).Once()
				repo.EXPECT().Fetch(mock.Anything, challengeID).Return(nil, auth.ErrChallengeNotFound).Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed identity",
			body:       `{"challenge_id":"not-a-uuid","code":"` + code + `"}`,
			wire:       func(*authmocks.MockChallengeRepository) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "expired challenge",
			body: `{"challenge_id":"` + challengeID + `","code":"` + code + `"}`,
			wire: func(repo *authmocks.MockChallengeRepository) {
				expired := row()
				expired.ExpiresAt = now
				repo.EXPECT().ClaimAttempt(mock.Anything, challengeID, mock.Anything, mock.Anything).Return(nil, auth.ErrChallengeUnusable).Once()
				repo.EXPECT().Fetch(mock.Anything, challengeID).Return(expired, nil).Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "exhausted challenge",
			body: `{"challenge_id":"` + challengeID + `","code":"` + code + `"}`,
			wire: func(repo *authmocks.MockChallengeRepository) {
				exhausted := row()
				exhausted.Attempts = 5
				repo.EXPECT().ClaimAttempt(mock.Anything, challengeID, mock.Anything, mock.Anything).Return(nil, auth.ErrChallengeUnusable).Once()
				repo.EXPECT().Fetch(mock.Anything, challengeID).Return(exhausted, nil).Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "consumed challenge",
			body: `{"challenge_id":"` + challengeID + `","code":"` + code + `"}`,
			wire: func(repo *authmocks.MockChallengeRepository) {
				repo.EXPECT().ClaimAttempt(mock.Anything, challengeID, mock.Anything, mock.Anything).Return(row(), nil).Once()
				repo.EXPECT().CountForUserSince(mock.Anything, userID, mock.Anything).Return(0, nil).Once()
				repo.EXPECT().Consume(mock.Anything, challengeID, mock.Anything, mock.Anything).Return(false, nil).Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "wrong code",
			body: `{"challenge_id":"` + challengeID + `","code":"000000"}`,
			wire: func(repo *authmocks.MockChallengeRepository) {
				repo.EXPECT().ClaimAttempt(mock.Anything, challengeID, mock.Anything, mock.Anything).Return(row(), nil).Once()
				repo.EXPECT().CountForUserSince(mock.Anything, userID, mock.Anything).Return(0, nil).Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	var bodies [][]byte
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log := &captureLogger{}
			service, repo := newChallengeServiceFixture(t, now, log)
			tc.wire(repo)

			rec := doRequest(t, newChallengeRouter(t, service, &countingObserver{}), http.MethodPost, verifyRoute, tc.body)

			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			assert.Equal(t, string(apperr.CodeOTPInvalid), decodeErrorBody(t, rec).Error.Code)
			assert.Empty(t, rec.Header().Get("Set-Cookie"), "no failure may issue a session")

			for key, values := range rec.Header() {
				for _, value := range values {
					assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), value, "header %s must carry no code", key)
					assert.NotContains(t, value, "@", "header %s must carry no address", key)
				}
			}
			assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), rec.Body.String())
			assert.NotContains(t, rec.Body.String(), "@")

			assert.NotRegexp(t, regexp.MustCompile(`\b\d{6}\b`), log.text())
			assert.NotContains(t, log.text(), "player@example.com")

			bodies = append(bodies, rec.Body.Bytes())
		})
	}

	t.Run("Edge: every failure answered byte-identically", func(t *testing.T) {
		require.Len(t, bodies, len(tests))
		for i := 1; i < len(bodies); i++ {
			assert.Equal(t, string(bodies[0]), string(bodies[i]),
				"failure %q must be indistinguishable from %q", tests[i].name, tests[0].name)
		}
	})
}

// newChallengeServiceFixture wires the real challenge service over the mockery
// mocks with a frozen clock, so a route test can drive the genuine failure
// classifications without a database.
func newChallengeServiceFixture(t *testing.T, now time.Time, log *captureLogger) (auth.Service, *authmocks.MockChallengeRepository) {
	t.Helper()

	digest, err := auth.NewCodeDigest(strings.Repeat("k", 32))
	require.NoError(t, err)

	repo := authmocks.NewMockChallengeRepository(t)
	service := auth.NewService(
		usermocks.NewMockRepository(t),
		authmocks.NewMockRefreshTokenRepository(t),
		authmocks.NewMockPasswordHasher(t),
		authmocks.NewMockTokenManager(t),
		authmocks.NewMockRefreshTokenGenerator(t),
		auth.NewCSRFTokenManager([]byte("csrf-test-secret")),
		authmocks.NewMockObserver(t),
		auth.Config{RefreshTTL: time.Hour, Now: func() time.Time { return now }},
		log,
		auth.ChallengeDeps{
			Repository: repo,
			Generator:  auth.NewCodeGenerator(),
			Digest:     digest,
			Mailer:     authmocks.NewMockCodeMailer(t),
			Observer:   auth.NewNopChallengeObserver(),
			Policy: auth.ChallengePolicy{
				Lifetime:         10 * time.Minute,
				Ceiling:          30 * time.Minute,
				ResendCooldown:   60 * time.Second,
				MaxAttempts:      5,
				DailySendCap:     80,
				UserWindow:       time.Hour,
				MaxPerUserWindow: 10,
			},
		},
	)
	return service, repo
}
