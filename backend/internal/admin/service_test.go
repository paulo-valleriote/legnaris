package admin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/admin"
	"legnaris/backend/internal/admin/mocks"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	actorID  = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e01"
	targetID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e02"
	otherID  = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e03"
)

var (
	adminNow     = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	errAdminDown = errors.New("mongo down")
)

type fakeTx struct {
	calls   int
	stepErr error
}

func (f *fakeTx) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

func (f *fakeTx) Step(context.Context, db.Step) error {
	return f.stepErr
}

type rig struct {
	users  *mocks.MockUsers
	subs   *mocks.MockSubscriptions
	quotas *mocks.MockQuotas
	plans  *mocks.MockPlans
	audits *mocks.MockAudits
	tx     *fakeTx
	svc    *admin.Service
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{
		users:  mocks.NewMockUsers(t),
		subs:   mocks.NewMockSubscriptions(t),
		quotas: mocks.NewMockQuotas(t),
		plans:  mocks.NewMockPlans(t),
		audits: mocks.NewMockAudits(t),
		tx:     &fakeTx{},
	}
	r.svc = admin.NewService(admin.Deps{
		Users:         r.users,
		Subscriptions: r.subs,
		Quotas:        r.quotas,
		Plans:         r.plans,
		Audits:        r.audits,
		Tx:            r.tx,
		Now:           func() time.Time { return adminNow },
	})
	return r
}

func (r *rig) expectUser(t *testing.T, id string) {
	t.Helper()
	r.users.EXPECT().FetchByID(mock.Anything, id).
		Return(&user.User{ID: id, Email: "player@example.com"}, nil).Once()
}

func (r *rig) expectNoLive(t *testing.T, id string) {
	t.Helper()
	r.subs.EXPECT().FindLive(mock.Anything, id).Return(billing.Subscription{}, false, nil).Once()
}

func (r *rig) expectView(t *testing.T, id, planID string, live billing.Subscription, found bool) {
	t.Helper()
	r.expectUser(t, id)
	r.plans.EXPECT().PlanOf(mock.Anything, id).Return(plan.Lookup(planID), nil).Once()
	r.quotas.EXPECT().DailyStatus(mock.Anything, id).
		Return(quota.DailyStatus{Limit: 30, Used: 4, Remaining: 26, ResetsAt: adminNow.Add(12 * time.Hour)}, nil).Once()
	r.quotas.EXPECT().TrialStatus(mock.Anything, id).
		Return(quota.TrialStatus{Allowance: 3, Used: 1, Remaining: 2}, nil).Once()
	r.subs.EXPECT().FindLive(mock.Anything, id).Return(live, found, nil).Once()
}

func (r *rig) expectAudit(t *testing.T, action, reason string, match func(admin.Entry) bool) {
	t.Helper()
	r.audits.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(entry admin.Entry) bool {
		return entry.ActorUserID == actorID &&
			entry.Action == action &&
			entry.TargetUserID == targetID &&
			entry.Reason == reason &&
			entry.CreatedAt.Equal(adminNow) &&
			entry.ID != "" &&
			(match == nil || match(entry))
	})).Return(nil).Once()
}

func requireAppErr(t *testing.T, err error, code apperr.Code, status int) *apperr.Error {
	t.Helper()
	require.Error(t, err)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
	assert.Equal(t, status, appErr.HttpStatus())
	return appErr
}

func activeStripeSub() billing.Subscription {
	paidAt := adminNow.Add(-time.Hour)
	return billing.Subscription{
		ID:          otherID,
		UserID:      targetID,
		PlanID:      plan.IDPaid,
		Provider:    billing.ProviderStripe,
		Status:      billing.StatusActive,
		FirstPaidAt: &paidAt,
	}
}

func activeCompSub(end *time.Time) billing.Subscription {
	return billing.Subscription{
		ID:               otherID,
		UserID:           targetID,
		PlanID:           plan.IDPaid,
		Provider:         billing.ProviderComp,
		Status:           billing.StatusActive,
		CurrentPeriodEnd: end,
	}
}

