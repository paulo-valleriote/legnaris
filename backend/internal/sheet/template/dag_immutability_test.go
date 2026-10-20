package template

import (
	"legnaris/backend/internal/engine"
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

// TestNode_ExposesNoPointerFields covers the residual LGN-54 hole the second
// AI review found: GetNode hands out a value copy, but an exported pointer
// field would still alias DAG state — CompiledFormula used to share the
// *vm.Program every concurrent Recalculate evaluates. The compiled program is
// private now, and the remaining exported fields are only a string, a bool and
// the Path slice that GetNode clones.
func TestNode_ExposesNoPointerFields(t *testing.T) {
	nodeType := reflect.TypeOf(Node{})

	for i := 0; i < nodeType.NumField(); i++ {
		field := nodeType.Field(i)
		if !field.IsExported() {
			continue
		}
		if field.Type.Kind() == reflect.Pointer {
			t.Errorf("exported field Node.%s is a pointer: GetNode copies would alias DAG state (LGN-54)", field.Name)
		}
	}

	_, exported := nodeType.FieldByName("CompiledFormula")
	assert.False(t, exported,
		"Node.CompiledFormula must stay unexported so callers cannot mutate the compiled program shared by Recalculate")
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
			}
		}()
	}
	wg.Wait()
}

// TestNormalizeAndRecalculate_ConcurrentWithWriter drives the single-lock PATCH
// entry point while a writer keeps taking the write lock: the read path must
// acquire the read lock exactly once per call and never deadlock against a
// queued writer (the failure mode a recursive RLock would produce, LGN-54).
func TestNormalizeAndRecalculate_ConcurrentWithWriter(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")

	original := map[string]any{"stats": map[string]any{"str": 10.0}}

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for {
			select {
			case <-stop:
				return
			default:
				dag.mu.Lock()
				dag.mu.Unlock()
			}
		}
	}()

	const goroutines, iterations = 8, 25
	var readers sync.WaitGroup
	done := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for i := 0; i < iterations; i++ {
				normalized, cascading, err := dag.NormalizeAndRecalculate(
					original,
					map[string]any{"stats.str": 16.0},
				)
				assert.NoError(t, err)
				assert.Equal(t, 16.0, normalized["stats.str"])
				assert.Equal(t, 32.0, cascading["derived.bonus"])
			}
		}()
	}

	go func() {
		readers.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		close(stop)
		writer.Wait()
		t.Fatal("NormalizeAndRecalculate deadlocked under writer contention: read lock acquired more than once?")
	}
	close(stop)
	writer.Wait()
}
