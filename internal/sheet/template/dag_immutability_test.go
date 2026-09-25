package template

import (
	"legnaris/internal/engine"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildImmutabilityTestDAG returns a small DAG with one editable base field and
// one derived field. It backs the LGN-54 locking and immutability tests.
func buildImmutabilityTestDAG(t *testing.T, id string) *DAG {
	t.Helper()
	dag, err := BuildDAG(id, FieldGroup{
		"stats": {Children: FieldGroup{
			"str": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"derived": {Children: FieldGroup{
			"bonus": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "stats_str * 2",
				DependsOn:  [][]string{{"stats", "str"}},
			}},
		}},
	}, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

// requireWriterQueued waits until the writer goroutine is parked on dag.mu.
// While our read lock is held, Go stops admitting new readers only once a
// writer is waiting, so a blocking read-lock probe is what tells us the writer
// is queued rather than merely started.
func requireWriterQueued(t *testing.T, dag *DAG) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		// Probe: with our read lock held, a fresh read lock only succeeds once
		// the writer is gone, and only blocks once the writer is queued.
		// Buffered: on the timeout path this helper returns before the probe
		// sends, so an unbuffered channel would block that goroutine forever.
		probed := make(chan int, 1)
		go func() {
			dag.mu.RLock()
			nodes := len(dag.nodes) // real read: the section guards state
			dag.mu.RUnlock()
			probed <- nodes
		}()

		select {
		case <-probed:
			// Reader still admitted: the writer has not parked yet.
			time.Sleep(5 * time.Millisecond)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
	t.Fatal("writer never parked on the DAG mutex")
}

// TestRecalculate_ValidationRunsUnderSingleReadLock is the LGN-54 regression
// test for the recursive read lock. validateUpdateKeys used to take RLock and
// then call HasNode, which took RLock again: with a writer queued in between
// the two acquisitions, the second one waits for the writer while the writer
// waits for the first — the deadlock sync.RWMutex explicitly warns about.
// The validation must now read the graph under the lock the caller already
// holds instead of acquiring it a second time.
func TestRecalculate_ValidationRunsUnderSingleReadLock(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")

	var release sync.Once
	releaseRead := func() { release.Do(dag.mu.RUnlock) }
	t.Cleanup(releaseRead)

	// Simulate Recalculate holding the read lock.
	dag.mu.RLock()

	// Queue a writer behind it, exactly the window that used to deadlock.
	writerAcquired := make(chan struct{})
	go func() {
		dag.mu.Lock()
		close(writerAcquired)
		dag.mu.Unlock()
	}()
	requireWriterQueued(t, dag)

	validated := make(chan error, 1)
	go func() {
		validated <- dag.validateUpdateKeysLocked(map[string]any{"stats.str": 16.0})
	}()

	select {
	case err := <-validated:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		releaseRead()
		t.Fatal("validation reacquired the read lock while a writer was queued: recursive RLock (LGN-54)")
	}

	// Let the queued writer run before asserting it made progress.
	releaseRead()
	select {
	case <-writerAcquired:
	case <-time.After(2 * time.Second):
		t.Fatal("queued writer never acquired the write lock")
	}
}

// TestDAG_DoesNotExportLockMethods covers LGN-54: an embedded sync.RWMutex
// promoted Lock/Unlock/RLock/RUnlock onto DAG, so any package could freeze the
// graph, unlock a mutex it never locked (panic), or nest read locks and
// deadlock. The guard is private now.
func TestDAG_DoesNotExportLockMethods(t *testing.T) {
	dagType := reflect.TypeOf(&DAG{})
	for _, name := range []string{"Lock", "Unlock", "RLock", "RUnlock"} {
		if method, ok := dagType.MethodByName(name); ok {
			t.Errorf("DAG must not expose %s: found promoted method %s (LGN-54)", name, method.Type)
		}
	}
}

// TestDAG_TemplateIDIsPrivate documents that the template identity stays inside
// the DAG: it was an exported (therefore caller-writable) field while the rest
// of the graph was claimed to be immutable.
func TestDAG_TemplateIDIsPrivate(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")
	assert.Equal(t, "lgn54", dag.templateID)
}

// TestGetNode_ReturnsCopyIndependentOfDAG proves a caller cannot reach the
// graph through the value returned by GetNode: mutating the copy — including
// its Path slice — leaves the stored node untouched.
func TestGetNode_ReturnsCopyIndependentOfDAG(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")

	node, ok := dag.GetNode("stats.str")
	require.True(t, ok)
	require.True(t, node.IsEditable)

	node.IsEditable = false
	node.FormulaString = "tampered"
	node.Path[0] = "tampered"

	stored, ok := dag.GetNode("stats.str")
	require.True(t, ok)
	assert.True(t, stored.IsEditable, "mutating the returned copy changed the stored node")
	assert.Empty(t, stored.FormulaString)
	assert.Equal(t, "stats.str", stored.Path.String())

	missing, ok := dag.GetNode("stats.missing")
	assert.False(t, ok)
	assert.Equal(t, Node{}, missing)
}

// TestRecalculate_ConcurrentReaders drives the single-lock read path from many
// goroutines so `go test -race` actually observes concurrent DAG access.
func TestRecalculate_ConcurrentReaders(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")

	original := map[string]any{"stats": map[string]any{"str": 10.0}}
	wantBonus := 32.0 // 16 * 2

	const goroutines, iterations = 8, 25
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				changed, err := dag.Recalculate(original, map[string]any{"stats.str": 16.0})
				assert.NoError(t, err)
				assert.Equal(t, wantBonus, changed["derived.bonus"])

				assert.True(t, dag.HasNode("derived.bonus"))
				node, ok := dag.GetNode("derived.bonus")
				assert.True(t, ok)
				assert.False(t, node.IsEditable)
				assert.NotEmpty(t, dag.TopologicalOrder())
			}
		}()
	}
	wg.Wait()
}