func TestService_User(t *testing.T) {
	t.Run("Happy path: the view consolidates account, plan, subscription and quotas", func(t *testing.T) {
		r := newRig(t)
		end := adminNow.Add(24 * time.Hour)
		live := activeCompSub(&end)
		live.RefundState = billing.RefundRequested
		r.expectView(t, targetID, plan.IDPaid, live, true)

		view, err := r.svc.User(context.Background(), targetID)
		require.NoError(t, err)

		assert.Equal(t, admin.UserView{ID: targetID, Email: "player@example.com"}, view.User)
		assert.Equal(t, plan.IDPaid, view.Plan)
		require.NotNil(t, view.Subscription)
		assert.Equal(t, billing.ProviderComp, view.Subscription.Provider)
		assert.Equal(t, string(billing.RefundRequested), view.Subscription.RefundState)
		assert.Equal(t, 30, view.DailyQuota.Limit)
		assert.Equal(t, 26, view.DailyQuota.Remaining)
		assert.Equal(t, 3, view.Trial.Allowance)
	})

	t.Run("Edge: a user without a subscription reads as free with a nil subscription", func(t *testing.T) {
		r := newRig(t)
		r.expectView(t, targetID, plan.IDFree, billing.Subscription{}, false)

		view, err := r.svc.User(context.Background(), targetID)
		require.NoError(t, err)
		assert.Equal(t, plan.IDFree, view.Plan)
		assert.Nil(t, view.Subscription)
	})

	t.Run("Edge: a malformed user id is refused before storage", func(t *testing.T) {
		r := newRig(t)
		_, err := r.svc.User(context.Background(), "not-a-uuid")
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Error: an unknown user is 404", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByID(mock.Anything, targetID).Return(nil, user.ErrUserNotFound).Once()

		_, err := r.svc.User(context.Background(), targetID)
		_ = requireAppErr(t, err, apperr.CodeNotFound, 404)
	})

	t.Run("Error: an account read failure is internal", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByID(mock.Anything, targetID).Return(nil, errAdminDown).Once()

		_, err := r.svc.User(context.Background(), targetID)
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})

	t.Run("Error: a plan read failure is internal", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByID(mock.Anything, targetID).Return(&user.User{ID: targetID}, nil).Once()
		r.plans.EXPECT().PlanOf(mock.Anything, targetID).Return(plan.Plan{}, errAdminDown).Once()

		_, err := r.svc.User(context.Background(), targetID)
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})

	t.Run("Error: a subscription read failure is internal", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.plans.EXPECT().PlanOf(mock.Anything, targetID).Return(plan.Free, nil).Once()
		r.quotas.EXPECT().DailyStatus(mock.Anything, targetID).Return(quota.DailyStatus{}, nil).Once()
		r.quotas.EXPECT().TrialStatus(mock.Anything, targetID).Return(quota.TrialStatus{}, nil).Once()
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(billing.Subscription{}, false, errAdminDown).Once()

		_, err := r.svc.User(context.Background(), targetID)
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})
}

func TestService_UserByEmail(t *testing.T) {
	t.Run("Happy path: a display-name spelling resolves the canonical account", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByEmail(mock.Anything, "player@example.com").
			Return(&user.User{ID: targetID, Email: "player@example.com"}, nil).Once()
		r.plans.EXPECT().PlanOf(mock.Anything, targetID).Return(plan.Free, nil).Once()
		r.quotas.EXPECT().DailyStatus(mock.Anything, targetID).Return(quota.DailyStatus{}, nil).Once()
		r.quotas.EXPECT().TrialStatus(mock.Anything, targetID).Return(quota.TrialStatus{}, nil).Once()
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(billing.Subscription{}, false, nil).Once()

		view, err := r.svc.UserByEmail(context.Background(), " Player One <Player@Example.COM> ")
		require.NoError(t, err)
		assert.Equal(t, targetID, view.User.ID)
	})

	t.Run("Edge: a legacy mixed-case address is found by its raw spelling when the canonical form misses", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByEmail(mock.Anything, "legacy@example.com").
			Return(nil, user.ErrUserNotFound).Once()
		r.users.EXPECT().FetchByEmail(mock.Anything, "Legacy@Example.COM").
			Return(&user.User{ID: targetID, Email: "Legacy@Example.COM"}, nil).Once()
		r.plans.EXPECT().PlanOf(mock.Anything, targetID).Return(plan.Free, nil).Once()
		r.quotas.EXPECT().DailyStatus(mock.Anything, targetID).Return(quota.DailyStatus{}, nil).Once()
		r.quotas.EXPECT().TrialStatus(mock.Anything, targetID).Return(quota.TrialStatus{}, nil).Once()
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(billing.Subscription{}, false, nil).Once()

		view, err := r.svc.UserByEmail(context.Background(), "Legacy@Example.COM")
		require.NoError(t, err)
		assert.Equal(t, targetID, view.User.ID)
	})

	t.Run("Edge: a blank email is 400", func(t *testing.T) {
		r := newRig(t)
		_, err := r.svc.UserByEmail(context.Background(), "   ")
		appErr := requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
		assert.Equal(t, admin.MsgEmailRequired, appErr.Message)
	})

	t.Run("Edge: a malformed email is 400", func(t *testing.T) {
		r := newRig(t)
		_, err := r.svc.UserByEmail(context.Background(), "not-an-email")
		appErr := requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
		assert.Equal(t, admin.MsgInvalidEmail, appErr.Message)
	})

	t.Run("Error: an unknown address is 404", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByEmail(mock.Anything, "missing@example.com").
			Return(nil, user.ErrUserNotFound).Once()

		_, err := r.svc.UserByEmail(context.Background(), "missing@example.com")
		_ = requireAppErr(t, err, apperr.CodeNotFound, 404)
	})
}

