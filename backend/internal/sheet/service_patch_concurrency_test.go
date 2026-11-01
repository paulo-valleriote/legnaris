package sheet_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/shared/apperr"
	logmocks "legnaris/backend/shared/logger/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// patchStore is an in-memory stand-in for the stored sheet. All fields are
// guarded by mu, so one store can be shared by concurrent PATCH calls.
type patchStore struct {
	mu      sync.Mutex
	version int64
	data    map[string]any
	deleted bool
	fetches int
	writes  int

	// beforeWrite runs under mu before each CAS check. It simulates a competing
	// writer (bumping the version) or a deletion landing before the write.
	beforeWrite func(s *patchStore, attempt int)
	// fetchGate runs after the snapshot is taken and mu is released, so a test
	// can hold several readers at the same stale version.
	fetchGate func(fetchNo int)
}

func newPatchStore(sheetState *sheet.Sheet) *patchStore {
	return &patchStore{version: sheetState.Version, data: deepCopyPatchData(sheetState.Data)}
}

func (s *patchStore) fetch() (*sheet.Sheet, error) {
	s.mu.Lock()
	s.fetches++
	fetchNo := s.fetches
	if s.deleted {
		s.mu.Unlock()
		return nil, sheet.ErrSheetNotFound
	}
	snapshot := &sheet.Sheet{
		ID:         patchSheetID,
		TemplateID: patchTemplateID,
		OwnerID:    ownerUserID,
		Version:    s.version,
		Data:       deepCopyPatchData(s.data),
	}
	s.mu.Unlock()

	if s.fetchGate != nil {
		s.fetchGate(fetchNo)
	}
	return snapshot, nil
}

func (s *patchStore) compareAndSwap(expected int64, updates map[string]any) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writes++
	if s.beforeWrite != nil {
		s.beforeWrite(s, s.writes)
	}
	if s.deleted {
		return 0, sheet.ErrSheetNotFound
	}
	if expected != s.version {
		return 0, sheet.ErrVersionConflict
	}
	data, ok := updates["data"].(map[string]any)
	if !ok {
		return 0, errors.New("patchStore: update without data map")
	}
	s.data = deepCopyPatchData(data)
	s.version++
	return s.version, nil
}

// deepCopyPatchData copies nested maps so the store and the snapshots handed to
// the service never alias each other.
func deepCopyPatchData(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		if nested, ok := value.(map[string]any); ok {
			dst[key] = deepCopyPatchData(nested)
			continue
		}
		dst[key] = value
	}
	return dst
}

// newPatchStoreRepo wires a mock repository to the store. Expectations have no
// call limits; tests assert call counts through the store's counters.
func newPatchStoreRepo(t *testing.T, store *patchStore) *mocks.MockRepository {
	t.Helper()
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(mock.Anything, patchSheetID).RunAndReturn(
		func(_ context.Context, _ string) (*sheet.Sheet, error) { return store.fetch() },
	)
	repo.EXPECT().PartialUpdate(mock.Anything, patchSheetID, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string, expectedVersion int64, updates map[string]any) (int64, error) {
			return store.compareAndSwap(expectedVersion, updates)
		},
	)
	return repo
}

// newQuietPatchLogger accepts the two PATCH records with their attribute counts
// but does not record them. The strict mock's Run callbacks are not guarded by
// testify, so the recording logger from newPatchLogger cannot be shared by
// concurrent PATCH calls.
func newQuietPatchLogger(t *testing.T) *logmocks.MockStructuredLogger {
	t.Helper()
	log := logmocks.NewMockStructuredLogger(t)
	register := func(level, msg string, attrCount int) {
		args := []any{mock.Anything, msg}
		for range attrCount {
			args = append(args, mock.Anything)
		}
		log.On(level, args...).Maybe()
	}
	register("Info", "sheet patched", 9)
	register("Warn", "sheet patch version conflict", 8)
	return log
}

// runCompetingPatches runs one PATCH per command in its own goroutine and
// returns each error in command order.
func runCompetingPatches(service sheet.Service, cmds ...sheet.PartialUpdateSheetCommand) []error {
	errs := make([]error, len(cmds))
	var wg sync.WaitGroup
	for i, cmd := range cmds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = service.PartialUpdate(context.Background(), cmd)
		}()
	}
	wg.Wait()
	return errs
}

func TestService_PartialUpdate_Concurrency(t *testing.T) {
	ctx := context.Background()
	dag := patchDAG(t)

	t.Run("Edge: concurrent PATCHes on different fields never lose an update; the loser gets 409 and its retry succeeds", func(t *testing.T) {
		store := newPatchStore(patchSheet(0, "Aria", 10))

		var arrived sync.WaitGroup
		arrived.Add(2)
		released := make(chan struct{})
		go func() {
			arrived.Wait()
			close(released)
		}()
		store.fetchGate = func(fetchNo int) {
			if fetchNo <= 2 {
				arrived.Done()
				<-released
			}
		}

		service := newPatchService(t, newPatchStoreRepo(t, store), newQuietPatchLogger(t), dag)
		nameCmd := sheet.PartialUpdateSheetCommand{
			SheetID:   patchSheetID,
			OwnerID:   ownerUserID,
			RequestID: "req-name",
			Data:      map[string]any{"identity": map[string]any{"characterName": "Bryn"}},
		}

		errs := runCompetingPatches(service, nameCmd, dexterityPatch(18))
		failed := 0
		for _, err := range errs {
			if err == nil {
				continue
			}
			failed++
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeConflict, appErr.Code)
			assert.Equal(t, time.Second, appErr.RetryAfter)
		}
		require.Equal(t, 1, failed, "exactly one of the two racing PATCHes must lose the CAS")

		store.mu.Lock()
		assert.Equal(t, int64(1), store.version, "only the winner may bump the version")
		assert.Equal(t, 2, store.writes, "the loser must not retry the write")
		assert.Equal(t, 2, store.fetches, "the loser must not re-read the sheet")
		store.mu.Unlock()

		_, err := service.PartialUpdate(ctx, nameCmd)
		require.NoError(t, err)
		_, err = service.PartialUpdate(ctx, dexterityPatch(18))
		require.NoError(t, err)

		store.mu.Lock()
		defer store.mu.Unlock()
		assert.Equal(t, map[string]any{"characterName": "Bryn"}, store.data["identity"])
		assert.Equal(t, map[string]any{"dexterity": 18.0}, store.data["stats"])
		assert.Equal(t, map[string]any{"dexterityMod": 4.0}, store.data["modifiers"])
	})

	t.Run("Error: a PATCH that loses the CAS returns CONFLICT after exactly one read and one write", func(t *testing.T) {
		store := newPatchStore(patchSheet(0, "Aria", 14))
		store.beforeWrite = func(s *patchStore, _ int) { s.version++ }

		service := newPatchService(t, newPatchStoreRepo(t, store), newQuietPatchLogger(t), dag)
		cascade, err := service.PartialUpdate(ctx, dexterityPatch(18))

		require.Error(t, err)
		assert.Nil(t, cascade)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeConflict, appErr.Code)
		assert.Equal(t, time.Second, appErr.RetryAfter)

		store.mu.Lock()
		defer store.mu.Unlock()
		assert.Equal(t, 1, store.writes, "a lost CAS must not be retried inside the service")
		assert.Equal(t, 1, store.fetches, "a lost CAS must not trigger another read")
	})
}
