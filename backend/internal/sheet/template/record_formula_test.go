package template

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func savedFormulaFields() FieldGroup {
	fields := aggregateFields(map[string]*AggregateSchema{
		"inventoryWeight": aggregateSchema("inventory.items", "total", AggregateSum),
		"inventoryCount":  aggregateSchema("inventory.items", "quantity", AggregateCount),
	})
	fields["level"] = FieldNode{Schema: &FieldSchema{
		Type:       fieldTypeNumber,
		IsEditable: true,
		Default:    1,
	}}
	fields["inventoryWeightDouble"] = FieldNode{Schema: &FieldSchema{
		Type:      fieldTypeNumber,
		Formula:   "inventoryWeight * 2",
		DependsOn: [][]string{{"inventoryWeight"}},
	}}
	shape := fields["inventory"].Children["items"].List.Shape
	shape["scaled"] = FieldNode{Schema: &FieldSchema{
		Type:      fieldTypeNumber,
		Formula:   "record_quantity * level",
		DependsOn: [][]string{{"quantity"}, {"level"}},
	}}
	return fields
}

func savedFormulaDAG(t *testing.T) *DAG {
	t.Helper()
	dag, err := buildAggregateDAG(t, savedFormulaFields())
	require.NoError(t, err)
	return dag
}

func formulaRecord(id string, position float64, fields map[string]any) map[string]any {
	record := map[string]any{
		"name":            "",
		"quantity":        1.0,
		"weight":          2.0,
		"total":           2.0,
		"scaled":          1.0,
		ListPositionField: position,
	}
	for key, value := range fields {
		record[key] = value
	}
	return record
}

func formulaSheet(records ...map[string]any) map[string]any {
	list := make(map[string]any, len(records))
	weight := 0.0
	count := 0.0
	for i, record := range records {
		id := []string{validRecordID, secondRecordID, thirdRecordID}[i]
		list[id] = record
		weight += record["total"].(float64)
		if record["quantity"].(float64) != 0 {
			count++
		}
	}
	return map[string]any{
		"level": 1.0,
		"inventory": map[string]any{
			"items": list,
		},
		"inventoryWeight":       weight,
		"inventoryCount":        count,
		"inventoryWeightDouble": weight * 2,
	}
}

