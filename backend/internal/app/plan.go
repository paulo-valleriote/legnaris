package app

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

const MsgPlanUnavailable = "plan is unavailable"

type PlanResolver func(ctx context.Context, userID string) (bool, error)

func FreePlanResolver(context.Context, string) (bool, error) {
	return false, nil
}

func PlanResolverFromSource(source user.PlanSource) PlanResolver {
	return func(ctx context.Context, userID string) (bool, error) {
		resolved, err := source.PlanOf(ctx, userID)
		if err != nil {
			return false, err
		}
		return resolved.ID == plan.IDPaid, nil
	}
}

type planTrialView struct {
	Used      int `json:"used"`
	Remaining int `json:"remaining"`
}

type planGenerationsView struct {
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	ResetsAt  time.Time `json:"resets_at"`
}

type planView struct {
	Plan               string                   `json:"plan"`
	Trial              *planTrialView           `json:"trial,omitempty"`
	Generations        *planGenerationsView     `json:"generations,omitempty"`
	Sheets             map[string]planSheetView `json:"sheets,omitempty"`
	LLMConsentRequired bool                     `json:"llm_consent_required"`
	LastFreeGeneration bool                     `json:"last_free_generation"`
	ActiveGenerationID *string                  `json:"active_generation_id"`
	LLMConsentAt       *time.Time               `json:"llm_consent_at"`
	LLMConsentVersion  *string                  `json:"llm_consent_version"`
}

type ConsentChecker interface {
	Gate(ctx context.Context, userID string) error
	Status(ctx context.Context, userID string) (llmconsent.Record, error)
}

type ActiveGenerationReader interface {
	ActiveGenerationID(ctx context.Context, userID string) (string, bool, error)
}

type planSheetView struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

type PlanOption func(*PlanController)

func WithSheetUsage(sheets sheet.Service) PlanOption {
	return func(pc *PlanController) {
		pc.sheets = sheets
	}
}

func WithGenerationStatus(consent ConsentChecker, active ActiveGenerationReader) PlanOption {
	return func(pc *PlanController) {
		pc.consent = consent
		pc.active = active
	}
}

type PlanController struct {
	quota    *quota.Service
	resolver PlanResolver
	sheets   sheet.Service
	consent  ConsentChecker
	active   ActiveGenerationReader
}

func NewPlanController(allowance *quota.Service, resolver PlanResolver, opts ...PlanOption) *PlanController {
	pc := &PlanController{quota: allowance, resolver: resolver}
	for _, opt := range opts {
		opt(pc)
	}
	return pc
}

func (pc *PlanController) respond(c *echo.Context, userID string, view planView, canGenerate bool) error {
	ctx := c.Request().Context()
	if pc.sheets != nil {
		usage, err := pc.sheets.PlanUsage(ctx, userID)
		if err != nil {
			return err
		}
		view.Sheets = sheetUsageView(usage)
	}
	if pc.consent != nil && pc.active != nil {
		if err := pc.fillGenerationStatus(ctx, userID, canGenerate, &view); err != nil {
			return err
		}
	}
	return response.OK(c, view)
}

func (pc *PlanController) fillGenerationStatus(ctx context.Context, userID string, canGenerate bool, view *planView) error {
	record, err := pc.consent.Status(ctx, userID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgPlanUnavailable, err)
	}
	if record.At != nil {
		view.LLMConsentAt = record.At
		view.LLMConsentVersion = &record.Version
	}

	if canGenerate {
		err := pc.consent.Gate(ctx, userID)
		switch {
		case err == nil:
		case isConsentRequired(err):
			view.LLMConsentRequired = true
		default:
			return apperr.NewError(apperr.CodeInternal, MsgPlanUnavailable, err)
		}
	}
	id, active, err := pc.active.ActiveGenerationID(ctx, userID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgPlanUnavailable, err)
	}
	if active {
		view.ActiveGenerationID = &id
	}
	return nil
}

func isConsentRequired(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == apperr.CodeLLMConsentRequired
}

func sheetUsageView(usage sheet.PlanUsage) map[string]planSheetView {
	views := make(map[string]planSheetView, len(usage.Kinds))
	for _, status := range usage.Kinds {
		views[string(status.Kind)] = planSheetView{Used: status.Used, Limit: status.Limit}
	}
	return views
}

func (pc *PlanController) RegisterRoutes(group *echo.Group) {
	group.GET("/plan", pc.Get)
}

func (pc *PlanController) Get(c *echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return apperr.NewError(apperr.CodeUnauthorized, auth.MsgAuthorizationRequired, nil)
	}
	ctx := c.Request().Context()

	paid, err := pc.resolver(ctx, userID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgPlanUnavailable, err)
	}
	if paid {
		return pc.paidView(c, userID)
	}
	return pc.freeView(c, userID)
}

func (pc *PlanController) freeView(c *echo.Context, userID string) error {
	status, err := pc.quota.TrialStatus(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return pc.respond(c, userID, planView{
		Plan:               "free",
		Trial:              &planTrialView{Used: status.Used, Remaining: status.Remaining},
		LastFreeGeneration: status.Remaining == 1,
	}, status.Remaining > 0)
}

func (pc *PlanController) paidView(c *echo.Context, userID string) error {
	status, err := pc.quota.DailyStatus(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return pc.respond(c, userID, planView{
		Plan: "paid",
		Generations: &planGenerationsView{
			Used:      status.Used,
			Remaining: status.Remaining,
			ResetsAt:  status.ResetsAt,
		},
	}, true)
}
