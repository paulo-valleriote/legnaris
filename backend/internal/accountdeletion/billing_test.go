package accountdeletion_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"legnaris/backend/internal/accountdeletion"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
)

const (
	billingVictimSub = "01900000-0000-7000-8000-0000000000b1"
	billingOtherSub  = "01900000-0000-7000-8000-0000000000b2"
	billingVictimEv  = "01900000-0000-7000-8000-0000000000e1"
	billingOtherEv   = "01900000-0000-7000-8000-0000000000e2"
	billingLateEv    = "01900000-0000-7000-8000-0000000000e3"
)

func seedBilling(t *testing.T, mdb *mongo.Database, userID, subID, eventID string) {
	t.Helper()
	ctx := context.Background()
	seeds := map[string]bson.D{
		billing.SubscriptionsCollection: {
			{Key: "_id", Value: subID},
			{Key: "user_id", Value: userID},
			{Key: "plan_id", Value: "paid"},
			{Key: "provider", Value: "stripe"},
			{Key: "provider_customer_id", Value: "cus_" + userID},
			{Key: "provider_subscription_id", Value: "sub_" + userID},
			{Key: "status", Value: "active"},
		},
		billing.EventsCollection: {
			{Key: "_id", Value: eventID},
			{Key: "provider", Value: "stripe"},
			{Key: "provider_event_id", Value: "evt_" + eventID},
			{Key: "user_id", Value: userID},
			{Key: "status", Value: "received"},
		},
		billing.UserLocksCollection: {
			{Key: "_id", Value: userID},
			{Key: "locked_until", Value: deleteAt},
		},
	}
	for collection, doc := range seeds {
		_, err := mdb.Collection(collection).InsertOne(ctx, doc)
		require.NoError(t, err, collection)
	}
}

func subscriptionOf(t *testing.T, mdb *mongo.Database, subID string) bson.M {
	t.Helper()
	var doc bson.M
	require.NoError(t, mdb.Collection(billing.SubscriptionsCollection).FindOne(context.Background(), bson.D{{Key: "_id", Value: subID}}).Decode(&doc))
	return doc
}

func TestCascade_RemoveBilling(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, mdb *mongo.Database, cascade *accountdeletion.Cascade)
	}{
		{
			name: "Happy path: the account's billing events and lease are removed, its subscriptions are kept canceled for the fiscal retention, the other account is untouched",
			run: func(t *testing.T, mdb *mongo.Database, cascade *accountdeletion.Cascade) {
				seedAccount(t, mdb, victimID, victimEmail)
				seedAccount(t, mdb, otherID, otherEmail)
				seedBilling(t, mdb, victimID, billingVictimSub, billingVictimEv)
				seedBilling(t, mdb, otherID, billingOtherSub, billingOtherEv)

				require.NoError(t, cascade.Remove(context.Background(), victimID))

				assert.Zero(t, countOwned(t, mdb, billing.EventsCollection, "user_id", victimID))
				assert.Zero(t, countOwned(t, mdb, billing.UserLocksCollection, "_id", victimID))

				kept := subscriptionOf(t, mdb, billingVictimSub)
				assert.Equal(t, "canceled", kept["status"])
				assert.Equal(t, "sub_"+victimID, kept["provider_subscription_id"], "the provider reference is kept as the fiscal record")
				assert.True(t, kept["expires_at"].(bson.DateTime).Time().Equal(billing.RetentionUntil(deleteAt)))

				assert.Equal(t, int64(1), countOwned(t, mdb, billing.EventsCollection, "user_id", otherID))
				assert.Equal(t, int64(1), countOwned(t, mdb, billing.UserLocksCollection, "_id", otherID))
				untouched := subscriptionOf(t, mdb, billingOtherSub)
				assert.Equal(t, "active", untouched["status"])
				assert.NotContains(t, untouched, "expires_at")
			},
		},
		{
			name: "Edge: an account with billing events but no subscription loses its events and keeps no fiscal row",
			run: func(t *testing.T, mdb *mongo.Database, cascade *accountdeletion.Cascade) {
				seedAccount(t, mdb, victimID, victimEmail)
				_, err := mdb.Collection(billing.EventsCollection).InsertOne(context.Background(),
					bson.D{{Key: "_id", Value: billingVictimEv}, {Key: "user_id", Value: victimID}, {Key: "status", Value: "received"}})
				require.NoError(t, err)

				require.NoError(t, cascade.Remove(context.Background(), victimID))

				assert.Zero(t, countOwned(t, mdb, billing.EventsCollection, "user_id", victimID))
				assert.Zero(t, countOwned(t, mdb, billing.SubscriptionsCollection, "user_id", victimID))
			},
		},
		{
			name: "Edge: a repeat removal after a commit does not move the retention deadline",
			run: func(t *testing.T, mdb *mongo.Database, cascade *accountdeletion.Cascade) {
				seedAccount(t, mdb, victimID, victimEmail)
				seedBilling(t, mdb, victimID, billingVictimSub, billingVictimEv)

				require.NoError(t, cascade.Remove(context.Background(), victimID))
				later := accountdeletion.NewCascade(mdb, newOutboxRepo(mdb), func() time.Time { return deleteAt.AddDate(1, 0, 0) })
				require.NoError(t, later.Remove(context.Background(), victimID))

				kept := subscriptionOf(t, mdb, billingVictimSub)
				assert.True(t, kept["expires_at"].(bson.DateTime).Time().Equal(billing.RetentionUntil(deleteAt)))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			tc.run(t, mdb, newCascade(mdb, newOutboxRepo(mdb)))
		})
	}
}

