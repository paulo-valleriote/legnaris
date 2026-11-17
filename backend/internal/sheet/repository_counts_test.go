package sheet_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCountsTest(t *testing.T) (sheet.Repository, *db.Transactor) {
	t.Helper()

	repo, mdb := newRepositoryTestWithDB(t)
	return repo, db.NewTransactor(mdb.Client(), nil)
}

func reserve(ctx context.Context, tx *db.Transactor, repo sheet.Repository, owner string, kind plan.SheetKind, limit int) error {
	return tx.WithTransaction(ctx, func(txCtx context.Context) error {
		return repo.ReserveSlot(txCtx, owner, kind, limit)
	})
}

func TestRepository_ReserveSlot(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		kind      plan.SheetKind
		limit     int
		reserves  int
		wantCount int
		wantErr   error
	}{
		{name: "Happy path: a reservation under the limit is counted", kind: plan.KindCharacter, limit: 1, reserves: 1, wantCount: 1},
		{name: "Happy path: reservations up to the limit all succeed", kind: plan.KindNPC, limit: 2, reserves: 2, wantCount: 2},
		{name: "Edge: a reservation at the limit is refused and not counted", kind: plan.KindCharacter, limit: 1, reserves: 2, wantCount: 1, wantErr: sheet.ErrSheetLimitReached},
		{name: "Edge: a zero allowance refuses the first reservation", kind: plan.KindEnemy, limit: 0, reserves: 1, wantCount: 0, wantErr: sheet.ErrSheetLimitReached},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, tx := newCountsTest(t)
			var lastErr error
			for range tc.reserves {
				lastErr = reserve(ctx, tx, repo, "owner-a", tc.kind, tc.limit)
			}
			if tc.wantErr != nil {
				require.ErrorIs(t, lastErr, tc.wantErr)
			} else {
				require.NoError(t, lastErr)
			}
			got, err := repo.CountOf(ctx, "owner-a", tc.kind)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCount, got)
		})
	}
}

func TestRepository_ReserveSlotConcurrentAtLimit(t *testing.T) {
	ctx := context.Background()
	const limit = 3
	const attempts = 12

	repo, tx := newCountsTest(t)

	var wg sync.WaitGroup
	var mu sync.Mutex
	granted, refused := 0, 0
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := reserve(ctx, tx, repo, "owner-race", plan.KindNPC, limit)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				granted++
			case errors.Is(err, sheet.ErrSheetLimitReached):
				refused++
			default:
				t.Errorf("unexpected reservation error: %v", err)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, limit, granted, "exactly the allowance is granted under a race")
	assert.Equal(t, attempts-limit, refused)
	got, err := repo.CountOf(ctx, "owner-race", plan.KindNPC)
	require.NoError(t, err)
	assert.Equal(t, limit, got)
}

func TestRepository_ReleaseSlot(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		reserves  int
		releases  int
		wantCount int
	}{
		{name: "Happy path: a release gives the slot back", reserves: 1, releases: 1, wantCount: 0},
		{name: "Edge: a release never takes the counter below zero", reserves: 0, releases: 1, wantCount: 0},
		{name: "Edge: a second release after the counter reached zero is a no-op", reserves: 1, releases: 2, wantCount: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, tx := newCountsTest(t)
			for range tc.reserves {
				require.NoError(t, reserve(ctx, tx, repo, "owner-r", plan.KindCharacter, 5))
			}
			for range tc.releases {
				require.NoError(t, tx.WithTransaction(ctx, func(txCtx context.Context) error {
					return repo.ReleaseSlot(txCtx, "owner-r", plan.KindCharacter)
				}))
			}
			got, err := repo.CountOf(ctx, "owner-r", plan.KindCharacter)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCount, got)
		})
	}
}

func TestRepository_CountOfMissingCounterIsZero(t *testing.T) {
	repo := newRepositoryTest(t)

	got, err := repo.CountOf(context.Background(), "owner-nobody", plan.KindNPC)
	require.NoError(t, err)
	assert.Zero(t, got)
}

func TestRepository_ReservationRollsBackWithFailedInsert(t *testing.T) {
	ctx := context.Background()
	repo, tx := newCountsTest(t)

	insertErr := errors.New("insert refused")
	err := tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.ReserveSlot(txCtx, "owner-rb", plan.KindCharacter, 1); err != nil {
			return err
		}
		return insertErr
	})
	require.ErrorIs(t, err, db.ErrTransactionAborted)
	require.ErrorIs(t, err, insertErr)

	got, err := repo.CountOf(ctx, "owner-rb", plan.KindCharacter)
	require.NoError(t, err)
	assert.Zero(t, got, "a failed insert leaves no slot behind")
}
