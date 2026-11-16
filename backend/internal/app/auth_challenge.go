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

type ChallengeController struct {
	service  auth.Service
	observer Observer
}

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
		ClientIP:    c.RealIP(),
	})
	if err != nil {
		return cc.observe(err)
	}

	setSessionCookies(c, session)
	return response.OK(c, sessionResponse(session))
}

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

func (cc *ChallengeController) observe(err error) error {
	var appErr *apperr.Error
	if cc.observer != nil && errors.As(err, &appErr) && appErr.Code == apperr.CodeRateLimited {
		cc.observer.RateLimitBlocked()
	}
	return err
}
