package app

import (
	"io"
	"net/http"
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
	group.POST("/refund", c.Refund)
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

const (
	billingSignatureHeader = "Stripe-Signature"
	billingWebhookMaxBody  = 64 * 1024
)

type BillingWebhookController struct {
	inbox *billing.Inbox
}

func NewBillingWebhookController(inbox *billing.Inbox) *BillingWebhookController {
	return &BillingWebhookController{inbox: inbox}
}

func (c *BillingWebhookController) Receive(ec *echo.Context) error {
	body := http.MaxBytesReader(ec.Response(), ec.Request().Body, billingWebhookMaxBody)
	defer func() { _ = body.Close() }()
	payload, err := io.ReadAll(body)
	if err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, billing.MsgInvalidWebhook, err)
	}
	if err := c.inbox.Receive(ec.Request().Context(), payload, ec.Request().Header.Get(billingSignatureHeader)); err != nil {
		return err
	}
	return ec.NoContent(http.StatusOK)
}
