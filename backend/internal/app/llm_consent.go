package app

import (
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"

	"github.com/labstack/echo/v5"
)

// LLMConsentController serves PUT and DELETE /me/llm-consent (ADR 0008).
type LLMConsentController struct {
	service llmconsent.Service
}

// NewLLMConsentController builds the consent controller over the service.
func NewLLMConsentController(service llmconsent.Service) *LLMConsentController {
	return &LLMConsentController{service: service}
}

// RegisterRoutes mounts the consent routes on a group that already carries the
// auth and user rate-limit guards.
func (lc *LLMConsentController) RegisterRoutes(group *echo.Group) {
	group.PUT("", lc.Grant)
	group.DELETE("", lc.Revoke)
}

type llmConsentRequest struct {
	Version string `json:"version"`
}

// Grant records the player's consent to the disclosure version in the body.
func (lc *LLMConsentController) Grant(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}

	var body llmConsentRequest
	if err := json.Decode(c, &body); err != nil {
		return decodeError(err)
	}

	if err := lc.service.Grant(c.Request().Context(), userID, body.Version); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// Revoke withdraws the consent and deletes the generation data the service
// holds for the player.
func (lc *LLMConsentController) Revoke(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}

	if err := lc.service.Revoke(c.Request().Context(), userID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
