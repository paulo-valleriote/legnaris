package app

import (
	"legnaris/internal/sheet/template"
	"legnaris/shared/response"

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
	//group.POST("", tc.Create)
}

func (tc *TemplateController) List(c *echo.Context) error {
	items, err := tc.service.List(c.Request().Context())
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

/*
func (tc *TemplateController) Create(c *echo.Context) error {
	var cmd template.CreateTemplateCommand

	if err := json.Decode(c, &cmd); err != nil {
		return err
	}

	err := tc.service.Create(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.Created(c, map[string]any{})
}
*/
