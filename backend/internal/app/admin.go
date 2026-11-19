package app

import (
	"net/http"
	"time"

	"legnaris/backend/internal/admin"
	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/apperr"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

const (
	adminCompCanceled = "canceled"
	adminQuotaReset   = "reset"
	adminRefundQueued = "requested"
)

type AdminController struct {
	service *admin.Service
}

func NewAdminController(service *admin.Service) *AdminController {
	return &AdminController{service: service}
}

func authHidden() error {
	return apperr.NewError(apperr.CodeNotFound, auth.MsgNotFound, nil)
}

func (c *AdminController) RegisterRoutes(group *echo.Group) {
	group.GET("/users", c.SearchUsers)
	group.GET("/users/:id", c.User)
	group.POST("/users/:id/comp", c.GrantComp)
	group.POST("/users/:id/comp/revoke", c.RevokeComp)
	group.POST("/users/:id/quota/reset", c.ResetQuota)
	group.POST("/users/:id/refund", c.Refund)
}

func auditMeta(ec *echo.Context) admin.RequestMeta {
	return admin.RequestMeta{
		RequestID: ec.Response().Header().Get(echo.HeaderXRequestID),
		ClientIP:  ec.RealIP(),
	}
}

type adminCompRequest struct {
	PlanID    string     `json:"planId"`
	ExpiresAt *time.Time `json:"expiresAt"`
	Reason    string     `json:"reason"`
}

type adminReasonRequest struct {
	Reason string `json:"reason"`
}

type adminRefundRequest struct {
	Override bool   `json:"override"`
	Reason   string `json:"reason"`
}

type adminStatusView struct {
	Status string `json:"status"`
}

func (c *AdminController) User(ec *echo.Context) error {
	view, err := c.service.User(ec.Request().Context(), ec.Param("id"))
	if err != nil {
		return err
	}
	return response.OK(ec, view)
}

func (c *AdminController) SearchUsers(ec *echo.Context) error {
	view, err := c.service.UserByEmail(ec.Request().Context(), ec.QueryParam("email"))
	if err != nil {
		return err
	}
	return response.OK(ec, view)
}

func (c *AdminController) GrantComp(ec *echo.Context) error {
	actorID, ok := auth.UserID(ec)
	if !ok {
		return authHidden()
	}
	var body adminCompRequest
	if err := sharedjson.Decode(ec, &body); err != nil {
		return decodeError(err)
	}
	sub, err := c.service.GrantComp(ec.Request().Context(), actorID, ec.Param("id"), admin.CompGrant{
		PlanID:    body.PlanID,
		ExpiresAt: body.ExpiresAt,
		Reason:    body.Reason,
	}, auditMeta(ec))
	if err != nil {
		return err
	}
	return response.Created(ec, admin.SubscriptionViewOf(sub))
}

func (c *AdminController) RevokeComp(ec *echo.Context) error {
	actorID, ok := auth.UserID(ec)
	if !ok {
		return authHidden()
	}
	var body adminReasonRequest
	if err := sharedjson.Decode(ec, &body); err != nil {
		return decodeError(err)
	}
	if err := c.service.RevokeComp(ec.Request().Context(), actorID, ec.Param("id"), body.Reason, auditMeta(ec)); err != nil {
		return err
	}
	return response.OK(ec, adminStatusView{Status: adminCompCanceled})
}

func (c *AdminController) ResetQuota(ec *echo.Context) error {
	actorID, ok := auth.UserID(ec)
	if !ok {
		return authHidden()
	}
	var body adminReasonRequest
	if err := sharedjson.Decode(ec, &body); err != nil {
		return decodeError(err)
	}
	if err := c.service.ResetQuota(ec.Request().Context(), actorID, ec.Param("id"), body.Reason, auditMeta(ec)); err != nil {
		return err
	}
	return response.OK(ec, adminStatusView{Status: adminQuotaReset})
}

func (c *AdminController) Refund(ec *echo.Context) error {
	actorID, ok := auth.UserID(ec)
	if !ok {
		return authHidden()
	}
	var body adminRefundRequest
	if err := sharedjson.Decode(ec, &body); err != nil {
		return decodeError(err)
	}
	if err := c.service.Refund(ec.Request().Context(), actorID, ec.Param("id"), body.Override, body.Reason, auditMeta(ec)); err != nil {
		return err
	}
	return response.JSON(ec, http.StatusAccepted, adminStatusView{Status: adminRefundQueued})
}
