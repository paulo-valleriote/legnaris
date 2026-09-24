package app

import (
	"errors"

	"legnaris/internal/sheet/template"
	"legnaris/shared/apperr"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
)

type TemplateController struct {
	service template.Service
}

// User-facing error message and cause for an empty template ID path param.
const msgTemplateIDNotProvided = "template id not provided"

var errTemplateIDEmpty = errors.New("template id is empty")

func NewTemplateController(service template.Service) *TemplateController {
	return &TemplateController{
		service: service,
	}
}

func (tc *TemplateController) RegisterRoutes(group *echo.Group) {
	group.GET("", tc.List)
	group.GET("/:id", tc.FetchByID)
	//group.POST("", tc.Create)
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
	id := c.Param("id")
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, msgTemplateIDNotProvided, errTemplateIDEmpty)
	}

	item, err := tc.service.FetchTemplate(c.Request().Context(), &template.FetchTemplateByIDCommand{ID: id})
	if err != nil {
		return err
	}

	return response.OK(c, item.ToDTO())
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
