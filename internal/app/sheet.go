package app

import (
	"legnaris/internal/sheet"
	"legnaris/shared/apperr"
	"legnaris/shared/json"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
)

type SheetController struct {
	service sheet.Service
}

func NewSheetController(service sheet.Service) *SheetController {
	return &SheetController{
		service: service,
	}
}

func (sc *SheetController) RegisterRoutes(group *echo.Group) {
	group.POST("", sc.Create)
	group.GET("/:sheetID", sc.FetchByID)
	group.PATCH("/:sheetID", sc.PartialUpdate)
}

func (sc *SheetController) Create(c *echo.Context) error {
	var cmd sheet.CreateSheetCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	newSheet, err := sc.service.Create(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.Created(c, newSheet.ToDTO())
}

func (sc *SheetController) FetchByID(c *echo.Context) error {
	id := c.Param("sheetID")
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, sheet.MsgSheetIDNotProvided, nil)
	}

	storedSheet, err := sc.service.FetchByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.OK(c, storedSheet.ToDTO())
}

func (sc *SheetController) PartialUpdate(c *echo.Context) error {
	var cmd sheet.PartialUpdateSheetCommand

	id := c.Param("sheetID")
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, sheet.MsgSheetIDNotProvided, nil)
	}

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	cascadingEffects, err := sc.service.PartialUpdate(c.Request().Context(), sheet.PartialUpdateSheetCommand{
		SheetID: id,
		Data:    cmd.Data,
	})
	if err != nil {
		return err
	}
	return response.OK(c, cascadingEffects)
}