func TestService_GrantComp(t *testing.T) {
	future := adminNow.Add(48 * time.Hour)

	t.Run("Happy path: an active comp is written with the plan, expiry and audit in one transaction", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.expectNoLive(t, targetID)
		r.subs.EXPECT().InsertSubscription(mock.Anything, mock.MatchedBy(func(sub billing.Subscription) bool {
			return sub.UserID == targetID &&
				sub.PlanID == plan.IDPaid &&
				sub.Provider == billing.ProviderComp &&
				sub.Status == billing.StatusActive &&
				sub.CurrentPeriodEnd != nil && sub.CurrentPeriodEnd.Equal(future) &&
				sub.ID != ""
		})).Return(nil).Once()
		r.expectAudit(t, admin.ActionCompGrant, "support comp", func(entry admin.Entry) bool {
			return entry.Payload["plan_id"] == plan.IDPaid
		})

		sub, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{
			PlanID:    plan.IDPaid,
			ExpiresAt: &future,
			Reason:    "  support comp  ",
		})
		require.NoError(t, err)
		assert.Equal(t, billing.ProviderComp, sub.Provider)
		assert.Equal(t, 1, r.tx.calls)
	})

	t.Run("Happy path: a comp without an expiry never lapses", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.expectNoLive(t, targetID)
		r.subs.EXPECT().InsertSubscription(mock.Anything, mock.MatchedBy(func(sub billing.Subscription) bool {
			return sub.CurrentPeriodEnd == nil
		})).Return(nil).Once()
		r.expectAudit(t, admin.ActionCompGrant, "vip", nil)

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "vip"})
		require.NoError(t, err)
	})

	t.Run("Edge: a blank reason is 400 before any write", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "   "})
		appErr := requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
		assert.Equal(t, admin.MsgReasonRequired, appErr.Message)
		assert.Zero(t, r.tx.calls)
	})

	t.Run("Edge: a reason above the cap is 400", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{
			PlanID: plan.IDPaid,
			Reason: string(make([]byte, admin.MaxReasonLength+1)),
		})
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
		assert.Zero(t, r.tx.calls)
	})

	t.Run("Edge: the free plan cannot be granted as a comp", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDFree, Reason: "x"})
		appErr := requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
		assert.Equal(t, admin.MsgInvalidPlan, appErr.Message)
	})

	t.Run("Edge: an unknown plan is 400", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: "enterprise", Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Edge: an expiry in the past is 400", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		past := adminNow.Add(-time.Minute)

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, ExpiresAt: &past, Reason: "x"})
		appErr := requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
		assert.Equal(t, admin.MsgExpiryNotFuture, appErr.Message)
	})

	t.Run("Edge: an expiry exactly now is 400", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		now := adminNow

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, ExpiresAt: &now, Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Error: a malformed target id is 400", func(t *testing.T) {
		r := newRig(t)
		_, err := r.svc.GrantComp(context.Background(), actorID, "nope", admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Error: an unknown target user is 404", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByID(mock.Anything, targetID).Return(nil, user.ErrUserNotFound).Once()

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeNotFound, 404)
	})

	t.Run("Edge: an active stripe subscription is 409 and writes nothing", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeStripeSub(), true, nil).Once()

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		appErr := requireAppErr(t, err, apperr.CodeConflict, 409)
		assert.Equal(t, admin.MsgLiveSubscriptionExist, appErr.Message)
		assert.Zero(t, r.tx.calls)
	})

	t.Run("Edge: an active comp is 409", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		end := adminNow.Add(time.Hour)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeCompSub(&end), true, nil).Once()

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeConflict, 409)
	})

	t.Run("Edge: a pending live subscription loses the unique-index race as 409", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).
			Return(billing.Subscription{ID: otherID, Status: billing.StatusPending}, true, nil).Once()
		r.subs.EXPECT().InsertSubscription(mock.Anything, mock.Anything).Return(billing.ErrLiveExists).Once()

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeConflict, 409)
	})

	t.Run("Error: a write failure is internal and writes no audit", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.expectNoLive(t, targetID)
		r.subs.EXPECT().InsertSubscription(mock.Anything, mock.Anything).Return(errAdminDown).Once()

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})

	t.Run("Error: a crashed step aborts before the audit row", func(t *testing.T) {
		r := newRig(t)
		r.tx.stepErr = errAdminDown
		r.expectUser(t, targetID)
		r.expectNoLive(t, targetID)
		r.subs.EXPECT().InsertSubscription(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := r.svc.GrantComp(context.Background(), actorID, targetID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})
}

func TestService_RevokeComp(t *testing.T) {
	t.Run("Happy path: the live comp is canceled and audited", func(t *testing.T) {
		r := newRig(t)
		end := adminNow.Add(time.Hour)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeCompSub(&end), true, nil).Once()
		r.subs.EXPECT().CancelComp(mock.Anything, otherID, adminNow).Return(true, nil).Once()
		r.expectAudit(t, admin.ActionCompRevoke, "abuse", nil)

		require.NoError(t, r.svc.RevokeComp(context.Background(), actorID, targetID, " abuse "))
		assert.Equal(t, 1, r.tx.calls)
	})

	t.Run("Edge: a blank reason is 400 before the lookup", func(t *testing.T) {
		r := newRig(t)
		err := r.svc.RevokeComp(context.Background(), actorID, targetID, "")
		appErr := requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
		assert.Equal(t, admin.MsgReasonRequired, appErr.Message)
	})

	t.Run("Edge: an invalid target id is 400", func(t *testing.T) {
		r := newRig(t)
		err := r.svc.RevokeComp(context.Background(), actorID, "nope", "x")
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Edge: a user without a live subscription is 404", func(t *testing.T) {
		r := newRig(t)
		r.expectNoLive(t, targetID)

		err := r.svc.RevokeComp(context.Background(), actorID, targetID, "x")
		appErr := requireAppErr(t, err, apperr.CodeNotFound, 404)
		assert.Equal(t, admin.MsgCompNotFound, appErr.Message)
	})

	t.Run("Edge: a live stripe subscription is not a comp and is 409", func(t *testing.T) {
		r := newRig(t)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeStripeSub(), true, nil).Once()

		err := r.svc.RevokeComp(context.Background(), actorID, targetID, "x")
		appErr := requireAppErr(t, err, apperr.CodeConflict, 409)
		assert.Equal(t, admin.MsgNotComp, appErr.Message)
	})

	t.Run("Edge: a concurrent cancel is 409", func(t *testing.T) {
		r := newRig(t)
		end := adminNow.Add(time.Hour)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeCompSub(&end), true, nil).Once()
		r.subs.EXPECT().CancelComp(mock.Anything, otherID, adminNow).Return(false, nil).Once()

		err := r.svc.RevokeComp(context.Background(), actorID, targetID, "x")
		appErr := requireAppErr(t, err, apperr.CodeConflict, 409)
		assert.Equal(t, admin.MsgCompStateChanged, appErr.Message)
	})

	t.Run("Error: a cancel failure is internal", func(t *testing.T) {
		r := newRig(t)
		end := adminNow.Add(time.Hour)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeCompSub(&end), true, nil).Once()
		r.subs.EXPECT().CancelComp(mock.Anything, otherID, adminNow).Return(false, errAdminDown).Once()

		err := r.svc.RevokeComp(context.Background(), actorID, targetID, "x")
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})

	t.Run("Error: an audit failure is internal", func(t *testing.T) {
		r := newRig(t)
		end := adminNow.Add(time.Hour)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeCompSub(&end), true, nil).Once()
		r.subs.EXPECT().CancelComp(mock.Anything, otherID, adminNow).Return(true, nil).Once()
		r.audits.EXPECT().Insert(mock.Anything, mock.Anything).Return(errAdminDown).Once()

		err := r.svc.RevokeComp(context.Background(), actorID, targetID, "x")
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})
}

