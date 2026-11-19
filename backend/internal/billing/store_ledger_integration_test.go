package billing_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const integrationNow = "2026-11-16T10:00:00Z"

func ledgerDB(t *testing.T) (*mongo.Client, *mongo.Database) {
	t.Helper()
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return client, mdb
}

func seedAccount(t *testing.T, mdb *mongo.Database, userID string) {
	t.Helper()
	_, err := mdb.Collection("users").InsertOne(context.Background(), bson.D{{Key: "_id", Value: userID}, {Key: "email", Value: userID + "@example.test"}, {Key: "username", Value: "user_" + userID}})
	require.NoError(t, err)
}

func newID(t *testing.T) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return id
}

func TestMongoLedger_UserLease(t *testing.T) {
	t.Run("Edge: concurrent claims for one user let exactly one consumer in", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		userID := newID(t)
		at := mustParse(t, integrationNow)
		require.NoError(t, store.InsertEvent(context.Background(), billing.Event{
			ID: newID(t), Provider: billing.ProviderStripe, ProviderEventID: "evt_lease_1",
			Kind: billing.KindSubscriptionChanged, UserID: userID, Status: billing.EventReceived,
			NextAttemptAt: at, ReceivedAt: at, ExpiresAt: at.Add(time.Hour),
		}))
		require.NoError(t, store.InsertEvent(context.Background(), billing.Event{
			ID: newID(t), Provider: billing.ProviderStripe, ProviderEventID: "evt_lease_2",
			Kind: billing.KindSubscriptionChanged, UserID: userID, Status: billing.EventReceived,
			NextAttemptAt: at, ReceivedAt: at, ExpiresAt: at.Add(time.Hour),
		}))

		var wg sync.WaitGroup
		results := make(chan int, 4)
		for i := range 4 {
			wg.Add(1)
			go func(claimer string) {
				defer wg.Done()
				claimed, err := store.ClaimEvents(context.Background(), claimer, at, time.Minute, 10)
				assert.NoError(t, err)
				results <- len(claimed)
			}(string(rune('a' + i)))
		}
		wg.Wait()
		close(results)

		total := 0
		for n := range results {
			total += n
		}
		assert.Equal(t, 1, total)
	})

	t.Run("Edge: a released lease can be claimed again, an expired one is reclaimed", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		userID := newID(t)
		at := mustParse(t, integrationNow)
		require.NoError(t, store.InsertEvent(context.Background(), billing.Event{
			ID: newID(t), Provider: billing.ProviderStripe, ProviderEventID: "evt_lease_3",
			Kind: billing.KindSubscriptionChanged, UserID: userID, Status: billing.EventReceived,
			NextAttemptAt: at, ReceivedAt: at, ExpiresAt: at.Add(time.Hour),
		}))
		ctx := context.Background()

		first, err := store.ClaimEvents(ctx, "relay-1", at, time.Minute, 10)
		require.NoError(t, err)
		require.Len(t, first, 1)
		require.NoError(t, store.ReleaseUser(ctx, userID, "relay-1"))

		second, err := store.ClaimEvents(ctx, "relay-2", at, time.Minute, 10)
		require.NoError(t, err)
		require.Len(t, second, 1)

		expired, err := store.ClaimEvents(ctx, "relay-3", at.Add(2*time.Minute), time.Minute, 10)
		require.NoError(t, err)
		assert.Len(t, expired, 1)
	})
}

func TestMongoLedger_Events(t *testing.T) {
	t.Run("Error: a second delivery of the same provider event is a duplicate", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		at := mustParse(t, integrationNow)
		ev := billing.Event{
			ID: newID(t), Provider: billing.ProviderStripe, ProviderEventID: "evt_dup",
			Kind: billing.KindIgnored, Status: billing.EventProcessed, ReceivedAt: at, NextAttemptAt: at, ExpiresAt: at.Add(time.Hour),
		}
		require.NoError(t, store.InsertEvent(context.Background(), ev))
		ev.ID = newID(t)
		assert.ErrorIs(t, store.InsertEvent(context.Background(), ev), billing.ErrDuplicateEvent)
	})
}

