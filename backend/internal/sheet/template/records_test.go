package template

import (
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validRecordID  = "r0123456789abcdef0123456789abcdef"
	secondRecordID = "r0123456789abcdef0123456789abcdee"
	thirdRecordID  = "r0123456789abcdef0123456789abcde1"
)

func recordShape() FieldGroup {
	maxLength := 5
	return FieldGroup{
		"name": {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true, MaxLength: &maxLength}},
		"quantity": {
			Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 1},
		},
		"active": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 0}},
		"derivedTotal": {Schema: &FieldSchema{
			Type:       fieldTypeNumber,
			IsEditable: false,
			Formula:    "record_quantity * 2",
			DependsOn:  [][]string{{"quantity"}},
		}},
	}
}

func recordsDAG(t *testing.T) *DAG {
	t.Helper()
	fields := FieldGroup{
		"name": {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true}},
		"inventory": {Children: FieldGroup{
			"items": {List: &ListSchema{
				Label:    "Items",
				Columns:  []string{"name", "quantity"},
				Capacity: 2,
				Shape:    recordShape(),
			}},
		}},
	}
	dag, err := BuildDAG("records_test_v1", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

func TestDAG_SplitRecordUpdates(t *testing.T) {
	dag := recordsDAG(t)

	tests := []struct {
		name       string
		updates    map[string]any
		wantStatic map[string]any
		want       []RecordUpdate
		check      func(t *testing.T, err error)
	}{
		{
			name:       "Happy path: a static leaf stays a static update",
			updates:    map[string]any{"name": "Aria"},
			wantStatic: map[string]any{"name": "Aria"},
			want:       []RecordUpdate{},
		},
		{
			name:       "Happy path: an empty map at the list path is a no-op",
			updates:    map[string]any{"inventory.items": map[string]any{}},
			wantStatic: map[string]any{},
			want:       []RecordUpdate{},
		},
		{
			name:       "Happy path: an empty map at the parent group is a no-op",
			updates:    map[string]any{"inventory": map[string]any{}},
			wantStatic: map[string]any{},
			want:       []RecordUpdate{},
		},
		{
			name:       "Happy path: null at a record path removes it",
			updates:    map[string]any{"inventory.items." + validRecordID: nil},
			wantStatic: map[string]any{},
			want: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: validRecordID,
				Fields:   map[string]any{},
				Remove:   true,
			}},
		},
		{
			name:       "Happy path: an empty map at a new record path creates it",
			updates:    map[string]any{"inventory.items." + validRecordID: map[string]any{}},
			wantStatic: map[string]any{},
			want: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: validRecordID,
				Fields:   map[string]any{},
			}},
		},
		{
			name: "Happy path: fields and position on one record stay together",
			updates: map[string]any{
				"inventory.items." + validRecordID + ".name":     "Rope",
				"inventory.items." + validRecordID + ".position": 3.0,
			},
			wantStatic: map[string]any{},
			want: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: validRecordID,
				Fields:   map[string]any{"name": "Rope", "position": 3.0},
			}},
		},
		{
			name:       "Error: a malformed record id",
			updates:    map[string]any{"inventory.items.rXYZ.name": "Rope"},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *RecordIDError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, "rXYZ", target.ID)
			},
		},
		{
			name:       "Error: an uppercase record id",
			updates:    map[string]any{"inventory.items.r0123456789ABCDEF0123456789abcdef.name": "Rope"},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *RecordIDError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name:       "Error: an unknown record field",
			updates:    map[string]any{"inventory.items." + validRecordID + ".weight": 2.0},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *UnknownRecordFieldError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, "weight", target.Field)
			},
		},
		{
			name:       "Error: a derived record field",
			updates:    map[string]any{"inventory.items." + validRecordID + ".derivedTotal": 2.0},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *NotEditableError
				require.ErrorAs(t, err, &target)
				assert.Contains(t, target.Key, "derivedTotal")
			},
		},
		{
			name:       "Error: a non-numeric position",
			updates:    map[string]any{"inventory.items." + validRecordID + ".position": "first"},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *RecordPositionError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name:       "Error: null at a record field",
			updates:    map[string]any{"inventory.items." + validRecordID + ".name": nil},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *InvalidRecordValueError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name:       "Error: a record path deeper than a field",
			updates:    map[string]any{"inventory.items." + validRecordID + ".name.extra": "x"},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *UnknownRecordFieldError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name:       "Error: a scalar at the record path",
			updates:    map[string]any{"inventory.items." + validRecordID: 5.0},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *InvalidRecordValueError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name:       "Error: null at the list path",
			updates:    map[string]any{"inventory.items": nil},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *InvalidRecordValueError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name:       "Error: a text value over max_length",
			updates:    map[string]any{"inventory.items." + validRecordID + ".name": "too long"},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *TextTooLongError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, 8, target.Length)
				assert.Equal(t, 5, target.Max)
			},
		},
		{
			name:       "Error: a flag value other than zero or one",
			updates:    map[string]any{"inventory.items." + validRecordID + ".active": 2.0},
			wantStatic: map[string]any{},
			check: func(t *testing.T, err error) {
				var target *FlagValueError
				require.ErrorAs(t, err, &target)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			static, updates, err := dag.SplitRecordUpdates(tc.updates)
			if tc.check != nil {
				tc.check(t, err)
				assert.Nil(t, static)
				assert.Nil(t, updates)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatic, static)
			assert.Equal(t, tc.want, updates)
		})
	}
}

