package template

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	resetTestListPath    = "inventory.items"
	resetTestRecordID    = "r0123456789abcdef0123456789abcdef"
	resetTestRecordID2   = "r0123456789abcdef0123456789abcdee"
	resetTestNumberField = "number"
)

func resetTestFields() FieldGroup {
	return FieldGroup{
		"level": {Schema: &FieldSchema{Type: resetTestNumberField, IsEditable: true, Default: 1}},
		"hp":    {Schema: &FieldSchema{Type: resetTestNumberField, IsEditable: true, Default: 10}},
		"note":  {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true}},
		"derived": {Schema: &FieldSchema{
			Type:      resetTestNumberField,
			Formula:   "level * 2",
			DependsOn: [][]string{{"level"}},
		}},
		"inventory": {Children: FieldGroup{
			"items": {List: &ListSchema{
				Label:    "Items",
				Capacity: 10,
				Shape: FieldGroup{
					"name": {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true}},
					"quantity": {Schema: &FieldSchema{
						Type: resetTestNumberField, IsEditable: true, Default: 1,
					}},
					"active": {Schema: &FieldSchema{
						Type: resetTestNumberField, IsEditable: true, Default: 0, Flag: true,
					}},
					"total": {Schema: &FieldSchema{
						Type:      resetTestNumberField,
						Formula:   "record_quantity * 2",
						DependsOn: [][]string{{"quantity"}},
					}},
				},
			}},
		}},
	}
}

func buildResetDAG(t *testing.T, actions []ResetAction) (*DAG, error) {
	t.Helper()
	return BuildDAGWithActions("reset_test_v1", resetTestFields(), actions, engine.NewFormulaEngine())
}

func mustResetDAG(t *testing.T, actions []ResetAction) *DAG {
	t.Helper()
	dag, err := buildResetDAG(t, actions)
	require.NoError(t, err)
	return dag
}

func consumeAction() ResetAction {
	return ResetAction{
		ID:    "consume",
		Label: "Consume",
		Assignments: []ResetAssignment{
			{Target: "level", Expression: "max(0, level - 1)"},
			{Target: "hp", Expression: "level"},
		},
	}
}

func restockAction(filter string) ResetAction {
	return ResetAction{
		ID:    "restock",
		Label: "Restock",
		Assignments: []ResetAssignment{
			{Target: resetTestListPath + ".quantity", Expression: "record_quantity + 1", Filter: filter},
		},
	}
}

func TestBuildDAG_ResetActions(t *testing.T) {
	tests := []struct {
		name   string
		action []ResetAction
		check  func(t *testing.T, err error)
	}{
		{
			name: "Happy path: static and list field assignments build",
			action: []ResetAction{
				consumeAction(),
				restockAction("active"),
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Happy path: an action with no assignments is a valid no-op",
			action: []ResetAction{
				{ID: "noop", Label: "Nothing"},
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:   "Error: duplicate action ids fail boot",
			action: []ResetAction{consumeAction(), consumeAction()},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "duplicate")
			},
		},
		{
			name: "Error: an action id outside the slug grammar fails boot",
			action: []ResetAction{{
				ID:          "Long Rest",
				Assignments: []ResetAssignment{{Target: "level", Expression: "0"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "id must match")
			},
		},
		{
			name: "Error: an unknown static target fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: "ghost", Expression: "0"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "is not a field")
			},
		},
		{
			name: "Error: a derived static target fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: "derived", Expression: "0"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "is derived")
			},
		},
		{
			name: "Error: a text static target fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: "note", Expression: "0"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "must be a number field")
			},
		},
		{
			name: "Error: a text list field fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: resetTestListPath + ".name", Expression: "0"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "must be a number field")
			},
		},
		{
			name: "Error: an unknown list field fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: resetTestListPath + ".ghost", Expression: "0"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "is not a list field")
			},
		},
		{
			name: "Error: a derived list field fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: resetTestListPath + ".total", Expression: "0"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "is derived")
			},
		},
		{
			name: "Error: an uncompilable static expression fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: "level", Expression: "level +"}},
			}},
			check: func(t *testing.T, err error) {
				var compileErr *FormulaCompileError
				require.ErrorAs(t, err, &compileErr)
				assert.Equal(t, "reset.rest.level", compileErr.NodeID)
			},
		},
		{
			name: "Error: an uncompilable record expression fails boot",
			action: []ResetAction{{
				ID: "rest",
				Assignments: []ResetAssignment{{
					Target:     resetTestListPath + ".quantity",
					Expression: "record_missing + 1",
				}},
			}},
			check: func(t *testing.T, err error) {
				var compileErr *FormulaCompileError
				require.ErrorAs(t, err, &compileErr)
			},
		},
		{
			name: "Error: an empty expression fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: "level", Expression: ""}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "empty")
			},
		},
		{
			name: "Error: a filter naming no shape field fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: resetTestListPath + ".quantity", Expression: "0", Filter: "ghost"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "filter")
			},
		},
		{
			name: "Error: a text filter fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: resetTestListPath + ".quantity", Expression: "0", Filter: "name"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "must be a number field")
			},
		},
		{
			name: "Error: a filter on a static target fails boot",
			action: []ResetAction{{
				ID:          "rest",
				Assignments: []ResetAssignment{{Target: "level", Expression: "0", Filter: "active"}},
			}},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResetActionError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Reason, "requires a list field target")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildResetDAG(t, tc.action)
			tc.check(t, err)
		})
	}
}

