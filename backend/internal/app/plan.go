package app

import (
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

// PlanController serves GET /me/plan. It only reads: the plan and the usage it
// reports are the same figures the creation gate checks.
type PlanController struct {
	service sheet.Service
}

// NewPlanController builds the plan endpoint over the sheet service, which owns
// the plan lookup and the counters.
func NewPlanController(service sheet.Service) *PlanController {
	return &PlanController{service: service}
}

// RegisterRoutes mounts the plan endpoint on the authenticated /me group.
func (pc *PlanController) RegisterRoutes(group *echo.Group) {
	group.GET("/plan", pc.Plan)
}

// PlanDTO is the wire shape of GET /me/plan: the plan identifier and, per Sheet
// kind, how many Sheets the owner holds and how many the plan allows.
type PlanDTO struct {
	Plan   string                   `json:"plan"`
	Sheets map[string]SheetUsageDTO `json:"sheets"`
}

// SheetUsageDTO is the usage of one kind.
type SheetUsageDTO struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

// Plan answers the authenticated owner's plan and usage.
func (pc *PlanController) Plan(c *echo.Context) error {
	ownerID, _ := auth.UserID(c)
	usage, err := pc.service.PlanUsage(c.Request().Context(), ownerID)
	if err != nil {
		return err
	}
	return response.OK(c, planDTOFrom(usage))
}

// planDTOFrom maps the service's usage onto the wire shape, keyed by kind name.
func planDTOFrom(usage sheet.PlanUsage) PlanDTO {
	dto := PlanDTO{Plan: usage.PlanID, Sheets: make(map[string]SheetUsageDTO, len(usage.Kinds))}
	for _, status := range usage.Kinds {
		dto.Sheets[string(status.Kind)] = SheetUsageDTO{Used: status.Used, Limit: status.Limit}
	}
	return dto
}
