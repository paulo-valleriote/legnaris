package app

import (
	"legnaris/internal/sheet"
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
	group.GET("/:id", sc.FetchByID)
	group.PATCH("/:id", sc.PartialUpdate)
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
	storedSheet, err := sc.service.FetchByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return response.OK(c, storedSheet.ToDTO())
}

func (sc *SheetController) PartialUpdate(c *echo.Context) error {
	var cmd sheet.PartialUpdateSheetCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	cascadingEffects, err := sc.service.PartialUpdate(c.Request().Context(), sheet.PartialUpdateSheetCommand{
		SheetID: c.Param("id"),
		Data:    cmd.Data,
	})
	if err != nil {
		return err
	}
	return response.OK(c, cascadingEffects)
}