func TestService_ResetQuota(t *testing.T) {
	t.Run("Happy path: today's counter is deleted and audited", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.quotas.EXPECT().ResetDaily(mock.Anything, targetID).Return(nil).Once()
		r.expectAudit(t, admin.ActionQuotaReset, "support", nil)

		require.NoError(t, r.svc.ResetQuota(context.Background(), actorID, targetID, "support"))
		assert.Equal(t, 1, r.tx.calls)
	})

	t.Run("Edge: a second reset is a success because the delete is idempotent", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByID(mock.Anything, targetID).
			Return(&user.User{ID: targetID}, nil).Twice()
		r.quotas.EXPECT().ResetDaily(mock.Anything, targetID).Return(nil).Twice()
		r.audits.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Twice()

		require.NoError(t, r.svc.ResetQuota(context.Background(), actorID, targetID, "x"))
		require.NoError(t, r.svc.ResetQuota(context.Background(), actorID, targetID, "x"))
	})

	t.Run("Edge: a blank reason is 400 before any reset", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)

		err := r.svc.ResetQuota(context.Background(), actorID, targetID, " ")
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Error: an unknown user is 404 before any reset", func(t *testing.T) {
		r := newRig(t)
		r.users.EXPECT().FetchByID(mock.Anything, targetID).Return(nil, user.ErrUserNotFound).Once()

		err := r.svc.ResetQuota(context.Background(), actorID, targetID, "x")
		_ = requireAppErr(t, err, apperr.CodeNotFound, 404)
	})

	t.Run("Error: a store failure is internal and writes no audit", func(t *testing.T) {
		r := newRig(t)
		r.expectUser(t, targetID)
		r.quotas.EXPECT().ResetDaily(mock.Anything, targetID).Return(errAdminDown).Once()

		err := r.svc.ResetQuota(context.Background(), actorID, targetID, "x")
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})
}

