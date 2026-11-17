package campaign_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
)

type limiterHarness struct {
	plans    *mocks.MockPlanSource
	counters *mocks.MockOwnershipCounter
}

func newLimiter(t *testing.T) (*campaign.OwnershipLimiter, limiterHarness) {
	t.Helper()
	h := limiterHarness{
		plans:    mocks.NewMockPlanSource(t),
		counters: mocks.NewMockOwnershipCounter(t),
	}
	return campaign.NewOwnershipLimiter(h.plans, h.counters), h
}

func TestOwnershipLimiter_ReserveOwnership(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		emptyUser bool
		plan      plan.Plan
		planErr   error
		reserve   error
		wantLimit int
		wantCode  apperr.Code
	}{
		{name: "Happy path: Free owner reserves against limit one", plan: plan.Free, wantLimit: 1},
		{name: "Happy path: Paid owner reserves against limit five", plan: plan.Paid, wantLimit: 5},
		{name: "Edge: zero-value plan reserves against limit zero and is refused", plan: plan.Plan{}, reserve: campaign.ErrOwnershipLimitReached, wantLimit: 0, wantCode: apperr.CodePlanLimitReached},
		{name: "Edge: at the boundary the counter refuses with PLAN_LIMIT_REACHED", plan: plan.Free, reserve: campaign.ErrOwnershipLimitReached, wantLimit: 1, wantCode: apperr.CodePlanLimitReached},
		{name: "Edge: wrapped limit sentinel still maps to PLAN_LIMIT_REACHED", plan: plan.Paid, reserve: fmt.Errorf("reserve: %w", campaign.ErrOwnershipLimitReached), wantLimit: 5, wantCode: apperr.CodePlanLimitReached},
		{name: "Edge: empty user is unauthorized before any read", emptyUser: true, wantCode: apperr.CodeUnauthorized},
		{name: "Error: plan lookup failure is internal", planErr: boom, wantCode: apperr.CodeInternal},
		{name: "Error: counter failure is internal and not a limit refusal", plan: plan.Free, reserve: boom, wantLimit: 1, wantCode: apperr.CodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limiter, h := newLimiter(t)
			owner := campaigntest.NewID(t)
			if tc.emptyUser {
				requireAppErr(t, limiter.ReserveOwnership(context.Background(), ""), tc.wantCode)
				return
			}
			if tc.planErr != nil {
				h.plans.EXPECT().PlanOf(mock.Anything, owner).Return(plan.Plan{}, tc.planErr).Once()
				requireAppErr(t, limiter.ReserveOwnership(context.Background(), owner), tc.wantCode)
				return
			}
			h.plans.EXPECT().PlanOf(mock.Anything, owner).Return(tc.plan, nil).Once()
			h.counters.EXPECT().Reserve(mock.Anything, owner, tc.wantLimit).Return(tc.reserve).Once()

			err := limiter.ReserveOwnership(context.Background(), owner)
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestOwnershipLimiter_BeforeCreateUsesTheSameCheck(t *testing.T) {
	limiter, h := newLimiter(t)
	owner := campaigntest.NewID(t)
	h.plans.EXPECT().PlanOf(mock.Anything, owner).Return(plan.Free, nil).Once()
	h.counters.EXPECT().Reserve(mock.Anything, owner, 1).Return(campaign.ErrOwnershipLimitReached).Once()

	requireAppErr(t, limiter.BeforeCreate(context.Background(), owner), apperr.CodePlanLimitReached)
}

func TestOwnershipLimiter_ReleaseOwnership(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		emptyUser bool
		release   error
		wantCode  apperr.Code
	}{
		{name: "Happy path: release decrements the owner counter"},
		{name: "Edge: empty user is unauthorized before any write", emptyUser: true, wantCode: apperr.CodeUnauthorized},
		{name: "Error: counter failure is internal", release: boom, wantCode: apperr.CodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limiter, h := newLimiter(t)
			owner := campaigntest.NewID(t)
			if tc.emptyUser {
				requireAppErr(t, limiter.ReleaseOwnership(context.Background(), ""), tc.wantCode)
				return
			}
			h.counters.EXPECT().Release(mock.Anything, owner).Return(tc.release).Once()

			err := limiter.ReleaseOwnership(context.Background(), owner)
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestOwnershipLimiter_Usage(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		plan      plan.Plan
		planErr   error
		count     int
		countErr  error
		wantUsed  int
		wantLimit int
		wantCode  apperr.Code
	}{
		{name: "Happy path: Paid owner with two owned Campaigns", plan: plan.Paid, count: 2, wantUsed: 2, wantLimit: 5},
		{name: "Edge: Free owner with none owned", plan: plan.Free, count: 0, wantUsed: 0, wantLimit: 1},
		{name: "Edge: Free owner at the limit", plan: plan.Free, count: 1, wantUsed: 1, wantLimit: 1},
		{name: "Error: plan lookup failure is internal", planErr: boom, wantCode: apperr.CodeInternal},
		{name: "Error: counter failure is internal", plan: plan.Free, countErr: boom, wantCode: apperr.CodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limiter, h := newLimiter(t)
			owner := campaigntest.NewID(t)
			if tc.planErr != nil {
				h.plans.EXPECT().PlanOf(mock.Anything, owner).Return(plan.Plan{}, tc.planErr).Once()
				_, err := limiter.Usage(context.Background(), owner)
				requireAppErr(t, err, tc.wantCode)
				return
			}
			h.plans.EXPECT().PlanOf(mock.Anything, owner).Return(tc.plan, nil).Once()
			h.counters.EXPECT().Count(mock.Anything, owner).Return(tc.count, tc.countErr).Once()

			usage, err := limiter.Usage(context.Background(), owner)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantUsed, usage.Used)
			require.Equal(t, tc.wantLimit, usage.Limit)
		})
	}
}