func TestDAG_CalculateSave_RecordFormulas(t *testing.T) {
	tests := []struct {
		name     string
		original map[string]any
		updates  map[string]any
		check    func(t *testing.T, calc SaveCalculation, err error)
	}{
		{
			name:     "Happy path: a new record evaluates its derived fields and feeds the aggregates",
			original: formulaSheet(),
			updates: map[string]any{
				"inventory.items." + validRecordID + ".quantity": 3.0,
				"inventory.items." + validRecordID + ".weight":   4.0,
			},
			check: func(t *testing.T, calc SaveCalculation, err error) {
				require.NoError(t, err)
				assert.Equal(t, map[string]any{
					"inventory.items." + validRecordID + ".total":  12.0,
					"inventory.items." + validRecordID + ".scaled": 3.0,
					"inventoryWeight":       12.0,
					"inventoryCount":        1.0,
					"inventoryWeightDouble": 24.0,
				}, calc.Cascade)
				record := listOf(calc.Merged)[validRecordID].(map[string]any)
				assert.InDelta(t, 12.0, record["total"], 0)
				assert.InDelta(t, 3.0, record["scaled"], 0)
			},
		},
		{
			name: "Happy path: editing one record leaves the other records derived values alone",
			original: formulaSheet(
				formulaRecord(validRecordID, 0, nil),
				formulaRecord(secondRecordID, 1, nil),
			),
			updates: map[string]any{
				"inventory.items." + validRecordID + ".quantity": 5.0,
			},
			check: func(t *testing.T, calc SaveCalculation, err error) {
				require.NoError(t, err)
				assert.Equal(t, map[string]any{
					"inventory.items." + validRecordID + ".total":  10.0,
					"inventory.items." + validRecordID + ".scaled": 5.0,
					"inventoryWeight":       12.0,
					"inventoryWeightDouble": 24.0,
				}, calc.Cascade)
			},
		},
		{
			name: "Happy path: removing the last record resets the aggregate to zero",
			original: formulaSheet(
				formulaRecord(validRecordID, 0, map[string]any{"quantity": 3.0, "total": 6.0, "scaled": 3.0}),
			),
			updates: map[string]any{
				"inventory.items." + validRecordID: nil,
			},
			check: func(t *testing.T, calc SaveCalculation, err error) {
				require.NoError(t, err)
				assert.Equal(t, map[string]any{
					"inventoryWeight":       0.0,
					"inventoryCount":        0.0,
					"inventoryWeightDouble": 0.0,
				}, calc.Cascade)
				assert.Empty(t, listOf(calc.Merged))
			},
		},
		{
			name:     "Edge: a no-op save cascades nothing",
			original: formulaSheet(formulaRecord(validRecordID, 0, nil)),
			updates: map[string]any{
				"inventory.items." + validRecordID + ".quantity": 1.0,
			},
			check: func(t *testing.T, calc SaveCalculation, err error) {
				require.NoError(t, err)
				assert.Empty(t, calc.Cascade)
			},
		},
		{
			name: "Happy path: changing a sheet field recomputes every record",
			original: formulaSheet(
				formulaRecord(validRecordID, 0, nil),
				formulaRecord(secondRecordID, 1, nil),
			),
			updates: map[string]any{"level": 4.0},
			check: func(t *testing.T, calc SaveCalculation, err error) {
				require.NoError(t, err)
				assert.Equal(t, map[string]any{
					"inventory.items." + validRecordID + ".scaled":  4.0,
					"inventory.items." + secondRecordID + ".scaled": 4.0,
				}, calc.Cascade)
			},
		},
		{
			name: "Happy path: count ignores zero values",
			original: formulaSheet(
				formulaRecord(validRecordID, 0, map[string]any{"quantity": 0.0, "total": 0.0, "scaled": 0.0}),
				formulaRecord(secondRecordID, 1, nil),
			),
			updates: map[string]any{
				"inventory.items." + thirdRecordID + ".quantity": 2.0,
			},
			check: func(t *testing.T, calc SaveCalculation, err error) {
				require.NoError(t, err)
				assert.InDelta(t, 2.0, calc.Cascade["inventoryCount"], 0)
				assert.InDelta(t, 6.0, calc.Cascade["inventoryWeight"], 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calc, err := savedFormulaDAG(t).CalculateSave(tc.original, tc.updates)
			tc.check(t, calc, err)
		})
	}
}

func TestDAG_CalculateSave_EmptyList(t *testing.T) {
	dag := savedFormulaDAG(t)

	t.Run("Edge: an empty list aggregates to zero", func(t *testing.T) {
		value := dag.aggregateValue(
			AggregateSchema{List: "inventory.items", Field: "total", Op: AggregateSum},
			map[string]any{"inventory": map[string]any{"items": map[string]any{}}},
		)
		assert.InDelta(t, 0.0, value, 0)
	})

	t.Run("Edge: a missing list aggregates to zero", func(t *testing.T) {
		value := dag.aggregateValue(
			AggregateSchema{List: "inventory.items", Field: "total", Op: AggregateCount},
			map[string]any{},
		)
		assert.InDelta(t, 0.0, value, 0)
	})

	t.Run("Edge: a nil original sheet is refused", func(t *testing.T) {
		calc, err := dag.CalculateSave(nil, nil)
		require.ErrorIs(t, err, ErrNilOriginalData)
		assert.Nil(t, calc.Cascade)
	})
}

func TestCalculateSave_ConcurrentWithWriter(t *testing.T) {
	dag := savedFormulaDAG(t)
	original := formulaSheet(
		formulaRecord(validRecordID, 0, nil),
		formulaRecord(secondRecordID, 1, nil),
	)

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
	for range goroutines {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range iterations {
				calc, err := dag.CalculateSave(original, map[string]any{
					"inventory.items." + validRecordID + ".quantity": 3.0,
				})
				assert.NoError(t, err)
				assert.InDelta(t, 3.0, calc.Cascade["inventory.items."+validRecordID+".scaled"], 1e-9)
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
		t.Fatal("CalculateSave deadlocked under writer contention: read lock acquired more than once?")
	}
	close(stop)
	writer.Wait()
}

func TestDAG_CalculateSave_RecordExclusiveGroups(t *testing.T) {
	fields := savedFormulaFields()
	shape := fields["inventory"].Children["items"].List.Shape
	shape["optionA"] = FieldNode{Schema: &FieldSchema{
		Type: fieldTypeNumber, IsEditable: true, ExclusiveGroup: "pick",
	}}
	shape["optionB"] = FieldNode{Schema: &FieldSchema{
		Type: fieldTypeNumber, IsEditable: true, ExclusiveGroup: "pick",
	}}
	dag, err := buildAggregateDAG(t, fields)
	require.NoError(t, err)

	original := formulaSheet(
		formulaRecord(validRecordID, 0, map[string]any{"optionA": 0.0, "optionB": 0.0}),
		formulaRecord(secondRecordID, 1, map[string]any{"optionA": 0.0, "optionB": 0.0}),
	)

	t.Run("Error: two members on in one record are refused", func(t *testing.T) {
		_, err := dag.CalculateSave(original, map[string]any{
			"inventory.items." + validRecordID + ".optionA": 1.0,
			"inventory.items." + validRecordID + ".optionB": 1.0,
		})
		var conflict *ExclusiveGroupConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Equal(t, "pick", conflict.Group)
	})

	t.Run("Happy path: two records each choose their own member", func(t *testing.T) {
		calc, err := dag.CalculateSave(original, map[string]any{
			"inventory.items." + validRecordID + ".optionA":  1.0,
			"inventory.items." + secondRecordID + ".optionB": 1.0,
		})
		require.NoError(t, err)
		first := listOf(calc.Merged)[validRecordID].(map[string]any)
		second := listOf(calc.Merged)[secondRecordID].(map[string]any)
		assert.InDelta(t, 1.0, first["optionA"], 0)
		assert.InDelta(t, 0.0, first["optionB"], 0)
		assert.InDelta(t, 0.0, second["optionA"], 0)
		assert.InDelta(t, 1.0, second["optionB"], 0)
	})

	t.Run("Edge: choosing a member clears the sibling already on", func(t *testing.T) {
		withAOn := formulaSheet(
			formulaRecord(validRecordID, 0, map[string]any{"optionA": 1.0, "optionB": 0.0}),
		)
		calc, err := dag.CalculateSave(withAOn, map[string]any{
			"inventory.items." + validRecordID + ".optionB": 1.0,
		})
		require.NoError(t, err)
		record := listOf(calc.Merged)[validRecordID].(map[string]any)
		assert.InDelta(t, 0.0, record["optionA"], 0)
		assert.InDelta(t, 1.0, record["optionB"], 0)
	})

	t.Run("Error: a non-numeric exclusive member is refused", func(t *testing.T) {
		_, err := dag.CalculateSave(original, map[string]any{
			"inventory.items." + validRecordID + ".optionA": "on",
		})
		var invalid *ExclusiveValueTypeError
		require.ErrorAs(t, err, &invalid)
		assert.Equal(t, "pick", invalid.Group)
	})
}
