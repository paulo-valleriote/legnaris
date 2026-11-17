package app

import (
	"context"
	"errors"
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

type CampaignModule struct {
	Controller *CampaignController
	Service    campaign.Service
}

type CampaignController struct {
	service campaign.Service
}

func NewCampaignController(service campaign.Service) *CampaignController {
	return &CampaignController{service: service}
}

func (cc *CampaignController) RegisterRoutes(group *echo.Group) {
	group.POST("", cc.Create)
	group.GET("", cc.ListMine)
	group.GET("/:id", cc.Fetch)
	group.POST("/:id/leave", cc.Leave)
	group.POST("/:id/members/:userId/promote", cc.Promote)
	group.POST("/:id/members/:userId/demote", cc.Demote)
	group.POST("/:id/members/:userId/remove", cc.Remove)
}

func (cc *CampaignController) Create(c *echo.Context) error {
	var cmd campaign.CreateCommand
	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}
	cmd.OwnerID, _ = auth.UserID(c)
	view, err := cc.service.Create(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.Created(c, view)
}

func (cc *CampaignController) ListMine(c *echo.Context) error {
	userID, _ := auth.UserID(c)
	summaries, err := cc.service.ListMine(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return response.OK(c, summaries)
}

func (cc *CampaignController) Fetch(c *echo.Context) error {
	userID, _ := auth.UserID(c)
	view, err := cc.service.Fetch(c.Request().Context(), c.Param("id"), userID)
	if err != nil {
		return err
	}
	return response.OK(c, view)
}

func (cc *CampaignController) Leave(c *echo.Context) error {
	userID, _ := auth.UserID(c)
	err := cc.service.Leave(c.Request().Context(), campaign.LeaveCommand{
		CampaignID: c.Param("id"),
		UserID:     userID,
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (cc *CampaignController) Promote(c *echo.Context) error {
	return cc.changeRole(c, cc.service.Promote)
}

func (cc *CampaignController) Demote(c *echo.Context) error {
	return cc.changeRole(c, cc.service.Demote)
}

func (cc *CampaignController) Remove(c *echo.Context) error {
	return cc.changeRole(c, cc.service.Remove)
}

func (cc *CampaignController) changeRole(c *echo.Context, apply func(context.Context, campaign.RoleCommand) (campaign.View, error)) error {
	actorID, _ := auth.UserID(c)
	view, err := apply(c.Request().Context(), campaign.RoleCommand{
		CampaignID: c.Param("id"),
		ActorID:    actorID,
		TargetID:   c.Param("userId"),
	})
	if err != nil {
		return err
	}
	return response.OK(c, view)
}

type campaignNameLookup struct {
	users user.Repository
}

func (l campaignNameLookup) Username(ctx context.Context, userID string) (string, error) {
	found, err := l.users.FetchByID(ctx, userID)
	if errors.Is(err, user.ErrUserNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return found.Username, nil
}
