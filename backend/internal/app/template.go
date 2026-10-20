package app

import (
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

type TemplateController struct {
	service template.Service
}

func NewTemplateController(service template.Service) *TemplateController {
	return &TemplateController{
		service: service,
	}
}

func (tc *TemplateController) RegisterRoutes(group *echo.Group) {
	group.GET("", tc.List)
	group.GET("/:id", tc.FetchByID)
}

func (tc *TemplateController) List(c *echo.Context) error {
	items, err := tc.service.List(c.Request().Context())
	if err != nil {
		return err
	}

	dtos := make([]template.TemplateDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, item.ToDTO())
	}

	return response.OK(c, dtos)
}

func (tc *TemplateController) FetchByID(c *echo.Context) error {
	item, err := tc.service.FetchTemplate(c.Request().Context(), &template.FetchTemplateByIDCommand{ID: c.Param("id")})
	if err != nil {
		return templateError(err)
	}

	return response.OK(c, item.ToDTO())
}
