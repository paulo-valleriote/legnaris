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

func buildListAtCapacityDAG(b *testing.B) (*DAG, map[string]any) {
	b.Helper()

	fields := FieldGroup{
		"level": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 1}},
		"inventory": {Children: FieldGroup{
			"items": {List: &ListSchema{
				Label:    "Items",
				Capacity: MaxListCapacity,
				Shape: FieldGroup{
					"quantity": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 1}},
					"weight":   {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 1}},
					"total": {Schema: &FieldSchema{
						Type:      "number",
						Formula:   "record_quantity * record_weight",
						DependsOn: [][]string{{"quantity"}, {"weight"}},
					}},
					"scaled": {Schema: &FieldSchema{
						Type:      "number",
						Formula:   "record_quantity * level",
						DependsOn: [][]string{{"quantity"}, {"level"}},
					}},
				},
			}},
		}},
		"inventoryValue": {Schema: &FieldSchema{
			Type:      "number",
			Aggregate: &AggregateSchema{List: "inventory.items", Field: "total", Op: AggregateSum},
		}},
	}
	dag, err := BuildDAG("bench_list_capacity", fields, engine.NewFormulaEngine())
	require.NoError(b, err)

	records := make(map[string]any, MaxListCapacity)
	for i := range MaxListCapacity {
		records[fmt.Sprintf("r%032x", i)] = map[string]any{
			"quantity":        1.0,
			"weight":          1.0,
			"total":           1.0,
			"scaled":          1.0,
			ListPositionField: float64(i),
		}
	}
	sheet := map[string]any{
		"level":          1.0,
		"inventory":      map[string]any{"items": records},
		"inventoryValue": float64(MaxListCapacity),
	}
	return dag, sheet
}

func BenchmarkCalculateSave_ListCapacity200(b *testing.B) {
	firstRecord := fmt.Sprintf("r%032x", 0)

	b.Run("record edit", func(b *testing.B) {
		dag, sheet := buildListAtCapacityDAG(b)
		b.ResetTimer()
		for i := range b.N {
			_, err := dag.CalculateSave(sheet, map[string]any{
				"inventory.items." + firstRecord + ".quantity": float64(i%10 + 2),
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("sheet field change", func(b *testing.B) {
		dag, sheet := buildListAtCapacityDAG(b)
		b.ResetTimer()
		for i := range b.N {
			_, err := dag.CalculateSave(sheet, map[string]any{"level": float64(i%10 + 2)})
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
