package template

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateUpdateKeys covers the update contract the sheet service enforces
// before Recalculate (LGN-66): a writable leaf key passes, and every other key
// comes back as the typed error the service boundary turns into an apperr code.
func TestValidateUpdateKeys(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn66")

	tests := []struct {
		name            string
		updates         map[string]any
		wantUnknown     string
		wantNotEditable string
		wantMsg         string
	}{
		{
			name:    "Happy path: editable leaf key passes",
			updates: map[string]any{"stats.str": 16.0},
		},
		{
			name:    "Edge: no updates passes",
			updates: nil,
		},
		{
			name:        "Error: key outside the DAG",
			updates:     map[string]any{"stats.unknown": 1.0},
			wantUnknown: "stats.unknown",
			wantMsg:     "unknown field 'stats.unknown' in update",
		},
		{
			// A group prefix is a valid key for Recalculate but never for an
			// update payload: applying it would replace the whole group with
			// the scalar and drop its siblings.
			name:        "Error: group prefix is not a writable field",
			updates:     map[string]any{"stats": 12.0},
			wantUnknown: "stats",
			wantMsg:     "unknown field 'stats' in update",
		},
		{
			name:            "Error: derived field is not writable",
			updates:         map[string]any{"derived.bonus": 5.0},
			wantNotEditable: "derived.bonus",
			wantMsg:         "field is not editable: derived.bonus",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := dag.ValidateUpdateKeys(tc.updates)

			if tc.wantUnknown == "" && tc.wantNotEditable == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Equal(t, tc.wantMsg, err.Error())

			if tc.wantUnknown != "" {
				var unknown *UnknownFieldError
				require.ErrorAs(t, err, &unknown)
				assert.Equal(t, tc.wantUnknown, unknown.Key)
				return
			}
			var notEditable *NotEditableError
			require.ErrorAs(t, err, &notEditable)
			assert.Equal(t, tc.wantNotEditable, notEditable.Key)
		})
	}
}

// TestValidateUpdateKeys_NilDAG mirrors Recalculate's guard: a missing graph
// is an internal invariant violation, not a caller mistake to panic on.
func TestValidateUpdateKeys_NilDAG(t *testing.T) {
	var dag *DAG
	require.ErrorIs(t, dag.ValidateUpdateKeys(map[string]any{"stats.str": 16.0}), ErrNilDAG)
}

// TestValidateUpdateKeys_ConcurrentWithRecalculate proves the new entry point
// reads under its own single read lock: running it against Recalculate in
// parallel must neither race nor deadlock against a queued writer (LGN-54
// discipline, exercised under the race detector by make test-race).
func TestValidateUpdateKeys_ConcurrentWithRecalculate(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn66")

	original := map[string]any{"stats": map[string]any{"str": 10.0}}
	updates := map[string]any{"stats.str": 16.0}

	const iterations = 50
	errs := make(chan error, iterations*2)
	var wg sync.WaitGroup
	for range iterations {
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs <- dag.ValidateUpdateKeys(updates)
		}()
		go func() {
			defer wg.Done()
			_, err := dag.Recalculate(original, updates)
			errs <- err
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("validation deadlocked while Recalculate ran concurrently")
	}

	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
