package app

import (
	"context"
	"errors"
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

type CampaignModule struct {
	Controller    *CampaignController
	Service       campaign.Service
	JoinIPLimiter ratelimit.Allower
}

type CampaignController struct {
	service campaign.Service
}

func NewCampaignController(service campaign.Service) *CampaignController {
	return &CampaignController{service: service}
}

func (cc *CampaignController) RegisterRoutes(group *echo.Group, joinIPLimit echo.MiddlewareFunc) {
	group.POST("", cc.Create)
	group.GET("", cc.ListMine)
	group.POST("/join", cc.Join, joinIPLimit)
	group.GET("/:id", cc.Fetch)
	group.POST("/:id/leave", cc.Leave)
	group.POST("/:id/members/:userId/promote", cc.Promote)
	group.POST("/:id/members/:userId/demote", cc.Demote)
	group.POST("/:id/members/:userId/remove", cc.Remove)
	group.PUT("/:id/character", cc.Link)
	group.DELETE("/:id/character", cc.Unlink)
	group.GET("/:id/sheets", cc.ListLinkedSheets)
	group.GET("/:id/sheets/:sheetId", cc.FetchLinkedSheet)
	group.POST("/:id/invite/rotate", cc.RotateInvite)
	group.PUT("/:id/password", cc.SetPassword)
	group.DELETE("/:id/password", cc.ClearPassword)
}

func (cc *CampaignController) Join(c *echo.Context) error {
	var cmd campaign.JoinCommand
	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}
	cmd.UserID, _ = auth.UserID(c)
	view, err := cc.service.JoinByCode(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.OK(c, view)
}

func (cc *CampaignController) RotateInvite(c *echo.Context) error {
	actorID, _ := auth.UserID(c)
	view, err := cc.service.RotateInviteCode(c.Request().Context(), campaign.InviteCommand{
		CampaignID: c.Param("id"),
		ActorID:    actorID,
	})
	if err != nil {
		return err
	}
	return response.OK(c, view)
}

func (cc *CampaignController) SetPassword(c *echo.Context) error {
	var cmd campaign.PasswordCommand
	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}
	cmd.ActorID, _ = auth.UserID(c)
	cmd.CampaignID = c.Param("id")
	view, err := cc.service.SetPassword(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.OK(c, view)
}

func (cc *CampaignController) ClearPassword(c *echo.Context) error {
	actorID, _ := auth.UserID(c)
	view, err := cc.service.ClearPassword(c.Request().Context(), campaign.InviteCommand{
		CampaignID: c.Param("id"),
		ActorID:    actorID,
	})
	if err != nil {
		return err
	}
	return response.OK(c, view)
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

func (cc *CampaignController) Link(c *echo.Context) error {
	var cmd campaign.LinkCommand
	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}
	cmd.CampaignID = c.Param("id")
	cmd.UserID, _ = auth.UserID(c)
	view, err := cc.service.Link(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.OK(c, view)
}

func (cc *CampaignController) Unlink(c *echo.Context) error {
	userID, _ := auth.UserID(c)
	view, err := cc.service.Unlink(c.Request().Context(), campaign.UnlinkCommand{
		CampaignID: c.Param("id"),
		UserID:     userID,
	})
	if err != nil {
		return err
	}
	return response.OK(c, view)
}

func (cc *CampaignController) ListLinkedSheets(c *echo.Context) error {
	userID, _ := auth.UserID(c)
	summaries, err := cc.service.ListLinkedSheets(c.Request().Context(), c.Param("id"), userID)
	if err != nil {
		return err
	}
	return response.OK(c, summaries)
}

func (cc *CampaignController) FetchLinkedSheet(c *echo.Context) error {
	userID, _ := auth.UserID(c)
	result, err := cc.service.ReadLinkedSheet(c.Request().Context(), campaign.ReadLinkedSheetQuery{
		CampaignID:  c.Param("id"),
		ViewerID:    userID,
		SheetID:     c.Param("sheetId"),
		IfNoneMatch: c.Request().Header.Get("If-None-Match"),
	})
	if err != nil {
		return err
	}
	c.Response().Header().Set("ETag", result.ETag)
	if result.NotModified {
		return c.NoContent(http.StatusNotModified)
	}
	return response.OK(c, result.Sheet.ToDTO())
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

type campaignCharacterUnlinker struct {
	sheets sheet.Repository
}

func (u campaignCharacterUnlinker) UnlinkCharacter(ctx context.Context, campaignID, userID string) error {
	_, err := u.sheets.UnlinkOwnerCampaign(ctx, userID, campaignID)
	return err
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