func TestMongoLedger_Subscriptions(t *testing.T) {
	t.Run("Edge: a stale provider read does not overwrite a newer synced state", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		at := mustParse(t, integrationNow)
		userID := newID(t)
		subID := newID(t)
		fresh := at.Add(time.Hour)
		require.NoError(t, store.InsertSubscription(context.Background(), billing.Subscription{
			ID: subID, UserID: userID, PlanID: "paid", Provider: billing.ProviderStripe,
			Status: billing.StatusActive, ProviderSubID: "sub_stale", ProviderSyncedAt: &fresh,
			CreatedAt: at, UpdatedAt: at,
		}))

		applied, err := store.ApplySubscription(context.Background(), billing.Subscription{
			ID: subID, Status: billing.StatusPastDue, ProviderSubID: "sub_stale", ProviderSyncedAt: &at, UpdatedAt: at,
		}, at)
		require.NoError(t, err)
		assert.False(t, applied)

		target, found, err := store.FindTarget(context.Background(), userID, "sub_stale")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, billing.StatusActive, target.Status)
	})

	t.Run("Error: a second live subscription for one user is rejected by the unique index", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		at := mustParse(t, integrationNow)
		userID := newID(t)
		base := billing.Subscription{UserID: userID, PlanID: "paid", Provider: billing.ProviderStripe, Status: billing.StatusPending, CreatedAt: at, UpdatedAt: at}
		base.ID = newID(t)
		require.NoError(t, store.InsertSubscription(context.Background(), base))
		base.ID = newID(t)
		assert.ErrorIs(t, store.InsertSubscription(context.Background(), base), billing.ErrLiveExists)
	})
}

func TestMongoConsumer_EndToEnd(t *testing.T) {
	t.Run("Happy path: an active read derives paid, bumps the version and writes plan.changed in one transaction", func(t *testing.T) {
		client, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		at := mustParse(t, integrationNow)
		userID := newID(t)
		seedAccount(t, mdb, userID)
		subID := newID(t)
		require.NoError(t, store.InsertPending(context.Background(), billing.Subscription{
			ID: subID, UserID: userID, PlanID: "paid", Provider: billing.ProviderStripe,
			ProviderCustomerID: "cus_e2e", Status: billing.StatusPending, CreatedAt: at, UpdatedAt: at,
		}))
		provider := billingtest.NewProvider()
		provider.SetSubscription(billing.ProviderSubscription{ID: "sub_e2e", CustomerID: "cus_e2e", Status: billing.StatusActive, CurrentPeriodEnd: at.AddDate(0, 1, 0)})
		consumer := billing.NewConsumer(billing.ConsumerDeps{
			Provider: provider,
			Ledger:   store,
			Tx:       db.NewTransactor(client, nil),
			Outbox:   outbox.NewRepository(mdb, logger.NewLogger(slog.LevelError)),
			Now:      func() time.Time { return at },
		})
		eventID := newID(t)
		ev := billing.Event{
			ID: eventID, Provider: billing.ProviderStripe, ProviderEventID: "evt_e2e",
			Kind: billing.KindSubscriptionChanged, UserID: userID, ProviderSubscriptionID: "sub_e2e",
			Status: billing.EventReceived, ReceivedAt: at, NextAttemptAt: at, ExpiresAt: at.Add(time.Hour),
		}
		require.NoError(t, store.InsertEvent(context.Background(), ev))
		claimed, err := store.ClaimEvents(context.Background(), "relay-e2e", at, time.Minute, 10)
		require.NoError(t, err)
		require.Len(t, claimed, 1)

		require.NoError(t, consumer.Handle(context.Background(), sweeper.Job[billing.Event]{ID: eventID, Payload: claimed[0]}, at))

		var user struct {
			Plan        string `bson:"plan"`
			PlanVersion int64  `bson:"plan_version"`
		}
		require.NoError(t, mdb.Collection("users").FindOne(context.Background(), bson.D{{Key: "_id", Value: userID}}).Decode(&user))
		assert.Equal(t, "paid", user.Plan)
		assert.EqualValues(t, 1, user.PlanVersion)

		count, err := mdb.Collection("event_outbox").CountDocuments(context.Background(), bson.D{})
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)

		var entries []outbox.Entry
		cursor, err := mdb.Collection("event_outbox").Find(context.Background(), bson.D{})
		require.NoError(t, err)
		require.NoError(t, cursor.All(context.Background(), &entries))
		envelope, err := events.Decode(entries[0].Body)
		require.NoError(t, err)
		assert.Equal(t, billing.PlanChangedEvent, envelope.Type)
		assert.Equal(t, userID, envelope.Aggregate.ID)
		var data map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &data))
		assert.Equal(t, "free", data["from"])
		assert.Equal(t, "paid", data["to"])

		replayed := claimed[0]
		require.NoError(t, consumer.Handle(context.Background(), sweeper.Job[billing.Event]{ID: eventID, Payload: replayed}, at))
		count, err = mdb.Collection("event_outbox").CountDocuments(context.Background(), bson.D{})
		require.NoError(t, err)
		assert.EqualValues(t, 1, count, "a replay must not emit plan.changed again")
	})
}

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return at
}
