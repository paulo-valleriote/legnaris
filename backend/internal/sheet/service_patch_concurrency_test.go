package sheet_test

import (
	"context"
	"errors"
	"sync"
	"testing"

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
	register("Info", "sheet patched", 10)
	register("Warn", "sheet patch version conflict", 9)
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

	t.Run("Happy path: concurrent PATCHes on different fields both survive without a lost update", func(t *testing.T) {
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
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])

		store.mu.Lock()
		defer store.mu.Unlock()
		assert.Equal(t, int64(2), store.version, "two successful writes must bump the version twice")
		assert.Equal(t, map[string]any{"characterName": "Bryn"}, store.data["identity"],
			"the name change must survive the concurrent dexterity write")
		assert.Equal(t, map[string]any{"dexterity": 18.0}, store.data["stats"],
			"the dexterity change must survive the concurrent name write")
		assert.Equal(t, map[string]any{"dexterityMod": 4.0}, store.data["modifiers"],
			"the derived modifier must match the stored dexterity")
	})

	t.Run("Edge: a PATCH that loses the CAS on every attempt returns CONFLICT after exactly three writes", func(t *testing.T) {
		store := newPatchStore(patchSheet(0, "Aria", 14))
		store.beforeWrite = func(s *patchStore, _ int) { s.version++ }

		service := newPatchService(t, newPatchStoreRepo(t, store), newQuietPatchLogger(t), dag)
		cascade, err := service.PartialUpdate(ctx, dexterityPatch(18))

		require.Error(t, err)
		assert.Nil(t, cascade)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeConflict, appErr.Code)

		store.mu.Lock()
		defer store.mu.Unlock()
		assert.Equal(t, 3, store.writes, "the service must stop after maxPartialUpdateAttempts writes")
		assert.Equal(t, 3, store.fetches, "each attempt must re-read the sheet exactly once")
	})

	t.Run("Error: a conflict followed by deletion surfaces NOT FOUND and never CONFLICT", func(t *testing.T) {
		store := newPatchStore(patchSheet(0, "Aria", 14))
		store.beforeWrite = func(s *patchStore, attempt int) {
			if attempt == 1 {
				s.version++
				s.deleted = true
			}
		}

		service := newPatchService(t, newPatchStoreRepo(t, store), newQuietPatchLogger(t), dag)
		cascade, err := service.PartialUpdate(ctx, dexterityPatch(18))

		require.Error(t, err)
		assert.Nil(t, cascade)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		assert.NotEqual(t, apperr.CodeConflict, appErr.Code)
		assert.Equal(t, 404, appErr.HttpStatus())

		store.mu.Lock()
		defer store.mu.Unlock()
		assert.Equal(t, 1, store.writes, "no write may be attempted once the sheet is gone")
	})
}
