package admin_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/admin"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var errInjected = errors.New("injected crash")

func newAdminStack(t *testing.T, hook db.StepHook) (*admin.Service, user.Repository, *billing.MongoStore, *mongo.Database) {
	t.Helper()
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	ctx := context.Background()
	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	now := time.Now().UTC()
	clock := func() time.Time { return now }
	log := logger.NewLogger(slog.LevelError)
	users := user.NewRepository(mdb, log)
	billingStore := billing.NewMongoStore(mdb)
	quotaService := quota.NewService(quota.NewMongoStore(mdb), clock)
	planSource := user.NewPlanSource(users).
		WithDeriver(billing.NewPlanReader(billingStore, clock).EffectivePlanID)

	service := admin.NewService(admin.Deps{
		Users:         users,
		Subscriptions: billingStore,
		Quotas:        quotaService,
		Plans:         planSource,
		Audits:        admin.NewMongoStore(mdb),
		Tx:            db.NewTransactor(client, hook),
		Now:           clock,
	})
	return service, users, billingStore, mdb
}

func createAccount(t *testing.T, users user.Repository) *user.User {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	account := &user.User{
		ID:        id,
		Email:     id + "@example.com",
		Username:  "u" + id[len(id)-8:],
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, users.Create(context.Background(), account))
	return account
}

func auditCount(t *testing.T, mdb *mongo.Database, target, action string) int64 {
	t.Helper()
	filter := bson.D{{Key: "target_user_id", Value: target}}
	if action != "" {
		filter = append(filter, bson.E{Key: "action", Value: action})
	}
	count, err := mdb.Collection(admin.AuditCollection).CountDocuments(context.Background(), filter)
	require.NoError(t, err)
	return count
}

func TestService_AuditAndTransactions(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a comp grant commits the subscription and one audit row", func(t *testing.T) {
		service, users, _, mdb := newAdminStack(t, nil)
		target := createAccount(t, users)
		expires := time.Now().UTC().Add(24 * time.Hour)

		sub, err := service.GrantComp(ctx, actorID, target.ID, admin.CompGrant{
			PlanID:    plan.IDPaid,
			ExpiresAt: &expires,
			Reason:    "support comp",
		})
		require.NoError(t, err)

		var stored admin.Entry
		require.NoError(t, mdb.Collection(admin.AuditCollection).
			FindOne(ctx, bson.D{{Key: "target_user_id", Value: target.ID}}).Decode(&stored))
		assert.Equal(t, actorID, stored.ActorUserID)
		assert.Equal(t, admin.ActionCompGrant, stored.Action)
		assert.Equal(t, "support comp", stored.Reason)
		assert.Equal(t, plan.IDPaid, stored.Payload["plan_id"])
		assert.NotEqual(t, sub.ID, "")

		view, err := service.User(ctx, target.ID)
		require.NoError(t, err)
		assert.Equal(t, plan.IDPaid, view.Plan)
		require.NotNil(t, view.Subscription)
		assert.Equal(t, billing.ProviderComp, view.Subscription.Provider)
	})

	t.Run("Edge: a crash after the comp write rolls back the subscription and the audit", func(t *testing.T) {
		service, users, billingStore, mdb := newAdminStack(t, func(context.Context, db.Step) error {
			return errInjected
		})
		target := createAccount(t, users)

		_, err := service.GrantComp(ctx, actorID, target.ID, admin.CompGrant{PlanID: plan.IDPaid, Reason: "x"})
		require.Error(t, err)

		_, found, err := billingStore.FindLive(ctx, target.ID)
		require.NoError(t, err)
		assert.False(t, found, "the comp must be rolled back")
		assert.Zero(t, auditCount(t, mdb, target.ID, admin.ActionCompGrant))
	})

	t.Run("Edge: a crash after the quota reset rolls back the counter and the audit", func(t *testing.T) {
		service, users, _, mdb := newAdminStack(t, func(context.Context, db.Step) error {
			return errInjected
		})
		target := createAccount(t, users)
		day := time.Now().UTC().Format("2006-01-02")
		usage := mdb.Collection(quota.UsageCollection)
		_, err := usage.InsertOne(ctx, bson.M{
			"user_id":    target.ID,
			"day_utc":    day,
			"count":      5,
			"expires_at": time.Now().UTC().Add(48 * time.Hour),
		})
		require.NoError(t, err)

		require.Error(t, service.ResetQuota(ctx, actorID, target.ID, "x"))

		count, err := usage.CountDocuments(ctx, bson.D{
			{Key: "user_id", Value: target.ID},
			{Key: "day_utc", Value: day},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "the counter must survive the rollback")
		assert.Zero(t, auditCount(t, mdb, target.ID, admin.ActionQuotaReset))
	})

	t.Run("Happy path: a refund request commits refund_state and the audit row", func(t *testing.T) {
		service, users, billingStore, mdb := newAdminStack(t, nil)
		target := createAccount(t, users)
		paidAt := time.Now().UTC().Add(-time.Hour)
		sub := billing.Subscription{
			ID:          mustID(t),
			UserID:      target.ID,
			PlanID:      plan.IDPaid,
			Provider:    billing.ProviderStripe,
			Status:      billing.StatusActive,
			FirstPaidAt: &paidAt,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		require.NoError(t, billingStore.InsertSubscription(ctx, sub))

		require.NoError(t, service.Refund(ctx, actorID, target.ID, false, "goodwill"))

		live, found, err := billingStore.FindLive(ctx, target.ID)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, billing.RefundRequested, live.RefundState)
		require.NotNil(t, live.RefundUsedAt)
		assert.Equal(t, int64(1), auditCount(t, mdb, target.ID, admin.ActionRefundRequest))

		err = service.Refund(ctx, actorID, target.ID, true, "again")
		require.Error(t, err)
		assert.Equal(t, int64(1), auditCount(t, mdb, target.ID, admin.ActionRefundRequest), "a refused refund writes no audit")
	})

	t.Run("Edge: an expired comp is ignored by the effective plan before the sweeper runs", func(t *testing.T) {
		service, users, billingStore, mdb := newAdminStack(t, nil)
		target := createAccount(t, users)
		past := time.Now().UTC().Add(-time.Hour)
		require.NoError(t, billingStore.InsertSubscription(ctx, billing.Subscription{
			ID:               mustID(t),
			UserID:           target.ID,
			PlanID:           plan.IDPaid,
			Provider:         billing.ProviderComp,
			Status:           billing.StatusActive,
			CurrentPeriodEnd: &past,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}))

		view, err := service.User(ctx, target.ID)
		require.NoError(t, err)
		assert.Equal(t, plan.IDFree, view.Plan)

		n, err := billingStore.CancelExpiredComps(ctx, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		canceled, found, err := billingStore.FindLive(ctx, target.ID)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Equal(t, billing.StatusCanceled, canceled.Status)
		assert.Zero(t, auditCount(t, mdb, target.ID, ""), "the sweeper does not write admin audit rows")
	})
}

func mustID(t *testing.T) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return id
}
