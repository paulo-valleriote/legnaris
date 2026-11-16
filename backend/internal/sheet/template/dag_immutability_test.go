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

func requireWriterQueued(t *testing.T, dag *DAG) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		probed := make(chan int, 1)
		go func() {
			dag.mu.RLock()
			nodes := len(dag.nodes)
			dag.mu.RUnlock()
			probed <- nodes
		}()

		select {
		case <-probed:
			time.Sleep(5 * time.Millisecond)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
	t.Fatal("writer never parked on the DAG mutex")
}

func TestRecalculate_ValidationRunsUnderSingleReadLock(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")

	var release sync.Once
	releaseRead := func() { release.Do(dag.mu.RUnlock) }
	t.Cleanup(releaseRead)

	dag.mu.RLock()

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

	releaseRead()
	select {
	case <-writerAcquired:
	case <-time.After(2 * time.Second):
		t.Fatal("queued writer never acquired the write lock")
	}
}

func TestDAG_DoesNotExportLockMethods(t *testing.T) {
	dagType := reflect.TypeOf(&DAG{})
	for _, name := range []string{"Lock", "Unlock", "RLock", "RUnlock"} {
		if method, ok := dagType.MethodByName(name); ok {
			t.Errorf("DAG must not expose %s: found promoted method %s (LGN-54)", name, method.Type)
		}
	}
}

func TestDAG_TemplateIDIsPrivate(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")
	assert.Equal(t, "lgn54", dag.templateID)
}

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

func TestRecalculate_ConcurrentReaders(t *testing.T) {
	dag := buildImmutabilityTestDAG(t, "lgn54")

	original := map[string]any{"stats": map[string]any{"str": 10.0}}
	wantBonus := 32.0

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
