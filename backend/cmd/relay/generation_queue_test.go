package main

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/accountdeletion"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/email/outbox"
	outboxmocks "legnaris/backend/internal/email/outbox/mocks"
	eventsoutbox "legnaris/backend/internal/events/outbox"
	eventsoutboxmocks "legnaris/backend/internal/events/outbox/mocks"
	"legnaris/backend/internal/generation"
	generationmocks "legnaris/backend/internal/generation/mocks"
	usermocks "legnaris/backend/internal/user/mocks"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeGenerationBroker struct {
	closed bool
}

func (b *fakeGenerationBroker) Publish(context.Context, generation.Message) error { return nil }
func (b *fakeGenerationBroker) Close() error                                      { b.closed = true; return nil }

func idleGenerationDeps(t *testing.T) generation.RepublishDeps {
	t.Helper()
	store := generationmocks.NewMockPublishStore(t)
	store.EXPECT().ClaimUnpublished(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	store.EXPECT().OldestQueued(mock.Anything).Return(time.Time{}, false, nil).Maybe()
	return generation.RepublishDeps{
		Store:     store,
		Publisher: &fakeGenerationBroker{},
		Quota:     generationmocks.NewMockQuota(t),
		Tx:        generationmocks.NewMockTransactor(t),
		Now:       time.Now,
	}
}

func idleDeletionDeps(t *testing.T) accountdeletion.Deps {
	t.Helper()
	store := usermocks.NewMockDeletionStore(t)
	store.EXPECT().ClaimDeleting(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	store.EXPECT().OldestDeleting(mock.Anything).Return(time.Time{}, false, nil).Maybe()
	return accountdeletion.Deps{Store: store}
}

func idleReapDeps(t *testing.T) generation.ReapDeps {
	t.Helper()
	store := generationmocks.NewMockLeaseStore(t)
	store.EXPECT().ClaimExpired(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	store.EXPECT().OldestRunning(mock.Anything).Return(time.Time{}, false, nil).Maybe()
	return generation.ReapDeps{
		Store: store,
		Quota: generationmocks.NewMockQuota(t),
		Tx:    generationmocks.NewMockTransactor(t),
	}
}

func TestSweepQueues_RegistersGenerationRepublish(t *testing.T) {
	tests := []struct {
		name      string
		batch     int
		wantErr   bool
		wantNames []string
	}{
		{name: "Happy path: the event outbox, the generation republish, the lease reaper and the deleting accounts and the billing consumer are registered", batch: 25, wantNames: []string{"event_outbox", "generation", "generation_lease", "users_deleting", "billing_events", "billing_refunds"}},
		{name: "Error: a zero batch refuses the generation queue", batch: 0, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testEnv()
			cfg.Variables.EmailRelayBatch = tc.batch
			queues, err := sweepQueues(cfg, eventsoutboxmocks.NewMockRepository(t), &fakeEventBroker{}, idleGenerationDeps(t), idleReapDeps(t), idleDeletionDeps(t), idleBillingDeps())
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var names []string
			for _, q := range queues {
				names = append(names, q.Name)
			}
			assert.Equal(t, tc.wantNames, names)
			assert.Equal(t, 30*time.Second, queues[0].Interval)
			assert.Equal(t, generation.RepublishInterval, queues[1].Interval)
			assert.Equal(t, generation.ReapInterval, queues[2].Interval)
			assert.Equal(t, 10*time.Second, queues[2].Interval)
		})
	}
}

func TestRun_ClosesGenerationBroker(t *testing.T) {
	cfg := testEnv()
	generationBroker := &fakeGenerationBroker{}

	deps := relayDepsForCancelledRun(t, generationBroker)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, run(ctx, cfg, deps))
	assert.True(t, generationBroker.closed, "the generation broker must be closed on shutdown")
}

func relayDepsForCancelledRun(t *testing.T, generationBroker *fakeGenerationBroker) dependencies {
	t.Helper()
	deps := idleRelayDeps(t)
	deps.dialGenerationBroker = func(env.Env) (generationBrokerClient, error) { return generationBroker, nil }
	deps.newGenerationRepublish = func(_ env.Env, _ databaseHandle, publisher generation.Publisher) (generation.RepublishDeps, error) {
		deps := idleGenerationDeps(t)
		deps.Publisher = publisher
		return deps, nil
	}
	deps.newGenerationReap = func(env.Env, databaseHandle) (generation.ReapDeps, error) {
		return idleReapDeps(t), nil
	}
	return deps
}

func idleRelayDeps(t *testing.T) dependencies {
	t.Helper()
	repository := outboxmocks.NewMockRepository(t)
	repository.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	repository.EXPECT().Backlog(mock.Anything).Return(0, nil).Maybe()

	eventRepository := eventsoutboxmocks.NewMockRepository(t)
	eventRepository.EXPECT().OldestPending(mock.Anything).Return(time.Time{}, false, nil).Maybe()
	eventRepository.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()

	return dependencies{
		openDatabase:  func(env.Env) (databaseHandle, error) { return &fakeDatabase{}, nil },
		ensureIndexes: func(context.Context, env.Env, databaseHandle) error { return nil },
		newRepository: func(env.Env, databaseHandle, logger.StructuredLogger) (outbox.Repository, error) {
			return repository, nil
		},
		dialBroker:      func(env.Env) (brokerClient, error) { return &fakeBroker{}, nil },
		dialEventBroker: func(env.Env) (eventBrokerClient, error) { return &fakeEventBroker{}, nil },
		newEventRepository: func(env.Env, databaseHandle, logger.StructuredLogger) (eventsoutbox.Repository, error) {
			return eventRepository, nil
		},
		serveMetrics: noopServeMetrics,
		newAccountDeletion: func(env.Env, databaseHandle, eventsoutbox.Repository) (accountdeletion.Deps, error) {
			return idleDeletionDeps(t), nil
		},
		newBilling: func(env.Env, databaseHandle, eventsoutbox.Repository) (billing.ConsumerDeps, error) {
			return idleBillingDeps(), nil
		},
	}
}

func idleBillingDeps() billing.ConsumerDeps {
	ledger := billingtest.NewLedger()
	return billing.ConsumerDeps{
		Provider: billingtest.NewProvider(),
		Ledger:   ledger,
		Tx:       billingtest.NewTransactor(ledger, nil),
		Outbox:   ledger.Outbox(),
		Now:      time.Now,
	}
}