func resetTestData(level, hp float64, items map[string]any) map[string]any {
	return map[string]any{
		"level":   level,
		"hp":      hp,
		"note":    "",
		"derived": level * 2,
		"inventory": map[string]any{
			"items": items,
		},
	}
}

func resetTestRecord(quantity, active float64) map[string]any {
	return map[string]any{"quantity": quantity, "active": active, "total": quantity * 2}
}

func TestCalculateReset(t *testing.T) {
	t.Run("Happy path: a decrement floors at zero and reads the state before the action", func(t *testing.T) {
		dag := mustResetDAG(t, []ResetAction{consumeAction()})
		data := resetTestData(5, 3, map[string]any{})

		calculation, err := dag.CalculateReset("consume", data)
		require.NoError(t, err)
		assert.InDelta(t, 4.0, calculation.Normalized["level"], 1e-9)
		assert.InDelta(t, 5.0, calculation.Normalized["hp"], 1e-9, "hp reads the level before the action")
		assert.InDelta(t, 8.0, calculation.Merged["derived"], 1e-9)
		assert.InDelta(t, 8.0, calculation.Cascade["derived"], 1e-9)
		assert.Equal(t, []string{"hp", "level"}, calculation.ChangedKeys)
	})

	t.Run("Edge: nothing to change still returns the assignment and no changed key", func(t *testing.T) {
		dag := mustResetDAG(t, []ResetAction{consumeAction()})
		data := resetTestData(0, 0, map[string]any{})

		calculation, err := dag.CalculateReset("consume", data)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, calculation.Normalized["level"], 1e-9)
		assert.Empty(t, calculation.Cascade)
		assert.Empty(t, calculation.ChangedKeys)
	})

	t.Run("Edge: an empty list changes nothing", func(t *testing.T) {
		dag := mustResetDAG(t, []ResetAction{restockAction("")})
		data := resetTestData(1, 1, map[string]any{})

		calculation, err := dag.CalculateReset("restock", data)
		require.NoError(t, err)
		assert.Empty(t, calculation.Normalized)
		assert.Empty(t, calculation.RecordUpdates)
		assert.Empty(t, calculation.ChangedKeys)
	})

	t.Run("Edge: a filter that matches no record changes nothing", func(t *testing.T) {
		dag := mustResetDAG(t, []ResetAction{restockAction("active")})
		data := resetTestData(1, 1, map[string]any{
			resetTestRecordID: resetTestRecord(2, 0),
		})

		calculation, err := dag.CalculateReset("restock", data)
		require.NoError(t, err)
		assert.Empty(t, calculation.ChangedKeys)
		assert.InDelta(t, 2.0, mapAt(calculation.Merged, resetTestListPath, resetTestRecordID)["quantity"], 1e-9)
	})

	t.Run("Edge: a filter that matches some records changes only those", func(t *testing.T) {
		dag := mustResetDAG(t, []ResetAction{restockAction("active")})
		data := resetTestData(1, 1, map[string]any{
			resetTestRecordID:  resetTestRecord(2, 1),
			resetTestRecordID2: resetTestRecord(5, 0),
		})

		calculation, err := dag.CalculateReset("restock", data)
		require.NoError(t, err)
		assert.Equal(t, []string{resetTestListPath + "." + resetTestRecordID + ".quantity"}, calculation.ChangedKeys)
		assert.InDelta(t, 3.0, mapAt(calculation.Merged, resetTestListPath, resetTestRecordID)["quantity"], 1e-9)
		assert.InDelta(t, 5.0, mapAt(calculation.Merged, resetTestListPath, resetTestRecordID2)["quantity"], 1e-9)
	})

	t.Run("Edge: a list expression sees the current record and recalculates its derived fields", func(t *testing.T) {
		action := ResetAction{
			ID: "double",
			Assignments: []ResetAssignment{{
				Target:     resetTestListPath + ".quantity",
				Expression: "record_quantity * 2",
			}},
		}
		dag := mustResetDAG(t, []ResetAction{action})
		data := resetTestData(1, 1, map[string]any{
			resetTestRecordID: resetTestRecord(3, 1),
		})

		calculation, err := dag.CalculateReset("double", data)
		require.NoError(t, err)
		assert.InDelta(t, 6.0, mapAt(calculation.Merged, resetTestListPath, resetTestRecordID)["quantity"], 1e-9)
		assert.InDelta(t, 12.0, mapAt(calculation.Merged, resetTestListPath, resetTestRecordID)["total"], 1e-9)
	})

	t.Run("Error: an unknown action is typed", func(t *testing.T) {
		dag := mustResetDAG(t, []ResetAction{consumeAction()})

		_, err := dag.CalculateReset("ghost", resetTestData(1, 1, map[string]any{}))
		var unknown *UnknownResetActionError
		require.ErrorAs(t, err, &unknown)
		assert.Equal(t, "ghost", unknown.ID)
	})

	t.Run("Error: nil data is rejected", func(t *testing.T) {
		dag := mustResetDAG(t, []ResetAction{consumeAction()})

		_, err := dag.CalculateReset("consume", nil)
		assert.ErrorIs(t, err, ErrNilOriginalData)
	})
}

