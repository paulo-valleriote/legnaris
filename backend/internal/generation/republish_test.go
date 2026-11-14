package generation_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/generation/mocks"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// noopObserver satisfies sweeper.Observer without recording anything.
type noopObserver struct{}

func (noopObserver) OldestPending(string, time.Duration) {}
func (noopObserver) Failed(string)                       {}

// republishFixture wires the republish queue to mocks.
type republishFixture struct {
	store     *mocks.MockPublishStore
	publisher *mocks.MockPublisher
	quota     *mocks.MockQuota
	tx        *mocks.MockTransactor
}

func newRepublishFixture(t *testing.T) republishFixture {
	t.Helper()
	return republishFixture{
		store:     mocks.NewMockPublishStore(t),
		publisher: mocks.NewMockPublisher(t),
		quota:     mocks.NewMockQuota(t),
		tx:        mocks.NewMockTransactor(t),
	}
}

// sweepOnce builds the queue over the fixture and runs one sweeper pass at now.
func (f republishFixture) sweepOnce(t *testing.T, attemptCap int, now time.Time) error {
	t.Helper()
	queue, err := generation.NewRepublishQueue(generation.RepublishDeps{
		Store:     f.store,
		Publisher: f.publisher,
		Quota:     f.quota,
		Tx:        f.tx,
		Now:       time.Now,
	}, attemptCap, 25)
	require.NoError(t, err)
	sw, err := sweeper.New([]sweeper.Queue{queue}, "relay-test", noopObserver{}, logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	return sw.RunOnce(context.Background(), now)
}

// runInlineTx makes the transaction mock run its callback in place.
func (f republishFixture) runInlineTx() {
	f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Maybe()
}

func TestRepublishQueue_Definition(t *testing.T) {
	tests := []struct {
		name         string
		attemptCap   int
		batch        int
		wantErr      bool
		wantName     string
		wantInterval time.Duration
	}{
		{name: "Happy path: the queue is named generation and runs every 10 s", attemptCap: 5, batch: 25, wantName: "generation", wantInterval: 10 * time.Second},
		{name: "Error: a zero attempt cap is refused", attemptCap: 0, batch: 25, wantErr: true},
		{name: "Error: a zero batch is refused", attemptCap: 5, batch: 0, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepublishFixture(t)
			queue, err := generation.NewRepublishQueue(generation.RepublishDeps{
				Store: f.store, Publisher: f.publisher, Quota: f.quota, Tx: f.tx,
			}, tc.attemptCap, tc.batch)
			if tc.wantErr {
				require.ErrorIs(t, err, sweeper.ErrInvalidQueue)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantName, queue.Name)
			assert.Equal(t, tc.wantInterval, queue.Interval)
		})
	}
}

// TestRepublishQueue_Sweep drives one pass over one due document and pins what the
// sweeper does with the publish outcome.
func TestRepublishQueue_Sweep(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"

	tests := []struct {
		name        string
		attempts    int
		attemptCap  int
		publishErr  error
		wantPublish bool
		wantResched bool
		wantDead    bool
		wantRefund  string
		refundDay   string
		quotaKind   string
	}{
		{name: "Happy path: a routed job is left to the worker", attemptCap: 5, wantPublish: true},
		{name: "Edge: a failed publish is rescheduled while attempts remain", attempts: 0, attemptCap: 5, publishErr: errors.New("broker down"), wantPublish: true, wantResched: true},
		{name: "Edge: the last attempt gives up and refunds the trial", attempts: 4, attemptCap: 5, publishErr: errors.New("broker down"), wantPublish: true, wantDead: true, wantRefund: "trial", quotaKind: generation.ReservationTrial},
		{name: "Edge: the last attempt gives up and refunds the daily day", attempts: 4, attemptCap: 5, publishErr: errors.New("broker down"), wantPublish: true, wantDead: true, wantRefund: "daily", quotaKind: generation.ReservationDaily, refundDay: "2026-11-14"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepublishFixture(t)
			doc := generation.Document{
				ID: jobID, UserID: userID, Kind: "character", Status: generation.StatusQueued,
				PublishAttempts: tc.attempts,
				Quota:           generation.Reservation{Kind: tc.quotaKind, DayUTC: tc.refundDay},
			}
			f.store.EXPECT().ClaimUnpublished(mock.Anything, "relay-test", now, sweeper.Lease, 25).
				Return([]generation.Document{doc}, nil).Once()
			f.store.EXPECT().OldestQueued(mock.Anything).Return(now.Add(-time.Minute), true, nil).Maybe()
			if tc.wantPublish {
				f.publisher.EXPECT().Publish(mock.Anything, generation.Message{ID: jobID, Kind: "character"}).Return(tc.publishErr).Once()
			}
			if tc.wantResched {
				f.store.EXPECT().ReschedulePublish(mock.Anything, jobID, now).Return(nil).Once()
			}
			if tc.wantDead {
				f.runInlineTx()
				f.store.EXPECT().FailQueued(mock.Anything, jobID, generation.ErrorCodePublishFailed).Return(doc, true, nil).Once()
				switch tc.wantRefund {
				case "trial":
					f.quota.EXPECT().RefundTrial(mock.Anything, userID).Return(nil).Once()
				case "daily":
					f.quota.EXPECT().RefundDaily(mock.Anything, userID, tc.refundDay).Return(nil).Once()
				}
			}

			err := f.sweepOnce(t, tc.attemptCap, now)
			require.NoError(t, err)
		})
	}
}

