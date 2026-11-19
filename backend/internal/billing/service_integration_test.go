package billing_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type noSessionProvider struct {
	billing.Provider
	t *testing.T
}

func (p noSessionProvider) CreateCustomer(ctx context.Context, req billing.CustomerRequest) (billing.Customer, error) {
	assert.Nil(p.t, mongo.SessionFromContext(ctx), "the provider must be called outside the transaction")
	return p.Provider.CreateCustomer(ctx, req)
}

func (p noSessionProvider) CreateCheckoutSession(ctx context.Context, req billing.CheckoutRequest) (billing.CheckoutSession, error) {
	assert.Nil(p.t, mongo.SessionFromContext(ctx), "the provider must be called outside the transaction")
	return p.Provider.CreateCheckoutSession(ctx, req)
}

func seedBillingAccount(t *testing.T, mdb *mongo.Database, userID, storedPlan string) {
	t.Helper()
	now := time.Now().UTC()
	_, err := mdb.Collection("users").InsertOne(context.Background(), bson.M{
		"_id":        userID,
		"email":      userID + "@example.test",
		"username":   "user_" + userID,
		"plan":       storedPlan,
		"created_at": now,
		"updated_at": now,
	})
	require.NoError(t, err)
}

func planChangesFor(t *testing.T, mdb *mongo.Database, userID string) []map[string]any {
	t.Helper()
	cursor, err := mdb.Collection("event_outbox").Find(context.Background(), bson.D{
		{Key: "event_type", Value: billing.PlanChangedEvent},
	})
	require.NoError(t, err)
	var entries []outbox.Entry
	require.NoError(t, cursor.All(context.Background(), &entries))

	var changes []map[string]any
	for _, entry := range entries {
		envelope, err := events.Decode(entry.Body)
		require.NoError(t, err)
		if envelope.Aggregate.ID != userID {
			continue
		}
		var data map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &data))
		changes = append(changes, data)
	}
	return changes
}

func TestService_CheckoutCancelsExpiredComp(t *testing.T) {
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	ctx := context.Background()
	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	userID := newID(t)
	store := billing.NewMongoStore(mdb)
	eventOutbox := outbox.NewRepository(mdb, logger.NewLogger(slog.LevelError))
	tx := db.NewTransactor(client, nil)
	provider := noSessionProvider{Provider: billingtest.NewProvider(), t: t}
	service := billing.NewService(store, provider, billing.Config{
		PriceID:    "price_test_usd",
		SuccessURL: "https://app.example/billing/success",
		CancelURL:  "https://app.example/billing/cancel",
		Now:        time.Now,
		Tx:         tx,
		PlanSync: billing.PlanSyncDeps{
			Ledger: store,
			Outbox: eventOutbox,
			Tx:     tx,
		},
	})

	t.Run("Happy path: an abandoned checkout cancels the expired comp and downgrades the plan once", func(t *testing.T) {
		now := time.Now().UTC()
		seedBillingAccount(t, mdb, userID, plan.IDPaid)
		past := now.Add(-time.Hour)
		expiredID, err := idgen.New()
		require.NoError(t, err)
		require.NoError(t, store.InsertSubscription(ctx, billing.Subscription{
			ID:               expiredID,
			UserID:           userID,
			PlanID:           plan.IDPaid,
			Provider:         billing.ProviderComp,
			Status:           billing.StatusActive,
			CurrentPeriodEnd: &past,
			CreatedAt:        now,
			UpdatedAt:        now,
		}))

		checkout, err := service.Checkout(ctx, userID)
		require.NoError(t, err)
		assert.NotEmpty(t, checkout.URL)

		var account struct {
			Plan string `bson:"plan"`
		}
		require.NoError(t, mdb.Collection("users").FindOne(ctx, bson.D{{Key: "_id", Value: userID}}).Decode(&account))
		assert.Equal(t, plan.IDFree, account.Plan)

		canceled, found, err := store.FindByID(ctx, expiredID)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, billing.StatusCanceled, canceled.Status)

		live, found, err := store.FindLive(ctx, userID)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, billing.StatusPending, live.Status)

		changes := planChangesFor(t, mdb, userID)
		require.Len(t, changes, 1)
		assert.Equal(t, plan.IDPaid, changes[0]["from"])
		assert.Equal(t, plan.IDFree, changes[0]["to"])
	})
}
