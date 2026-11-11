package app

import (
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

// csrfHeader carries the session-bound CSRF token the page echoes back on the
// cookie-authenticated routes (AUTH-09).
const csrfHeader = "X-CSRF-Token"

// bearerTokenType is the scheme echoed in the session body; the access-token
// contract is unchanged (AUTH-26).
const bearerTokenType = "Bearer"

type AuthController struct {
	service auth.Service
}

func NewAuthController(service auth.Service) *AuthController {
	return &AuthController{
		service: service,
	}
}

// registrationDeferredBody is the 201 answer when the account was created but
// the code could not be queued. It carries no challenge, cookie or session.
type registrationDeferredBody struct {
	Delivery string `json:"delivery"`
}

// Register creates the account and answers 201 with the started code step and
// no cookie at all (AUTH-02, TFA-01). When the daily allowance is spent the
// answer is 201 with delivery "deferred" and the same absence of credentials (D1).
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

// Login answers 200 with the session body and both cookies for a verified
// account (AUTH-01). An unverified account gets 202 with the started
// verify_email challenge and no cookie or session (D2).
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

// Refresh consumes the refresh credential from the cookie only and rotates the
// session: 200 with the session body and rotated cookies (AUTH-03, AUTH-05).
// The X-CSRF-Token header is read here and verified by the service (AUTH-09).
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

// CSRFTokenBody is the body of GET /auth/csrf.
type CSRFTokenBody struct {
	CSRFToken string `json:"csrf_token"`
}

// CSRF answers the CSRF token bound to the session named by the refresh cookie.
// The __Host- CSRF cookie is invisible to a page served from another host, so
// the page asks for the token here before a refresh or logout (AUTH-09).
func (ac *AuthController) CSRF(c *echo.Context) error {
	token, err := ac.service.IssueCSRF(c.Request().Context(), refreshCookieValue(c))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.OK(c, CSRFTokenBody{CSRFToken: token})
}

// Logout revokes the session's family and clears both cookies, answering 204
// (AUTH-04). With no cookie, logout is a 204 no-op that sets nothing (AUTH-06).
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

// Me answers the signed-in account: its identifier, its canonical address, its
// display name and whether its address was verified. The account identity comes
// from the access credential, never from the request, and the verification
// instant is never disclosed (TFA-08, TFA-09).
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

// setSessionCookies writes the refresh credential and the CSRF token as the two
// contract-fixed cookies. The credential never reaches the body (AUTH-01,
// AUTH-08, AUTH-12). It is shared with the challenge verification controller,
// which establishes the same session.
func setSessionCookies(c *echo.Context, session *auth.Session) {
	c.SetCookie(auth.RefreshCookie(session.RefreshToken, session.RefreshMaxAge))
	c.SetCookie(auth.CSRFCookie(session.CSRFToken, session.RefreshMaxAge))
}

// refreshCookieValue reads the refresh credential from the __Host cookie. A
// value supplied in a header or body is never consulted (AUTH-05).
func refreshCookieValue(c *echo.Context) string {
	cookie, err := c.Cookie(auth.RefreshCookieName)
	if err != nil || cookie == nil {
		return ""
	}
	return cookie.Value
}

// sessionResponse projects the service's session onto the wire body, which
// carries no refresh credential (AUTH-08).
func sessionResponse(session *auth.Session) auth.SessionResponse {
	return auth.SessionResponse{
		AccessToken: session.AccessToken,
		TokenType:   bearerTokenType,
		ExpiresIn:   session.ExpiresIn,
	}
}