func TestBillingEventsOfDeletedAccountAreIgnored(t *testing.T) {
	tests := []struct {
		name       string
		deleteUser bool
		wantReason string
	}{
		{
			name:       "Happy path: a provider event for a removed account is marked processed and ignored without a provider read",
			deleteUser: true,
			wantReason: billing.ReasonIgnored,
		},
		{
			name:       "Edge: an event whose account still exists is applied, not ignored",
			deleteUser: false,
			wantReason: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			ctx := context.Background()
			seedAccount(t, mdb, victimID, victimEmail)
			seedBilling(t, mdb, victimID, billingVictimSub, billingVictimEv)

			provider := billingtest.NewProvider()
			provider.SetSubscription(billing.ProviderSubscription{ID: "sub_" + victimID, CustomerID: "cus_" + victimID, Status: billing.StatusActive})
			ledger := billing.NewMongoStore(mdb)
			outboxRepo := newOutboxRepo(mdb)
			tx := db.NewTransactor(mdb.Client(), nil)

			if tc.deleteUser {
				deletion := billing.NewDeletion(billing.DeletionDeps{Provider: provider, Ledger: ledger})
				require.NoError(t, deletion.Settle(ctx, victimID))
				require.NoError(t, newCascade(mdb, outboxRepo).Remove(ctx, victimID))
			}
			readsBefore := provider.Reads()

			ev := billing.Event{
				ID:                     billingLateEv,
				Provider:               billing.ProviderStripe,
				ProviderEventID:        "evt_late",
				Kind:                   billing.KindSubscriptionChanged,
				UserID:                 victimID,
				ProviderSubscriptionID: "sub_" + victimID,
				Status:                 billing.EventReceived,
				ReceivedAt:             deleteAt,
				NextAttemptAt:          deleteAt,
				ExpiresAt:              deleteAt.Add(time.Hour),
			}
			require.NoError(t, ledger.InsertEvent(ctx, ev))
			consumer := billing.NewConsumer(billing.ConsumerDeps{Provider: provider, Ledger: ledger, Tx: tx, Outbox: outboxRepo, Now: func() time.Time { return deleteAt }})
			err := consumer.Handle(ctx, sweeper.Job[billing.Event]{ID: ev.ID, Payload: ev}, deleteAt)
			require.NoError(t, err)

			var stored billing.Event
			require.NoError(t, mdb.Collection(billing.EventsCollection).FindOne(ctx, bson.D{{Key: "_id", Value: ev.ID}}).Decode(&stored))
			assert.Equal(t, billing.EventProcessed, stored.Status)
			assert.Equal(t, tc.wantReason, stored.Reason)
			if tc.deleteUser {
				assert.Equal(t, readsBefore, provider.Reads(), "an ignored event must not read the provider")
			}
		})
	}
}
