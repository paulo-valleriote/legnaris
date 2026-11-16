package billing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	inboxUserID   = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"
	inboxSubID    = "sub_inbox_1"
	inboxCustomer = "cus_inbox_1"
)

var inboxNow = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

func newInboxFixture(t *testing.T) (*billing.Inbox, *billingtest.Ledger, *billingtest.Provider) {
	t.Helper()
	ledger := billingtest.NewLedger()
	ledger.SeedSubscription(billing.Subscription{
		ID:                 billingtest.NewEventID(),
		UserID:             inboxUserID,
		Status:             billing.StatusPending,
		ProviderCustomerID: inboxCustomer,
	})
	provider := billingtest.NewProvider()
	return billing.NewInbox(provider, ledger, func() time.Time { return inboxNow }), ledger, provider
}

func notificationPayload(t *testing.T, n billing.Notification) []byte {
	t.Helper()
	body, err := json.Marshal(n)
	require.NoError(t, err)
	return body
}

func findEvent(t *testing.T, ledger *billingtest.Ledger, providerEventID string) billing.Event {
	t.Helper()
	for _, ev := range ledger.Events() {
		if ev.ProviderEventID == providerEventID {
			return ev
		}
	}
	require.FailNow(t, "event not stored", providerEventID)
	return billing.Event{}
}

func TestInbox_Receive(t *testing.T) {
	subChanged := billing.Notification{
		ID:                     "evt_1",
		Kind:                   billing.KindSubscriptionChanged,
		ProviderSubscriptionID: inboxSubID,
		ProviderCustomerID:     inboxCustomer,
	}

	t.Run("Error: invalid signature is INVALID_SIGNATURE and stores nothing", func(t *testing.T) {
		inbox, ledger, _ := newInboxFixture(t)
		err := inbox.Receive(context.Background(), notificationPayload(t, subChanged), "forged")
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidSignature, appErr.Code)
		assert.Empty(t, ledger.Events())
	})

	t.Run("Error: unconfigured provider is BILLING_UNAVAILABLE", func(t *testing.T) {
		inbox := billing.NewInbox(billing.UnconfiguredProvider{}, billingtest.NewLedger(), func() time.Time { return inboxNow })
		err := inbox.Receive(context.Background(), notificationPayload(t, subChanged), billingtest.ValidSignature)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeBillingUnavailable, appErr.Code)
	})

	t.Run("Error: a malformed body with a valid signature is INVALID_REQUEST", func(t *testing.T) {
		inbox, ledger, _ := newInboxFixture(t)
		err := inbox.Receive(context.Background(), []byte("{not json"), billingtest.ValidSignature)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
		assert.Empty(t, ledger.Events())
	})

	t.Run("Happy path: a subscription change for a known customer is stored received for the owner", func(t *testing.T) {
		inbox, ledger, _ := newInboxFixture(t)
		require.NoError(t, inbox.Receive(context.Background(), notificationPayload(t, subChanged), billingtest.ValidSignature))

		ev := findEvent(t, ledger, "evt_1")
		assert.Equal(t, billing.EventReceived, ev.Status)
		assert.Equal(t, inboxUserID, ev.UserID)
		assert.Equal(t, inboxSubID, ev.ProviderSubscriptionID)
		assert.Equal(t, billing.KindSubscriptionChanged, ev.Kind)
		assert.True(t, ev.NextAttemptAt.Equal(inboxNow))
		assert.True(t, ev.ExpiresAt.Equal(inboxNow.Add(90*24*time.Hour)))
		assert.Equal(t, billing.ProviderStripe, ev.Provider)
	})

	t.Run("Edge: a duplicate provider event is a no-op that still succeeds", func(t *testing.T) {
		inbox, ledger, _ := newInboxFixture(t)
		body := notificationPayload(t, subChanged)
		require.NoError(t, inbox.Receive(context.Background(), body, billingtest.ValidSignature))
		require.NoError(t, inbox.Receive(context.Background(), body, billingtest.ValidSignature))
		assert.Len(t, ledger.Events(), 1)
	})

	t.Run("Edge: an ignored kind is stored processed with reason ignored", func(t *testing.T) {
		inbox, ledger, _ := newInboxFixture(t)
		n := billing.Notification{ID: "evt_ignored", Kind: billing.KindIgnored}
		require.NoError(t, inbox.Receive(context.Background(), notificationPayload(t, n), billingtest.ValidSignature))

		ev := findEvent(t, ledger, "evt_ignored")
		assert.Equal(t, billing.EventProcessed, ev.Status)
		assert.Equal(t, billing.ReasonIgnored, ev.Reason)
		assert.Empty(t, ev.UserID)
	})

	t.Run("Edge: a notification for an unknown account is stored processed with reason ignored", func(t *testing.T) {
		inbox, ledger, _ := newInboxFixture(t)
		n := billing.Notification{ID: "evt_orphan", Kind: billing.KindSubscriptionChanged, ProviderSubscriptionID: "sub_unknown", ProviderCustomerID: "cus_unknown"}
		require.NoError(t, inbox.Receive(context.Background(), notificationPayload(t, n), billingtest.ValidSignature))

		ev := findEvent(t, ledger, "evt_orphan")
		assert.Equal(t, billing.EventProcessed, ev.Status)
		assert.Equal(t, billing.ReasonIgnored, ev.Reason)
	})

	t.Run("Happy path: an explicit user id in the notification wins over lookups", func(t *testing.T) {
		inbox, ledger, _ := newInboxFixture(t)
		n := billing.Notification{ID: "evt_meta", Kind: billing.KindSubscriptionChanged, UserID: inboxUserID, ProviderSubscriptionID: "sub_new"}
		require.NoError(t, inbox.Receive(context.Background(), notificationPayload(t, n), billingtest.ValidSignature))
		assert.Equal(t, billing.EventReceived, findEvent(t, ledger, "evt_meta").Status)
	})
}
