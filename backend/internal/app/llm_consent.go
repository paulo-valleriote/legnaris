package app

import (
	"context"
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"

	"github.com/labstack/echo/v5"
)

type generationQuotaRefunder struct {
	quota generation.Quota
}

func (r generationQuotaRefunder) RefundOpen(ctx context.Context, open llmconsent.OpenGeneration) error {
	return generation.RefundQuota(ctx, r.quota, generation.Document{
		ID:     open.ID,
		UserID: open.UserID,
		Quota:  generation.Reservation{Kind: open.ReservationKind, DayUTC: open.DayUTC},
	})
}

type LLMConsentController struct {
	service llmconsent.Service
}

func NewLLMConsentController(service llmconsent.Service) *LLMConsentController {
	return &LLMConsentController{service: service}
}

func (lc *LLMConsentController) RegisterRoutes(group *echo.Group) {
	group.PUT("", lc.Grant)
	group.DELETE("", lc.Revoke)
}

type llmConsentRequest struct {
	Version string `json:"version"`
}

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
