package app

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/quota"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

// MsgPlanUnavailable hides a failure to resolve the caller's Plan behind a
// generic internal error.
const MsgPlanUnavailable = "plan is unavailable"

// PlanResolver reports whether the account holds a paid Plan. Until billing
// ships in F1b every account is Free, so the default resolver answers false.
type PlanResolver func(ctx context.Context, userID string) (bool, error)

// FreePlanResolver is the resolver of F1a, where no account can be paid yet.
func FreePlanResolver(context.Context, string) (bool, error) {
	return false, nil
}

// planTrialView is the Free Plan's lifetime allowance as the client reads it.
type planTrialView struct {
	Used      int `json:"used"`
	Remaining int `json:"remaining"`
}

// planGenerationsView is the paid Plan's daily quota as the client reads it.
type planGenerationsView struct {
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	ResetsAt  time.Time `json:"resets_at"`
}

// planView answers GET /me/plan. Exactly one of Trial and Generations is set,
// according to Plan. Fields are only ever added, so readers that ignore unknown
// keys keep working.
type planView struct {
	Plan        string               `json:"plan"`
	Trial       *planTrialView       `json:"trial,omitempty"`
	Generations *planGenerationsView `json:"generations,omitempty"`
	// Sheets is the per-kind Sheet usage against the Plan's allowance. It is
	// present only when the controller was built WithSheetUsage (LGN-183).
	Sheets map[string]planSheetView `json:"sheets,omitempty"`
	// LLMConsentRequired tells the client to show the consent disclosure before a
	// generation. A Free account with no allowance left is never asked (ADR 0008).
	LLMConsentRequired bool `json:"llm_consent_required"`
	// LastFreeGeneration tells the client that the next generation spends the
	// Free account's last trial allowance, so it can ask before regenerating
	// (LGN-196). The rule lives here so every client applies the same one.
	LastFreeGeneration bool `json:"last_free_generation"`
	// ActiveGenerationID is the caller's queued or running generation, or null.
	ActiveGenerationID *string `json:"active_generation_id"`
	// LLMConsentAt and LLMConsentVersion are the stored consent, both null when
	// the player never consented or has withdrawn it (LGN-195).
	LLMConsentAt      *time.Time `json:"llm_consent_at"`
	LLMConsentVersion *string    `json:"llm_consent_version"`
}

// ConsentChecker reports whether the caller holds a current LLM consent. Gate is
// the consent gate of POST /generations; a missing consent is
// LLM_CONSENT_REQUIRED. Status is the stored record shown on the account screen.
type ConsentChecker interface {
	Gate(ctx context.Context, userID string) error
	Status(ctx context.Context, userID string) (llmconsent.Record, error)
}

// ActiveGenerationReader reports the caller's queued or running generation.
type ActiveGenerationReader interface {
	ActiveGenerationID(ctx context.Context, userID string) (string, bool, error)
}

// planSheetView is the usage of one Sheet kind: how many are held and the
// allowance the Plan grants.
type planSheetView struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

// PlanOption configures the optional parts of the plan view.
type PlanOption func(*PlanController)

// WithSheetUsage adds the caller's per-kind Sheet usage to GET /me/plan, read
// from the sheet service that enforces the same allowance on creation (LGN-183).
func WithSheetUsage(sheets sheet.Service) PlanOption {
	return func(pc *PlanController) {
		pc.sheets = sheets
	}
}

// WithGenerationStatus adds the consent prompt and the active generation to
// GET /me/plan (LGN-186).
func WithGenerationStatus(consent ConsentChecker, active ActiveGenerationReader) PlanOption {
	return func(pc *PlanController) {
		pc.consent = consent
		pc.active = active
	}
}

// PlanController serves the caller's Plan, its generation allowance and, when
// configured, its Sheet usage per kind and its generation status.
type PlanController struct {
	quota    *quota.Service
	resolver PlanResolver
	sheets   sheet.Service
	consent  ConsentChecker
	active   ActiveGenerationReader
}

// NewPlanController returns a controller that reads allowance from quota and
// decides the Plan with resolver. Options add the Sheet usage.
func NewPlanController(allowance *quota.Service, resolver PlanResolver, opts ...PlanOption) *PlanController {
	pc := &PlanController{quota: allowance, resolver: resolver}
	for _, opt := range opts {
		opt(pc)
	}
	return pc
}

// respond writes the view, adding the Sheet usage and the generation status when
// the controller has them. canGenerate is whether the caller has allowance left:
// only an account that can generate is asked for consent.
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

// fillGenerationStatus reads the stored consent, the consent prompt and the active
// generation. A consent refusal from the gate is the answer "required", never an
// error.
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

// isConsentRequired reports whether err is the LLM_CONSENT_REQUIRED refusal.
func isConsentRequired(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == apperr.CodeLLMConsentRequired
}

// sheetUsageView maps the service's per-kind usage onto the wire shape.
func sheetUsageView(usage sheet.PlanUsage) map[string]planSheetView {
	views := make(map[string]planSheetView, len(usage.Kinds))
	for _, status := range usage.Kinds {
		views[string(status.Kind)] = planSheetView{Used: status.Used, Limit: status.Limit}
	}
	return views
}

// RegisterRoutes mounts GET /plan on group, which the router guards with the
// JWT and user rate limit middleware.
func (pc *PlanController) RegisterRoutes(group *echo.Group) {
	group.GET("/plan", pc.Get)
}

// Get answers the caller's Plan and the matching allowance. A paid account
// reports its daily quota, a Free account its lifetime trial.
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
	return response.OK(c, planView{
		Plan: "paid",
		Generations: &planGenerationsView{
			Used:      status.Used,
			Remaining: status.Remaining,
			ResetsAt:  status.ResetsAt,
		},
	}, true)
}
