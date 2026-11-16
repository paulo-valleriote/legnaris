package app

import (
	"context"
	"net/http"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

type GenerationService interface {
	Create(ctx context.Context, cmd generation.CreateCommand) (string, error)
	Fetch(ctx context.Context, userID, id string) (generation.View, error)
	Accept(ctx context.Context, cmd generation.AcceptCommand) (generation.AcceptResult, error)
}

type GenerationController struct {
	service GenerationService
}

func NewGenerationController(service GenerationService) *GenerationController {
	return &GenerationController{service: service}
}

func (gc *GenerationController) RegisterRoutes(group *echo.Group, create, read ratelimit.Allower) {
	group.POST("", gc.Create, userRateLimit(create))
	group.GET("/:id", gc.Get, userRateLimit(read))
	group.POST("/:id/accept", gc.Accept, userRateLimit(create))
}

type createGenerationRequest struct {
	TemplateID string `json:"template_id"`
	Kind       string `json:"kind"`
	Concept    string `json:"concept"`
}

type createGenerationResponse struct {
	ID string `json:"id"`
}

func (gc *GenerationController) Create(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	var body createGenerationRequest
	if err := json.Decode(c, &body); err != nil {
		return decodeError(err)
	}

	id, err := gc.service.Create(c.Request().Context(), generation.CreateCommand{
		UserID:     userID,
		TemplateID: body.TemplateID,
		Kind:       body.Kind,
		Concept:    body.Concept,
	})
	if err != nil {
		return err
	}
	return response.JSON(c, http.StatusAccepted, createGenerationResponse{ID: id})
}

type generationView struct {
	ID            string         `json:"id"`
	Status        string         `json:"status"`
	Kind          string         `json:"kind"`
	TemplateID    string         `json:"template_id"`
	Values        map[string]any `json:"values,omitempty"`
	ErrorCode     string         `json:"error_code,omitempty"`
	SuggestedName *string        `json:"suggested_name"`
	CreatedAt     time.Time      `json:"created_at"`
}

func (gc *GenerationController) Get(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	view, err := gc.service.Fetch(c.Request().Context(), userID, c.Param("id"))
	if err != nil {
		return err
	}
	return response.OK(c, generationView{
		ID:            view.ID,
		Status:        string(view.Status),
		Kind:          view.Kind,
		TemplateID:    view.TemplateID,
		Values:        view.Values,
		ErrorCode:     view.ErrorCode,
		SuggestedName: view.SuggestedName,
		CreatedAt:     view.CreatedAt,
	})
}

type acceptGenerationRequest struct {
	SheetName string         `json:"sheet_name"`
	Values    map[string]any `json:"values"`
}

type acceptGenerationResponse struct {
	SheetID string `json:"sheet_id"`
}

func (gc *GenerationController) Accept(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	var body acceptGenerationRequest
	if err := json.Decode(c, &body); err != nil {
		return decodeError(err)
	}

	result, err := gc.service.Accept(c.Request().Context(), generation.AcceptCommand{
		UserID:    userID,
		ID:        c.Param("id"),
		SheetName: body.SheetName,
		Values:    body.Values,
	})
	if err != nil {
		return err
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	return response.JSON(c, status, acceptGenerationResponse{SheetID: result.SheetID})
}
