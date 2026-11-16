package app

import (
	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

type portalView struct {
	URL string `json:"url"`
}

type cancelView struct {
	Status string `json:"status"`
}

const (
	cancelStatusRequested = "cancel_requested"
	refundStatusRequested = "requested"
)

type refundView struct {
	Status string `json:"status"`
}

func (c *BillingController) Refund(ec *echo.Context) error {
	userID, ok := auth.UserID(ec)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	if err := c.svc.Refund(ec.Request().Context(), userID); err != nil {
		return err
	}
	return response.JSON(ec, 202, refundView{Status: refundStatusRequested})
}

func (c *BillingController) Portal(ec *echo.Context) error {
	userID, ok := auth.UserID(ec)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	url, err := c.svc.Portal(ec.Request().Context(), userID)
	if err != nil {
		return err
	}
	return response.OK(ec, portalView{URL: url})
}

func (c *BillingController) Cancel(ec *echo.Context) error {
	userID, ok := auth.UserID(ec)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	if err := c.svc.CancelAtPeriodEnd(ec.Request().Context(), userID); err != nil {
		return err
	}
	return response.JSON(ec, 202, cancelView{Status: cancelStatusRequested})
}
