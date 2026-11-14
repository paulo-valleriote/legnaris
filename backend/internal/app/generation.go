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

// GenerationService is the generation use case the controller drives. The
// generation service satisfies it.
type GenerationService interface {
	Create(ctx context.Context, cmd generation.CreateCommand) (string, error)
	Fetch(ctx context.Context, userID, id string) (generation.View, error)
	Accept(ctx context.Context, cmd generation.AcceptCommand) (generation.AcceptResult, error)
}

// GenerationController serves the character generation routes (ADR 0007, LGN-186).
type GenerationController struct {
	service GenerationService
}

// NewGenerationController builds the controller over the generation service.
func NewGenerationController(service GenerationService) *GenerationController {
	return &GenerationController{service: service}
}

// RegisterRoutes mounts the generation routes on a group that carries the auth
// guard. Each route takes its own per-user budget from its own limiter namespace:
// POST and accept share the create budget, GET has the read budget (ADR 0007).
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

// Create answers 202 with the id of the queued generation. The concept is never
// echoed back or logged.
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

// generationView is the wire shape of GET /generations/{id}. Values and
// error_code are present only in the states that carry them.
type generationView struct {
	ID         string         `json:"id"`
	Status     string         `json:"status"`
	Kind       string         `json:"kind"`
	TemplateID string         `json:"template_id"`
	Values     map[string]any `json:"values,omitempty"`
	ErrorCode  string         `json:"error_code,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Get answers the player's own generation. A foreign or expired id is 404.
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
		ID:         view.ID,
		Status:     string(view.Status),
		Kind:       view.Kind,
		TemplateID: view.TemplateID,
		Values:     view.Values,
		ErrorCode:  view.ErrorCode,
		CreatedAt:  view.CreatedAt,
	})
}

type acceptGenerationRequest struct {
	SheetName string         `json:"sheet_name"`
	Values    map[string]any `json:"values"`
}

type acceptGenerationResponse struct {
	SheetID string `json:"sheet_id"`
}

// Accept turns a succeeded generation into a Sheet: 201 on the first accept and
// 200 with the same sheet_id on every repeat.
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
