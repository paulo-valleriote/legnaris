package app

import (
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/billing"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

type checkoutView struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type BillingController struct {
	svc *billing.Service
}

func NewBillingController(svc *billing.Service) *BillingController {
	return &BillingController{svc: svc}
}

func (c *BillingController) RegisterRoutes(group *echo.Group) {
	group.POST("/checkout", c.Checkout)
	group.POST("/portal", c.Portal)
	group.POST("/cancel", c.Cancel)
}

func (c *BillingController) Checkout(ec *echo.Context) error {
	userID, ok := auth.UserID(ec)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	out, err := c.svc.Checkout(ec.Request().Context(), userID)
	if err != nil {
		return err
	}
	return response.OK(ec, checkoutView{URL: out.URL, ExpiresAt: out.ExpiresAt})
}
