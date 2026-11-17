package campaign_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
)

func newCounterFixture(t *testing.T) (*db.Transactor, campaign.OwnershipCounter) {
	t.Helper()
	ctx := context.Background()
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	require.NoError(t, db.EnsureIndexes(ctx, mdb))
	return db.NewTransactor(client, nil), campaign.NewOwnershipCounter(mdb)
}

func TestOwnershipCounter_ReserveRefusesAtTheLimit(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		limit     int
		reserves  int
		wantOK    int
		wantCount int
	}{
		{name: "Edge: zero limit refuses every reserve", limit: 0, reserves: 3, wantOK: 0, wantCount: 0},
		{name: "Edge: Free limit of one allows exactly one", limit: 1, reserves: 3, wantOK: 1, wantCount: 1},
		{name: "Happy path: Paid limit of five allows five", limit: 5, reserves: 5, wantOK: 5, wantCount: 5},
		{name: "Edge: reserves under the limit all succeed", limit: 5, reserves: 2, wantOK: 2, wantCount: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, counters := newCounterFixture(t)
			owner := campaigntest.NewID(t)
			refused := 0
			for range tc.reserves {
				err := counters.Reserve(ctx, owner, tc.limit)
				if err != nil {
					require.ErrorIs(t, err, campaign.ErrOwnershipLimitReached)
					refused++
				}
			}
			assert.Equal(t, tc.reserves-tc.wantOK, refused)
			count, err := counters.Count(ctx, owner)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCount, count)
		})
	}
}

func TestOwnershipCounter_ConcurrentReservesNeverExceedTheLimit(t *testing.T) {
	ctx := context.Background()
	const limit = 3
	const attempts = 20
	tx, counters := newCounterFixture(t)
	owner := campaigntest.NewID(t)

	var succeeded, refused, other atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := tx.WithTransaction(ctx, func(txCtx context.Context) error {
				return counters.Reserve(txCtx, owner, limit)
			})
			switch {
			case err == nil:
				succeeded.Add(1)
			case errors.Is(err, campaign.ErrOwnershipLimitReached):
				refused.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(0), other.Load())
	assert.Equal(t, int32(limit), succeeded.Load())
	assert.Equal(t, int32(attempts-limit), refused.Load())
	count, err := counters.Count(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, limit, count)
}

func TestOwnershipCounter_ReleaseFreesASlot(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		releases  int
		wantCount int
	}{
		{name: "Happy path: one release frees one slot", releases: 1, wantCount: 0},
		{name: "Edge: releasing past zero stays at zero", releases: 3, wantCount: 0},
		{name: "Edge: releasing an owner with no counter is a no-op", releases: 0, wantCount: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, counters := newCounterFixture(t)
			owner := campaigntest.NewID(t)
			if tc.releases > 0 {
				require.NoError(t, counters.Reserve(ctx, owner, 1))
			}
			for range tc.releases {
				require.NoError(t, counters.Release(ctx, owner))
			}
			count, err := counters.Count(ctx, owner)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCount, count)
		})
	}
}

func TestOwnershipCounter_ReserveAfterReleaseSucceeds(t *testing.T) {
	ctx := context.Background()
	_, counters := newCounterFixture(t)
	owner := campaigntest.NewID(t)
	require.NoError(t, counters.Reserve(ctx, owner, 1))
	require.ErrorIs(t, counters.Reserve(ctx, owner, 1), campaign.ErrOwnershipLimitReached)
	require.NoError(t, counters.Release(ctx, owner))
	require.NoError(t, counters.Reserve(ctx, owner, 1))
}

func TestOwnershipCounter_OwnersAreIndependent(t *testing.T) {
	ctx := context.Background()
	_, counters := newCounterFixture(t)
	first := campaigntest.NewID(t)
	second := campaigntest.NewID(t)
	require.NoError(t, counters.Reserve(ctx, first, 1))
	require.NoError(t, counters.Reserve(ctx, second, 1))
	require.ErrorIs(t, counters.Reserve(ctx, first, 1), campaign.ErrOwnershipLimitReached)
}
