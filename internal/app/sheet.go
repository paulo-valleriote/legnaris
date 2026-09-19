package app

import (
	"legnaris/internal/sheet"
	"legnaris/shared/json"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
)

type SheetController struct {
	service *sheet.Service
}

func NewSheetController(service *sheet.Service) *SheetController {
	return &SheetController{
		service: service,
	}
}

func (tc *SheetController) RegisterRoutes(group *echo.Group) {
	group.POST("", tc.Create)
}

func (tc *SheetController) Create(c *echo.Context) error {
	var cmd sheet.CreateSheetCommand

	if err := json.Decode(c, &cmd); err != nil {
		return err
	}

	newSheet, err := tc.service.Create(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.Created(c, newSheet)
}
