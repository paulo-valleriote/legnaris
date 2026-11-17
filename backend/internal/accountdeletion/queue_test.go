package accountdeletion_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"legnaris/backend/internal/accountdeletion"
	accountdeletionmocks "legnaris/backend/internal/accountdeletion/mocks"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/internal/user"
	usermocks "legnaris/backend/internal/user/mocks"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/logger"
)

type stubTransactor struct {
	err   error
	calls int
}

func (s *stubTransactor) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	s.calls++
	if s.err != nil {
		return s.err
	}
	return fn(ctx)
}

func newQueueRunner(t *testing.T, deps accountdeletion.Deps, attemptCap int) *sweeper.Sweeper {
	t.Helper()
	queue, err := accountdeletion.NewQueue(deps, attemptCap, 10)
	require.NoError(t, err)
	runner, err := sweeper.New([]sweeper.Queue{queue}, "test-claimer", metrics.New(), logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	return runner
}

func TestNewQueue(t *testing.T) {
	tests := []struct {
		name             string
		mutate           func(*accountdeletion.Deps)
		attemptCap       int
		batch            int
		wantErr          bool
		wantInvalidQueue bool
	}{
		{name: "Happy path: the queue registers under users_deleting every minute", attemptCap: 5, batch: 10},
		{name: "Edge: a zero batch is refused", attemptCap: 5, batch: 0, wantErr: true, wantInvalidQueue: true},
		{name: "Edge: a zero attempt cap is refused", attemptCap: 0, batch: 10, wantErr: true, wantInvalidQueue: true},
		{name: "Error: a nil campaign settler is refused", mutate: func(d *accountdeletion.Deps) { d.Campaigns = nil }, attemptCap: 5, batch: 10, wantErr: true},
		{name: "Error: a nil billing settler is refused", mutate: func(d *accountdeletion.Deps) { d.Billing = nil }, attemptCap: 5, batch: 10, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := accountdeletion.Deps{
				Store:     usermocks.NewMockDeletionStore(t),
				Tx:        &stubTransactor{},
				Billing:   accountdeletionmocks.NewMockBillingSettler(t),
				Campaigns: noCampaigns{},
			}
			if tc.mutate != nil {
				tc.mutate(&deps)
			}
			queue, err := accountdeletion.NewQueue(deps, tc.attemptCap, tc.batch)
			if tc.wantInvalidQueue {
				require.ErrorIs(t, err, sweeper.ErrInvalidQueue)
				return
			}
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "users_deleting", queue.Name)
			assert.Equal(t, time.Minute, queue.Interval)
		})
	}
}

func TestQueue_Failures(t *testing.T) {
	const victim = "01900000-0000-7000-8000-000000000001"
	failure := errors.New("cascade refused")
	settleFailure := errors.New("provider unavailable")

	tests := []struct {
		name           string
		attemptCap     int
		claim          []user.User
		settleErr      error
		wantSettle     int
		campaignErr    error
		wantCampaign   int
		wantTx         int
		wantReschedule bool
		wantDead       bool
	}{
		{
			name:           "Error: a failed attempt reschedules the account under the cap",
			attemptCap:     5,
			claim:          []user.User{{ID: victim, DeletionAttempts: 1}},
			wantSettle:     1,
			wantCampaign:   1,
			wantTx:         1,
			wantReschedule: true,
		},
		{
			name:         "Error: the attempt that reaches the cap marks the account dead",
			attemptCap:   3,
			claim:        []user.User{{ID: victim, DeletionAttempts: 2}},
			wantSettle:   1,
			wantCampaign: 1,
			wantTx:       1,
			wantDead:     true,
		},
		{
			name:           "Error: a failed campaign settlement reschedules before the cascade starts",
			attemptCap:     5,
			claim:          []user.User{{ID: victim, DeletionAttempts: 1}},
			campaignErr:    failure,
			wantSettle:     1,
			wantCampaign:   1,
			wantTx:         0,
			wantReschedule: true,
		},
		{
			name:           "Error: a provider failure while settling billing reschedules before the cascade starts",
			attemptCap:     5,
			claim:          []user.User{{ID: victim, DeletionAttempts: 1}},
			settleErr:      settleFailure,
			wantSettle:     1,
			wantTx:         0,
			wantReschedule: true,
		},
		{
			name:       "Edge: an empty queue starts no transaction and settles nothing",
			attemptCap: 5,
			claim:      nil,
			wantTx:     0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := usermocks.NewMockDeletionStore(t)
			store.EXPECT().OldestDeleting(mock.Anything).Return(time.Time{}, false, nil).Maybe()
			store.EXPECT().ClaimDeleting(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(tc.claim, nil).Once()
			if tc.wantReschedule {
				store.EXPECT().RescheduleDeleting(mock.Anything, victim, mock.Anything, mock.MatchedBy(func(reason string) bool {
					return reason != ""
				})).Return(nil).Once()
			}
			if tc.wantDead {
				store.EXPECT().MarkDeletionDead(mock.Anything, victim, mock.Anything).Return(nil).Once()
			}

			settler := accountdeletionmocks.NewMockBillingSettler(t)
			if tc.wantSettle > 0 {
				settler.EXPECT().Settle(mock.Anything, victim).Return(tc.settleErr).Times(tc.wantSettle)
			}
			campaigns := accountdeletionmocks.NewMockCampaignSettler(t)
			if tc.wantCampaign > 0 {
				campaigns.EXPECT().SettleAccount(mock.Anything, victim).Return(tc.campaignErr).Times(tc.wantCampaign)
			}

			tx := &stubTransactor{err: failure}
			runner := newQueueRunner(t, accountdeletion.Deps{Store: store, Tx: tx, Billing: settler, Campaigns: campaigns}, tc.attemptCap)
			require.NoError(t, runner.RunOnce(context.Background(), time.Now()))
			assert.Equal(t, tc.wantTx, tx.calls)
		})
	}
}

type noCampaigns struct{}

func (noCampaigns) SettleAccount(context.Context, string) error { return nil }

func TestQueue_RemovesDeletingAccount(t *testing.T) {
	mdb := dbtest.Database(t)
	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	seedAccount(t, mdb, victimID, victimEmail)
	_, err := mdb.Collection("users").UpdateOne(context.Background(),
		bson.D{{Key: "_id", Value: victimID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: deleteAt}}}})
	require.NoError(t, err)

	deps := accountdeletion.Deps{
		Store:     user.NewDeletionStore(mdb, logger.NewLogger(slog.LevelError)),
		Cascade:   newCascade(mdb, outbox.NewRepository(mdb, logger.NewLogger(slog.LevelError))),
		Tx:        db.NewTransactor(mdb.Client(), nil),
		Billing:   billing.NewDeletion(billing.DeletionDeps{Provider: billingtest.NewProvider(), Ledger: billing.NewMongoStore(mdb)}),
		Campaigns: noCampaigns{},
	}
	runner := newQueueRunner(t, deps, 5)
	require.NoError(t, runner.RunOnce(context.Background(), time.Now()))

	assert.Zero(t, countOwned(t, mdb, "users", "_id", victimID))
	assert.Zero(t, countOwned(t, mdb, "sheets", "owner_id", victimID))
	assert.Len(t, userDeletedEntries(t, mdb, victimID), 1)
}
