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

func (tc *SheetController) RegisterRoutes(group *echo.Group) {
	group.POST("", tc.Create)
	group.GET("/:sheetID", tc.FetchByID)
	group.PATCH("/:sheetID", tc.PartialUpdate)
}

func (tc *SheetController) Create(c *echo.Context) error {
	var cmd sheet.CreateSheetCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	newSheet, err := tc.service.Create(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.Created(c, newSheet.ToDTO())
}

func (tc *SheetController) FetchByID(c *echo.Context) error {
	id := c.Param("sheetID")
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, sheet.MsgSheetIDNotProvided, nil)
	}

	storedSheet, err := tc.service.FetchByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.OK(c, storedSheet.ToDTO())
}

func (tc *SheetController) PartialUpdate(c *echo.Context) error {
	var cmd sheet.PartialUpdateSheetCommand

	id := c.Param("sheetID")
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, sheet.MsgSheetIDNotProvided, nil)
	}

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	cascadingEffects, err := tc.service.PartialUpdate(c.Request().Context(), sheet.PartialUpdateSheetCommand{
		SheetID: id,
		Data:    cmd.Data,
	})
	if err != nil {
		return err
	}
	return response.OK(c, cascadingEffects)
}