func mapAt(root map[string]any, listPath, recordID string) map[string]any {
	list, _ := root["inventory"].(map[string]any)
	items, _ := list["items"].(map[string]any)
	record, _ := items[recordID].(map[string]any)
	return record
}

func TestTemplateDTO_ResetActions(t *testing.T) {
	t.Run("Happy path: declared actions marshal under actions", func(t *testing.T) {
		tmpl := Template{
			ID:      "reset_dto_v1",
			Name:    "Reset DTO",
			Fields:  resetTestFields(),
			Actions: []ResetAction{restockAction("active")},
		}

		encoded, err := json.Marshal(tmpl.ToDTO())
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &raw))
		require.Contains(t, raw, "actions")

		var actions []ResetActionDTO
		require.NoError(t, json.Unmarshal(raw["actions"], &actions))
		require.Len(t, actions, 1)
		assert.Equal(t, "restock", actions[0].ID)
		assert.Equal(t, "Restock", actions[0].Label)
		require.Len(t, actions[0].Assignments, 1)
		assert.Equal(t, resetTestListPath+".quantity", actions[0].Assignments[0].Target)
		assert.Equal(t, "record_quantity + 1", actions[0].Assignments[0].Expression)
		assert.Equal(t, "active", actions[0].Assignments[0].Filter)
	})

	t.Run("Edge: a template without actions omits the key", func(t *testing.T) {
		tmpl := Template{ID: "no_actions_v1", Name: "No Actions", Fields: resetTestFields()}

		encoded, err := json.Marshal(tmpl.ToDTO())
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &raw))
		assert.NotContains(t, raw, "actions")
	})

	t.Run("Edge: fantasy_v1 carries no actions in its DTO", func(t *testing.T) {
		encoded, err := json.Marshal(fantasyV1Template.ToDTO())
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &raw))
		assert.NotContains(t, raw, "actions")
	})
}

func TestBuildDAG_RecordObjectIsNotAFormulaInput(t *testing.T) {
	t.Run("Error: a Record formula reading record.x fails boot", func(t *testing.T) {
		fields := resetTestFields()
		fields["inventory"].Children["items"].List.Shape["bad"] = FieldNode{Schema: &FieldSchema{
			Type:      resetTestNumberField,
			Formula:   "record.quantity * 2",
			DependsOn: [][]string{{"quantity"}},
		}}

		_, err := BuildDAG("record_object_forbidden", fields, engine.NewFormulaEngine())

		require.Error(t, err)
		var compileErr *FormulaCompileError
		require.ErrorAs(t, err, &compileErr)
	})

	t.Run("Error: a Reset list expression reading record.x fails boot", func(t *testing.T) {
		action := ResetAction{
			ID:    "read_record",
			Label: "Read record",
			Assignments: []ResetAssignment{{
				Target:     resetTestListPath + ".quantity",
				Expression: "record.quantity + 1",
			}},
		}

		_, err := buildResetDAG(t, []ResetAction{action})

		require.Error(t, err)
		var compileErr *FormulaCompileError
		require.ErrorAs(t, err, &compileErr)
	})

	t.Run("Happy path: a Reset list expression still reads record_<field>", func(t *testing.T) {
		_, err := buildResetDAG(t, []ResetAction{restockAction("")})
		require.NoError(t, err)
	})
}