func recordsData(records ...storedRecord) map[string]any {
	list := make(map[string]any, len(records))
	for _, record := range records {
		list[record.id] = map[string]any{
			"name":            "Rope",
			"quantity":        1.0,
			"active":          0.0,
			ListPositionField: record.position,
		}
	}
	return map[string]any{
		"name": "Aria",
		"inventory": map[string]any{
			"items": list,
		},
	}
}

type storedRecord struct {
	id       string
	position float64
}

func existingRecord(id string, position float64) storedRecord {
	return storedRecord{id: id, position: position}
}

func listOf(data map[string]any) map[string]any {
	return data["inventory"].(map[string]any)["items"].(map[string]any)
}

func TestDAG_ApplyRecordUpdates(t *testing.T) {
	dag := recordsDAG(t)

	tests := []struct {
		name    string
		data    func() map[string]any
		updates []RecordUpdate
		check   func(t *testing.T, data map[string]any, err error)
	}{
		{
			name: "Happy path: a new record is seeded from the shape and appended",
			data: func() map[string]any { return recordsData() },
			updates: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: validRecordID,
				Fields:   map[string]any{"name": "Rope"},
			}},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				assert.Equal(t, map[string]any{
					"name":            "Rope",
					"quantity":        1.0,
					"active":          0.0,
					"derivedTotal":    0.0,
					ListPositionField: 0.0,
				}, listOf(data)[validRecordID])
			},
		},
		{
			name: "Happy path: an explicit position is kept",
			data: func() map[string]any { return recordsData() },
			updates: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: validRecordID,
				Fields:   map[string]any{"position": 7.0},
			}},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				record := listOf(data)[validRecordID].(map[string]any)
				assert.InDelta(t, 7.0, record[ListPositionField], 0)
			},
		},
		{
			name: "Happy path: a new record after an existing one takes the next position",
			data: func() map[string]any {
				return recordsData(existingRecord(secondRecordID, 4))
			},
			updates: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: validRecordID,
				Fields:   map[string]any{"name": "Torch"},
			}},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				record := listOf(data)[validRecordID].(map[string]any)
				assert.InDelta(t, 5.0, record[ListPositionField], 0)
			},
		},
		{
			name: "Happy path: removing an existing record",
			data: func() map[string]any {
				return recordsData(existingRecord(secondRecordID, 0))
			},
			updates: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: secondRecordID,
				Fields:   map[string]any{},
				Remove:   true,
			}},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				assert.Empty(t, listOf(data))
			},
		},
		{
			name: "Happy path: removing an absent record is a no-op",
			data: func() map[string]any { return recordsData() },
			updates: []RecordUpdate{{
				ListPath: "inventory.items",
				RecordID: secondRecordID,
				Fields:   map[string]any{},
				Remove:   true,
			}},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				assert.Empty(t, listOf(data))
			},
		},
		{
			name: "Edge: writing to a record removed in the same patch leaves it removed",
			data: func() map[string]any {
				return recordsData(existingRecord(secondRecordID, 0))
			},
			updates: []RecordUpdate{
				{
					ListPath: "inventory.items",
					RecordID: validRecordID,
					Fields:   map[string]any{"name": "Torch"},
				},
				{
					ListPath: "inventory.items",
					RecordID: validRecordID,
					Fields:   map[string]any{},
					Remove:   true,
				},
			},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				_, exists := listOf(data)[validRecordID]
				assert.False(t, exists)
				_, otherExists := listOf(data)[secondRecordID]
				assert.True(t, otherExists)
			},
		},
		{
			name: "Edge: clearing the last record leaves an empty list that accepts a new record",
			data: func() map[string]any {
				return recordsData(existingRecord(secondRecordID, 0))
			},
			updates: []RecordUpdate{
				{
					ListPath: "inventory.items",
					RecordID: secondRecordID,
					Fields:   map[string]any{},
					Remove:   true,
				},
				{
					ListPath: "inventory.items",
					RecordID: validRecordID,
					Fields:   map[string]any{"name": "Torch"},
				},
			},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				assert.Len(t, listOf(data), 1)
				assert.Equal(t, "Torch", listOf(data)[validRecordID].(map[string]any)["name"])
			},
		},
		{
			name: "Error: capacity is checked on the merged result",
			data: func() map[string]any {
				return recordsData(existingRecord(secondRecordID, 0))
			},
			updates: []RecordUpdate{
				{
					ListPath: "inventory.items",
					RecordID: validRecordID,
					Fields:   map[string]any{"name": "Torch"},
				},
				{
					ListPath: "inventory.items",
					RecordID: thirdRecordID,
					Fields:   map[string]any{"name": "Rations"},
				},
			},
			check: func(t *testing.T, data map[string]any, err error) {
				var target *ListCapacityExceededError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, 3, target.Count)
				assert.Equal(t, 2, target.Capacity)
			},
		},
		{
			name: "Happy path: replacing a record at capacity stays at capacity",
			data: func() map[string]any {
				return recordsData(existingRecord(secondRecordID, 0))
			},
			updates: []RecordUpdate{
				{
					ListPath: "inventory.items",
					RecordID: secondRecordID,
					Fields:   map[string]any{},
					Remove:   true,
				},
				{
					ListPath: "inventory.items",
					RecordID: validRecordID,
					Fields:   map[string]any{"name": "Torch"},
				},
			},
			check: func(t *testing.T, data map[string]any, err error) {
				require.NoError(t, err)
				assert.Len(t, listOf(data), 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.data()
			err := dag.ApplyRecordUpdates(data, tc.updates)
			tc.check(t, data, err)
		})
	}
}

