package app

import (
	"errors"
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

// ChallengeController serves the two unified code-step endpoints. The purpose
// of a challenge is resolved server-side from the stored record and is never
// accepted from, or echoed to, the client (OTP-09, OTP-19).
type ChallengeController struct {
	service  auth.Service
	observer Observer
}

// NewChallengeController wires the challenge endpoints. The observer records
// the durable per-account bound's refusals in the existing auth metric
// (OTP-14); it may be nil in a test that does not exercise that bound.
func NewChallengeController(service auth.Service, observer Observer) *ChallengeController {
	return &ChallengeController{service: service, observer: observer}
}

type verifyChallengeRequest struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}

type resendChallengeRequest struct {
	ChallengeID string `json:"challenge_id"`
}

// Verify consumes a challenge and establishes a session exactly as sign-in
// does: 200 with the session body and both cookies on success, and the one
// uniform 401 for every failure (OTP-07, OTP-08).
func (cc *ChallengeController) Verify(c *echo.Context) error {
	var request verifyChallengeRequest
	if err := json.Decode(c, &request); err != nil {
		return decodeError(err)
	}
	if request.ChallengeID == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, auth.MsgChallengeIDRequired, nil)
	}
	if request.Code == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, auth.MsgCodeRequired, nil)
	}

	session, err := cc.service.VerifyChallenge(c.Request().Context(), auth.VerifyChallengeCommand{
		ChallengeID: request.ChallengeID,
		Code:        request.Code,
	})
	if err != nil {
		return cc.observe(err)
	}

	setSessionCookies(c, session)
	return response.OK(c, sessionResponse(session))
}

// Resend asks for the same code again: 202 with the retry window for every
// answer except the daily-cap refusal, which is the typed 503 with a retry hint
// (OTP-10, OTP-11, OTP-13).
func (cc *ChallengeController) Resend(c *echo.Context) error {
	var request resendChallengeRequest
	if err := json.Decode(c, &request); err != nil {
		return decodeError(err)
	}
	if request.ChallengeID == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, auth.MsgChallengeIDRequired, nil)
	}

	result, err := cc.service.ResendChallenge(c.Request().Context(), auth.ResendChallengeCommand{
		ChallengeID: request.ChallengeID,
	})
	if err != nil {
		return cc.observe(err)
	}

	return response.JSON(c, http.StatusAccepted, result)
}

// observe records a refusal produced by the service — the durable per-account
// bound answers the existing rate-limit outcome, which belongs in the existing
// auth metric (OTP-14) — and passes the typed error through untouched.
func (cc *ChallengeController) observe(err error) error {
	var appErr *apperr.Error
	if cc.observer != nil && errors.As(err, &appErr) && appErr.Code == apperr.CodeRateLimited {
		cc.observer.RateLimitBlocked()
	}
	return err
}