// TestRepublishQueue_GiveUpWithoutQueuedDocumentRefundsNothing pins the race where
// the worker moved the job first: the sweeper must not refund a job it did not fail.
func TestRepublishQueue_GiveUpWithoutQueuedDocumentRefundsNothing(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"

	f := newRepublishFixture(t)
	doc := generation.Document{ID: jobID, UserID: userID, Status: generation.StatusQueued, PublishAttempts: 4,
		Quota: generation.Reservation{Kind: generation.ReservationTrial}}
	f.store.EXPECT().ClaimUnpublished(mock.Anything, mock.Anything, now, sweeper.Lease, 25).Return([]generation.Document{doc}, nil).Once()
	f.store.EXPECT().OldestQueued(mock.Anything).Return(time.Time{}, false, nil).Maybe()
	f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("broker down")).Once()
	f.runInlineTx()
	f.store.EXPECT().FailQueued(mock.Anything, jobID, generation.ErrorCodePublishFailed).Return(generation.Document{}, false, nil).Once()

	require.NoError(t, f.sweepOnce(t, 5, now))
}

// TestRepublishQueue_RefundFailureAbortsGiveUp pins the rule that a failed refund
// keeps the job queued: the transaction rolls back and the job is retried.
func TestRepublishQueue_RefundFailureAbortsGiveUp(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"

	f := newRepublishFixture(t)
	doc := generation.Document{ID: jobID, UserID: userID, Status: generation.StatusQueued, PublishAttempts: 4,
		Quota: generation.Reservation{Kind: generation.ReservationTrial}}
	f.store.EXPECT().ClaimUnpublished(mock.Anything, mock.Anything, now, sweeper.Lease, 25).Return([]generation.Document{doc}, nil).Once()
	f.store.EXPECT().OldestQueued(mock.Anything).Return(time.Time{}, false, nil).Maybe()
	f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("broker down")).Once()
	f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).Return(errors.New("refund failed")).Once()

	require.NoError(t, f.sweepOnce(t, 5, now))
}

// TestRepublishStore_Refund covers a give-up whose reservation kind is unknown: the
// transaction fails and no refund is guessed at, so the quota is never mis-credited.
func TestRepublishStore_Refund(t *testing.T) {
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"
	tests := []struct {
		name string
		kind string
	}{
		{name: "Error: an unknown reservation kind gives up without refunding", kind: "unknown"},
		{name: "Edge: a job without a reservation gives up without refunding", kind: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepublishFixture(t)
			doc := generation.Document{ID: jobID, UserID: userID, Quota: generation.Reservation{Kind: tc.kind}}
			f.store.EXPECT().ClaimUnpublished(mock.Anything, mock.Anything, mock.Anything, mock.Anything, 25).
				Return([]generation.Document{doc}, nil).Once()
			f.store.EXPECT().OldestQueued(mock.Anything).Return(time.Time{}, false, nil).Maybe()
			f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("down")).Once()
			f.runInlineTx()
			f.store.EXPECT().FailQueued(mock.Anything, jobID, generation.ErrorCodePublishFailed).Return(doc, true, nil).Once()
			f.store.EXPECT().ReschedulePublish(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			require.NoError(t, f.sweepOnce(t, 1, time.Now()), "the sweeper logs a failed give-up and keeps running")
		})
	}
}
