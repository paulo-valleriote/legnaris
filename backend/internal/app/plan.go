package app

import (
	"context"
	"time"

	"legnaris/backend/internal/auth"
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
}

// PlanController serves the caller's generation allowance.
type PlanController struct {
	quota    *quota.Service
	resolver PlanResolver
}

// NewPlanController returns a controller that reads allowance from quota and
// decides the Plan with resolver.
func NewPlanController(allowance *quota.Service, resolver PlanResolver) *PlanController {
	return &PlanController{quota: allowance, resolver: resolver}
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
	return response.OK(c, planView{
		Plan:  "free",
		Trial: &planTrialView{Used: status.Used, Remaining: status.Remaining},
	})
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
	})
}
