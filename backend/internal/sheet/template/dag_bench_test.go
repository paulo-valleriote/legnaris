package template

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	benchFieldCount  = 200
	benchEditable    = benchFieldCount / 2
	benchSamples     = 2000
	benchP95Budget   = 50 * time.Millisecond
	benchP95Fraction = 0.95
)

// buildChainDAG builds a template of benchFieldCount fields: benchEditable
// editable leaves e000.. and an equally long derived chain d000.. where d000
// reads e000 and every other dNNN reads its predecessor. Updating e000 is the
// worst case for a cascade: it recomputes the whole chain.
func buildChainDAG(tb testing.TB) (*DAG, map[string]any) {
	tb.Helper()

	fields := FieldGroup{}
	sheet := map[string]any{}
	for i := range benchEditable {
		editKey := fmt.Sprintf("e%03d", i)
		fields[editKey] = FieldNode{Schema: &FieldSchema{Type: "number", IsEditable: true}}
		sheet[editKey] = 1.0

		derivedKey := fmt.Sprintf("d%03d", i)
		source := editKey
		if i > 0 {
			source = fmt.Sprintf("d%03d", i-1)
		}
		fields[derivedKey] = FieldNode{Schema: &FieldSchema{
			Type:      "number",
			Formula:   source + " + 1",
			DependsOn: [][]string{{source}},
		}}
		sheet[derivedKey] = 2.0
	}

	dag, err := BuildDAG("bench_chain", fields, engine.NewFormulaEngine())
	require.NoError(tb, err)
	return dag, sheet
}

func BenchmarkNormalizeAndRecalculate_Chain200(b *testing.B) {
	dag, sheet := buildChainDAG(b)
	b.ResetTimer()
	for i := range b.N {
		_, _, err := dag.NormalizeAndRecalculate(sheet, map[string]any{"e000": float64(i%1000 + 2)})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormalizeAndRecalculate_Chain200Parallel(b *testing.B) {
	dag, sheet := buildChainDAG(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			_, _, err := dag.NormalizeAndRecalculate(sheet, map[string]any{"e000": float64(i%1000 + 2)})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkNormalizeAndRecalculate_FantasyV1(b *testing.B) {
	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(b, err)
	dag, ok := registry.DAG("fantasy_v1")
	require.True(b, ok)

	tpl, ok := registry.FetchByID("fantasy_v1")
	require.True(b, ok)
	sheet := seedGroup(tpl.Fields)

	b.ResetTimer()
	for i := range b.N {
		_, _, err := dag.NormalizeAndRecalculate(sheet, map[string]any{"stats.strength": float64(i%20 + 1)})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// seedGroup mirrors the sheet seeding Create performs: every leaf starts at its
// SeedValue, nested groups stay nested.
func seedGroup(group FieldGroup) map[string]any {
	out := make(map[string]any, len(group))
	for key, node := range group {
		if node.Schema != nil {
			out[key] = node.Schema.SeedValue()
			continue
		}
		out[key] = seedGroup(node.Children)
	}
	return out
}

// TestRecalculateChain200_P95UnderBudget pins the LGN-33 acceptance criterion:
// the p95 of a worst-case cascade over a 200-field template stays under 50ms.
func TestRecalculateChain200_P95UnderBudget(t *testing.T) {
	dag, sheet := buildChainDAG(t)

	durations := make([]time.Duration, benchSamples)
	for i := range durations {
		start := time.Now()
		_, cascading, err := dag.NormalizeAndRecalculate(sheet, map[string]any{"e000": float64(i%1000 + 2)})
		durations[i] = time.Since(start)
		require.NoError(t, err)
		require.Len(t, cascading, benchEditable, "the whole derived chain must be recomputed")
	}

	slices.Sort(durations)
	p95 := durations[int(float64(len(durations))*benchP95Fraction)-1]
	assert.Less(t, p95, benchP95Budget)
}