func TestService_Refund(t *testing.T) {
	t.Run("Happy path: an in-window paid subscription queues a refund and audits it", func(t *testing.T) {
		r := newRig(t)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeStripeSub(), true, nil).Once()
		r.subs.EXPECT().RequestRefund(mock.Anything, otherID, adminNow).Return(true, nil).Once()
		r.expectAudit(t, admin.ActionRefundRequest, "goodwill", func(entry admin.Entry) bool {
			return entry.Payload["override"] == false
		})

		require.NoError(t, r.svc.Refund(context.Background(), actorID, targetID, false, "goodwill"))
		assert.Equal(t, 1, r.tx.calls)
	})

	t.Run("Happy path: override skips the window for a paid subscription", func(t *testing.T) {
		r := newRig(t)
		old := adminNow.Add(-30 * 24 * time.Hour)
		live := activeStripeSub()
		live.FirstPaidAt = &old
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(live, true, nil).Once()
		r.subs.EXPECT().RequestRefund(mock.Anything, otherID, adminNow).Return(true, nil).Once()
		r.expectAudit(t, admin.ActionRefundRequest, "exception", func(entry admin.Entry) bool {
			return entry.Payload["override"] == true
		})

		require.NoError(t, r.svc.Refund(context.Background(), actorID, targetID, true, "exception"))
	})

	t.Run("Edge: outside the window without override is refused with the window reason", func(t *testing.T) {
		r := newRig(t)
		old := adminNow.Add(-billing.RefundWindow)
		live := activeStripeSub()
		live.FirstPaidAt = &old
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(live, true, nil).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, false, "x")
		appErr := requireAppErr(t, err, apperr.CodeRefundNotEligible, 409)
		assert.Equal(t, billing.ReasonWindowExpired, appErr.Details["reason"])
		assert.Zero(t, r.tx.calls)
	})

	t.Run("Edge: a user without a subscription is refused as not subscribed", func(t *testing.T) {
		r := newRig(t)
		r.expectNoLive(t, targetID)

		err := r.svc.Refund(context.Background(), actorID, targetID, true, "x")
		appErr := requireAppErr(t, err, apperr.CodeRefundNotEligible, 409)
		assert.Equal(t, billing.ReasonNotSubscribed, appErr.Details["reason"])
	})

	t.Run("Edge: a pending subscription is not a paid subscription", func(t *testing.T) {
		r := newRig(t)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).
			Return(billing.Subscription{ID: otherID, Status: billing.StatusPending}, true, nil).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, true, "x")
		appErr := requireAppErr(t, err, apperr.CodeRefundNotEligible, 409)
		assert.Equal(t, billing.ReasonNotSubscribed, appErr.Details["reason"])
	})

	t.Run("Edge: a comp subscription is never refundable, even with override", func(t *testing.T) {
		r := newRig(t)
		end := adminNow.Add(time.Hour)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeCompSub(&end), true, nil).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, true, "x")
		appErr := requireAppErr(t, err, apperr.CodeRefundNotEligible, 409)
		assert.Equal(t, billing.ReasonCompNotRefundable, appErr.Details["reason"])
	})

	t.Run("Edge: a subscription whose right is already used is refused, even with override", func(t *testing.T) {
		r := newRig(t)
		live := activeStripeSub()
		used := adminNow.Add(-time.Hour)
		live.RefundUsedAt = &used
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(live, true, nil).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, true, "x")
		appErr := requireAppErr(t, err, apperr.CodeRefundNotEligible, 409)
		assert.Equal(t, billing.ReasonAlreadyUsed, appErr.Details["reason"])
	})

	t.Run("Edge: a concurrent refund that loses the claim is already used", func(t *testing.T) {
		r := newRig(t)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeStripeSub(), true, nil).Once()
		r.subs.EXPECT().RequestRefund(mock.Anything, otherID, adminNow).Return(false, nil).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, true, "x")
		appErr := requireAppErr(t, err, apperr.CodeRefundNotEligible, 409)
		assert.Equal(t, billing.ReasonAlreadyUsed, appErr.Details["reason"])
	})

	t.Run("Edge: the partial unique index reports a double refund", func(t *testing.T) {
		r := newRig(t)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeStripeSub(), true, nil).Once()
		r.subs.EXPECT().RequestRefund(mock.Anything, otherID, adminNow).Return(false, billing.ErrRefundAlreadyUsed).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, true, "x")
		appErr := requireAppErr(t, err, apperr.CodeRefundNotEligible, 409)
		assert.Equal(t, billing.ReasonAlreadyUsed, appErr.Details["reason"])
	})

	t.Run("Edge: a blank reason is 400 before the lookup", func(t *testing.T) {
		r := newRig(t)
		err := r.svc.Refund(context.Background(), actorID, targetID, false, "")
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Edge: a malformed target id is 400", func(t *testing.T) {
		r := newRig(t)
		err := r.svc.Refund(context.Background(), actorID, "nope", false, "x")
		_ = requireAppErr(t, err, apperr.CodeInvalidRequest, 400)
	})

	t.Run("Error: a refund queue write failure is internal", func(t *testing.T) {
		r := newRig(t)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(activeStripeSub(), true, nil).Once()
		r.subs.EXPECT().RequestRefund(mock.Anything, otherID, adminNow).Return(false, errAdminDown).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, false, "x")
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})

	t.Run("Error: a live lookup failure is internal", func(t *testing.T) {
		r := newRig(t)
		r.subs.EXPECT().FindLive(mock.Anything, targetID).Return(billing.Subscription{}, false, errAdminDown).Once()

		err := r.svc.Refund(context.Background(), actorID, targetID, false, "x")
		_ = requireAppErr(t, err, apperr.CodeInternal, 500)
	})
}
