package app

import (
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

const csrfHeader = "X-CSRF-Token"

const bearerTokenType = "Bearer"

type AuthController struct {
	service auth.Service
}

func NewAuthController(service auth.Service) *AuthController {
	return &AuthController{
		service: service,
	}
}

type registrationDeferredBody struct {
	Delivery string `json:"delivery"`
}

func (ac *AuthController) Register(c *echo.Context) error {
	var cmd auth.RegisterCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	issued, err := ac.service.Register(c.Request().Context(), cmd)
	if err != nil {
		return err
	}

	if issued.Delivery == auth.DeliveryDeferred {
		return response.JSON(c, http.StatusCreated, registrationDeferredBody{Delivery: auth.DeliveryDeferred})
	}
	return response.JSON(c, http.StatusCreated, issued)
}

func (ac *AuthController) Login(c *echo.Context) error {
	var cmd auth.LoginCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	result, err := ac.service.Login(c.Request().Context(), cmd)
	if err != nil {
		return err
	}

	if result.Challenge != nil {
		return response.JSON(c, http.StatusAccepted, result.Challenge)
	}
	setSessionCookies(c, result.Session)
	return response.OK(c, sessionResponse(result.Session))
}

func (ac *AuthController) Refresh(c *echo.Context) error {
	session, err := ac.service.Refresh(c.Request().Context(), auth.RefreshCommand{
		RefreshToken: refreshCookieValue(c),
		CSRFToken:    c.Request().Header.Get(csrfHeader),
	})
	if err != nil {
		return err
	}

	setSessionCookies(c, session)
	return response.OK(c, sessionResponse(session))
}

type CSRFTokenBody struct {
	CSRFToken string `json:"csrf_token"`
}

func (ac *AuthController) CSRF(c *echo.Context) error {
	token, err := ac.service.IssueCSRF(c.Request().Context(), refreshCookieValue(c))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.OK(c, CSRFTokenBody{CSRFToken: token})
}

func (ac *AuthController) Logout(c *echo.Context) error {
	credential := refreshCookieValue(c)

	err := ac.service.Logout(c.Request().Context(), auth.LogoutCommand{
		RefreshToken: credential,
		CSRFToken:    c.Request().Header.Get(csrfHeader),
	})
	if err != nil {
		return err
	}

	if credential != "" {
		c.SetCookie(auth.ClearRefreshCookie())
		c.SetCookie(auth.ClearCSRFCookie())
	}
	return c.NoContent(http.StatusNoContent)
}

func (ac *AuthController) Me(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}

	profile, err := ac.service.Profile(c.Request().Context(), userID)
	if err != nil {
		return err
	}

	return response.OK(c, profile)
}

func (ac *AuthController) RequestAccountDeletion(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}

	issued, err := ac.service.RequestAccountDeletion(c.Request().Context(), userID)
	if err != nil {
		return err
	}

	return response.JSON(c, http.StatusAccepted, issued)
}

type deleteAccountRequest struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}

func (ac *AuthController) DeleteAccount(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}

	var request deleteAccountRequest
	if err := json.Decode(c, &request); err != nil {
		return decodeError(err)
	}
	if request.ChallengeID == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, auth.MsgChallengeIDRequired, nil)
	}
	if request.Code == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, auth.MsgCodeRequired, nil)
	}

	if err := ac.service.DeleteAccount(c.Request().Context(), auth.DeleteAccountCommand{
		UserID:      userID,
		ChallengeID: request.ChallengeID,
		Code:        request.Code,
	}); err != nil {
		return err
	}

	return response.JSON(c, http.StatusAccepted, accountDeletionDTO{Status: user.StatusDeleting})
}

type accountDeletionDTO struct {
	Status string `json:"status"`
}

func setSessionCookies(c *echo.Context, session *auth.Session) {
	c.SetCookie(auth.RefreshCookie(session.RefreshToken, session.RefreshMaxAge))
	c.SetCookie(auth.CSRFCookie(session.CSRFToken, session.RefreshMaxAge))
}

func refreshCookieValue(c *echo.Context) string {
	cookie, err := c.Cookie(auth.RefreshCookieName)
	if err != nil || cookie == nil {
		return ""
	}
	return cookie.Value
}

func sessionResponse(session *auth.Session) auth.SessionResponse {
	return auth.SessionResponse{
		AccessToken: session.AccessToken,
		TokenType:   bearerTokenType,
		ExpiresIn:   session.ExpiresIn,
	}
}