func TestDAG_ApplyRecordUpdates_LeavesStaticDataAlone(t *testing.T) {
	dag := recordsDAG(t)
	data := recordsData()

	err := dag.ApplyRecordUpdates(data, []RecordUpdate{{
		ListPath: "inventory.items",
		RecordID: validRecordID,
		Fields:   map[string]any{"name": "Torch"},
	}})
	require.NoError(t, err)
	assert.Equal(t, "Aria", data["name"])
}

func TestDAG_DropDerivedRecordValues(t *testing.T) {
	dag := recordsDAG(t)

	tests := []struct {
		name string
		flat map[string]any
		want map[string]any
	}{
		{
			name: "Happy path: an editable Record field and position are kept",
			flat: map[string]any{
				"inventory.items." + validRecordID + ".quantity": 2.0,
				"inventory.items." + validRecordID + ".position": 1.0,
			},
			want: map[string]any{
				"inventory.items." + validRecordID + ".quantity": 2.0,
				"inventory.items." + validRecordID + ".position": 1.0,
			},
		},
		{
			name: "Happy path: a derived Record field is dropped",
			flat: map[string]any{
				"inventory.items." + validRecordID + ".derivedTotal": 9.0,
				"name": "Aria",
			},
			want: map[string]any{"name": "Aria"},
		},
		{
			name: "Edge: an unknown Record field is left for the strict split",
			flat: map[string]any{"inventory.items." + validRecordID + ".ghost": 1.0},
			want: map[string]any{"inventory.items." + validRecordID + ".ghost": 1.0},
		},
		{
			name: "Edge: a malformed id and the list path are left untouched",
			flat: map[string]any{
				"inventory.items.rXYZ.quantity": 2.0,
				"inventory.items":               nil,
			},
			want: map[string]any{
				"inventory.items.rXYZ.quantity": 2.0,
				"inventory.items":               nil,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dag.DropDerivedRecordValues(tc.flat))
		})
	}
}

func TestValidateRecordID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "Happy path: lowercase r and 32 hex characters", id: validRecordID},
		{name: "Edge: an empty id", id: "", wantErr: true},
		{name: "Edge: a plain UUID with hyphens", id: "01234567-89ab-cdef-0123-456789abcdef", wantErr: true},
		{name: "Edge: an uppercase hex digit", id: "r0123456789ABCDEF0123456789abcdef", wantErr: true},
		{name: "Edge: a missing r prefix", id: "0123456789abcdef0123456789abcdef", wantErr: true},
		{name: "Edge: the wrong length", id: "r0123456789abcdef", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRecordID(tc.id)
			if tc.wantErr {
				var target *RecordIDError
				require.ErrorAs(t, err, &target)
				return
			}
			require.NoError(t, err)
		})
	}
}
