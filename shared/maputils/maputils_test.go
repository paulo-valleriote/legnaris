package maputils

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name      string
		nested    map[string]any
		path      []string
		wantVal   any
		wantFound bool
	}{
		{
			name:      "Single level - exists",
			nested:    map[string]any{"a": 1.0},
			path:      []string{"a"},
			wantVal:   1.0,
			wantFound: true,
		},
		{
			name:      "Single level - not exists",
			nested:    map[string]any{"a": 1.0},
			path:      []string{"b"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Two levels - exists",
			nested:    map[string]any{"a": map[string]any{"b": 2.0}},
			path:      []string{"a", "b"},
			wantVal:   2.0,
			wantFound: true,
		},
		{
			name:      "Three levels - exists",
			nested:    map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
			path:      []string{"a", "b", "c"},
			wantVal:   3.0,
			wantFound: true,
		},
		{
			name:      "Deep nesting - 5 levels",
			nested:    map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": map[string]any{"e": 5.0}}}}},
			path:      []string{"a", "b", "c", "d", "e"},
			wantVal:   5.0,
			wantFound: true,
		},
		{
			name:      "Intermediate not a map",
			nested:    map[string]any{"a": "not a map"},
			path:      []string{"a", "b"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Nil map",
			nested:    nil,
			path:      []string{"a"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Empty path",
			nested:    map[string]any{"a": 1.0},
			path:      []string{},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Path with empty segment",
			nested:    map[string]any{"a": map[string]any{"": 42.0}},
			path:      []string{"a", ""},
			wantVal:   42.0,
			wantFound: true,
		},
		{
			name:      "Non-existent intermediate",
			nested:    map[string]any{"a": map[string]any{"b": 1.0}},
			path:      []string{"a", "c", "d"},
			wantVal:   nil,
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, found := Get(tc.nested, tc.path)
			assert.Equal(t, tc.wantFound, found, "found mismatch")
			if tc.wantFound {
				assert.Equal(t, tc.wantVal, val, "value mismatch")
			}
		})
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		name       string
		nested     map[string]any
		path       []string
		value      any
		wantNested map[string]any
		wantErr    error
	}{
		{
			name:       "Single level - new key",
			nested:     map[string]any{},
			path:       []string{"a"},
			value:      1.0,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    nil,
		},
		{
			name:       "Single level - overwrite",
			nested:     map[string]any{"a": 1.0},
			path:       []string{"a"},
			value:      2.0,
			wantNested: map[string]any{"a": 2.0},
			wantErr:    nil,
		},
		{
			name:       "Two levels - creates intermediate",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      2.0,
			wantNested: map[string]any{"a": map[string]any{"b": 2.0}},
			wantErr:    nil,
		},
		{
			name:       "Three levels - creates all intermediates",
			nested:     map[string]any{},
			path:       []string{"a", "b", "c"},
			value:      3.0,
			wantNested: map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
			wantErr:    nil,
		},
		{
			name:       "Existing intermediate map - adds to it",
			nested:     map[string]any{"a": map[string]any{"x": 1.0}},
			path:       []string{"a", "b"},
			value:      2.0,
			wantNested: map[string]any{"a": map[string]any{"x": 1.0, "b": 2.0}},
			wantErr:    nil,
		},
		{
			name:       "Intermediate exists but not a map - returns error without mutating",
			nested:     map[string]any{"a": "not a map"},
			path:       []string{"a", "b"},
			value:      2.0,
			wantNested: map[string]any{"a": "not a map"},
			wantErr:    ErrNotMap,
		},
		{
			name:       "Nil map - returns error",
			nested:     nil,
			path:       []string{"a"},
			value:      1.0,
			wantNested: nil,
			wantErr:    ErrNilMap,
		},
		{
			name:       "Empty path - returns error",
			nested:     map[string]any{"a": 1.0},
			path:       []string{},
			value:      2.0,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    ErrEmptyPath,
		},
		{
			name:       "Path with empty segment",
			nested:     map[string]any{},
			path:       []string{"a", "", "b"},
			value:      42.0,
			wantNested: map[string]any{"a": map[string]any{"": map[string]any{"b": 42.0}}},
			wantErr:    nil,
		},
		{
			name:       "String value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      "hello",
			wantNested: map[string]any{"a": map[string]any{"b": "hello"}},
			wantErr:    nil,
		},
		{
			name:       "Bool value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      true,
			wantNested: map[string]any{"a": map[string]any{"b": true}},
			wantErr:    nil,
		},
		{
			name:       "Nil value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      nil,
			wantNested: map[string]any{"a": map[string]any{"b": nil}},
			wantErr:    nil,
		},
		{
			name:       "Slice value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      []int{1, 2, 3},
			wantNested: map[string]any{"a": map[string]any{"b": []int{1, 2, 3}}},
			wantErr:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Deep copy input for comparison
			inputCopy := DeepCopy(tc.nested)
			err := Set(inputCopy, tc.path, tc.value)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantNested, inputCopy)
			}
		})
	}
}

func TestApplyUpdates(t *testing.T) {
	tests := []struct {
		name       string
		nested     map[string]any
		updates    map[string]any
		wantNested map[string]any
		wantErr    error
	}{
		{
			name:       "Single update",
			nested:     map[string]any{"stats": map[string]any{"str": 10.0}},
			updates:    map[string]any{"stats.str": 16.0},
			wantNested: map[string]any{"stats": map[string]any{"str": 16.0}},
			wantErr:    nil,
		},
		{
			name:       "Multiple updates",
			nested:     map[string]any{"stats": map[string]any{"str": 10.0, "dex": 12.0}},
			updates:    map[string]any{"stats.str": 16.0, "stats.dex": 14.0},
			wantNested: map[string]any{"stats": map[string]any{"str": 16.0, "dex": 14.0}},
			wantErr:    nil,
		},
		{
			name:       "Update creates new nested path",
			nested:     map[string]any{},
			updates:    map[string]any{"character.attributes.str": 15.0},
			wantNested: map[string]any{"character": map[string]any{"attributes": map[string]any{"str": 15.0}}},
			wantErr:    nil,
		},
		{
			name:       "Empty updates - no change",
			nested:     map[string]any{"a": 1.0},
			updates:    map[string]any{},
			wantNested: map[string]any{"a": 1.0},
			wantErr:    nil,
		},
		{
			name:       "Nil updates - no change",
			nested:     map[string]any{"a": 1.0},
			updates:    nil,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    nil,
		},
		{
			name:       "Nil nested - no change, no error",
			nested:     nil,
			updates:    map[string]any{"a.b": 1.0},
			wantNested: nil,
			wantErr:    nil,
		},
		{
			name:       "Updates with string values",
			nested:     map[string]any{},
			updates:    map[string]any{"name": "test", "nested.value": "hello"},
			wantNested: map[string]any{"name": "test", "nested": map[string]any{"value": "hello"}},
			wantErr:    nil,
		},
		{
			name:       "Keys with multiple dots",
			nested:     map[string]any{},
			updates:    map[string]any{"a.b.c.d": 1.0},
			wantNested: map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": 1.0}}}},
			wantErr:    nil,
		},
		{
			name:       "Intermediate not a map - returns error",
			nested:     map[string]any{"a": "not a map"},
			updates:    map[string]any{"a.b": 1.0},
			wantNested: map[string]any{"a": "not a map"},
			wantErr:    ErrNotMap,
		},
		{
			// Parity with the former sheet/service splitPathKey: an empty key
			// must error instead of silently dropping the update.
			name:       "Empty key - returns ErrEmptyPath",
			nested:     map[string]any{"a": 1.0},
			updates:    map[string]any{"": 2.0},
			wantNested: map[string]any{"a": 1.0},
			wantErr:    ErrEmptyPath,
		},
		{
			// Trailing separators keep their empty segment, matching splitPath.
			name:       "Trailing dot key keeps empty segment",
			nested:     map[string]any{},
			updates:    map[string]any{"a.": 1.0},
			wantNested: map[string]any{"a": map[string]any{"": 1.0}},
			wantErr:    nil,
		},
		{
			// Parity with the former sheet/service splitPathKey on multibyte
			// keys: splitting must not break rune sequences.
			name:       "Multibyte segments split on dots",
			nested:     map[string]any{},
			updates:    map[string]any{"action.mod": 2.0},
			wantNested: map[string]any{"action": map[string]any{"mod": 2.0}},
			wantErr:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputCopy := DeepCopy(tc.nested)
			err := ApplyUpdates(inputCopy, tc.updates)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantNested, inputCopy)
			}
		})
	}
}

func TestToEngineEnv(t *testing.T) {
	tests := []struct {
		name     string
		nested   map[string]any
		wantFlat map[string]any
	}{
		{
			name:     "Simple flat",
			nested:   map[string]any{"a": 1.0, "b": 2.0},
			wantFlat: map[string]any{"a": 1.0, "b": 2.0},
		},
		{
			name:     "Two levels",
			nested:   map[string]any{"stats": map[string]any{"str": 10.0, "dex": 12.0}},
			wantFlat: map[string]any{"stats_str": 10.0, "stats_dex": 12.0},
		},
		{
			name:     "Three levels",
			nested:   map[string]any{"character": map[string]any{"attributes": map[string]any{"str": 15.0}}},
			wantFlat: map[string]any{"character_attributes_str": 15.0},
		},
		{
			name:     "Mixed leaf and branch",
			nested:   map[string]any{"base": 5.0, "nested": map[string]any{"val": 10.0}},
			wantFlat: map[string]any{"base": 5.0, "nested_val": 10.0},
		},
		{
			name:     "Empty nested map - skipped",
			nested:   map[string]any{"empty": map[string]any{}, "val": 1.0},
			wantFlat: map[string]any{"empty": map[string]any{}, "val": 1.0},
		},
		{
			name:     "Nil nested",
			nested:   nil,
			wantFlat: map[string]any{},
		},
		{
			name:     "Empty nested",
			nested:   map[string]any{},
			wantFlat: map[string]any{},
		},
		{
			name:     "String values",
			nested:   map[string]any{"name": "hero", "stats": map[string]any{"class": "warrior"}},
			wantFlat: map[string]any{"name": "hero", "stats_class": "warrior"},
		},
		{
			name:     "Bool values",
			nested:   map[string]any{"active": true, "config": map[string]any{"enabled": false}},
			wantFlat: map[string]any{"active": true, "config_enabled": false},
		},
		{
			name:     "Keys with underscores preserved",
			nested:   map[string]any{"stats_dex": 10.0, "stats": map[string]any{"str_dex": 12.0}},
			wantFlat: map[string]any{"stats_dex": 10.0, "stats_str_dex": 12.0},
		},
		{
			name:     "Keys with dots in name",
			nested:   map[string]any{"stats.str": 10.0},
			wantFlat: map[string]any{"stats.str": 10.0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ToEngineEnv(tc.nested)
			assert.Equal(t, tc.wantFlat, result)
		})
	}
}

func TestFlattenPathKeys(t *testing.T) {
	tests := []struct {
		name     string
		nested   map[string]any
		wantFlat map[string]any
	}{
		{
			name:     "Simple flat",
			nested:   map[string]any{"a": 1.0, "b": 2.0},
			wantFlat: map[string]any{"a": 1.0, "b": 2.0},
		},
		{
			name:     "Two levels",
			nested:   map[string]any{"stats": map[string]any{"str": 10.0, "dex": 12.0}},
			wantFlat: map[string]any{"stats.str": 10.0, "stats.dex": 12.0},
		},
		{
			name:     "Three levels",
			nested:   map[string]any{"character": map[string]any{"attributes": map[string]any{"str": 15.0}}},
			wantFlat: map[string]any{"character.attributes.str": 15.0},
		},
		{
			name:     "Mixed leaf and branch",
			nested:   map[string]any{"base": 5.0, "nested": map[string]any{"val": 10.0}},
			wantFlat: map[string]any{"base": 5.0, "nested.val": 10.0},
		},
		{
			name:     "Empty nested map - treated as leaf",
			nested:   map[string]any{"empty": map[string]any{}, "val": 1.0},
			wantFlat: map[string]any{"empty": map[string]any{}, "val": 1.0},
		},
		{
			name:     "Nil nested",
			nested:   nil,
			wantFlat: map[string]any{},
		},
		{
			name:     "Empty nested",
			nested:   map[string]any{},
			wantFlat: map[string]any{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FlattenPathKeys(tc.nested)
			assert.Equal(t, tc.wantFlat, result)
		})
	}
}

func TestDeepCopy(t *testing.T) {
	tests := []struct {
		name     string
		src      map[string]any
		wantCopy map[string]any
	}{
		{
			name:     "Simple map",
			src:      map[string]any{"a": 1.0, "b": "hello"},
			wantCopy: map[string]any{"a": 1.0, "b": "hello"},
		},
		{
			name:     "Nested map",
			src:      map[string]any{"a": map[string]any{"b": 2.0}},
			wantCopy: map[string]any{"a": map[string]any{"b": 2.0}},
		},
		{
			name:     "Deep nested",
			src:      map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
			wantCopy: map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
		},
		{
			name:     "Mixed values",
			src:      map[string]any{"num": 1.0, "str": "x", "bool": true, "nil": nil, "nested": map[string]any{"x": 1.0}},
			wantCopy: map[string]any{"num": 1.0, "str": "x", "bool": true, "nil": nil, "nested": map[string]any{"x": 1.0}},
		},
		{
			name:     "Empty map",
			src:      map[string]any{},
			wantCopy: map[string]any{},
		},
		{
			name:     "Nil map",
			src:      nil,
			wantCopy: nil,
		},
		{
			name:     "Slice values copied by reference",
			src:      map[string]any{"arr": []int{1, 2, 3}},
			wantCopy: map[string]any{"arr": []int{1, 2, 3}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DeepCopy(tc.src)
			assert.Equal(t, tc.wantCopy, result)

			// Verify deep copy independence for nested maps
			if tc.src != nil {
				for k, v := range tc.src {
					if nestedMap, ok := v.(map[string]any); ok {
						if resultMap, ok := result[k].(map[string]any); ok {
							// Verify they are different map instances by modifying one
							// and checking the other is unaffected
							originalLen := len(nestedMap)
							nestedMap["__test_mutation__"] = "mutated"
							assert.Len(t, resultMap, originalLen, "nested map should be independent copy")
							// Clean up
							delete(nestedMap, "__test_mutation__")
						}
					}
				}
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        any
		b        any
		expected bool
	}{
		{
			name:     "float64 equal",
			a:        float64(10.0),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "float64 not equal",
			a:        float64(10.0),
			b:        float64(20.0),
			expected: false,
		},
		{
			name:     "float64 with fractional part",
			a:        float64(10.5),
			b:        float64(10.5),
			expected: true,
		},
		{
			name:     "string equal",
			a:        "hello",
			b:        "hello",
			expected: true,
		},
		{
			name:     "string not equal",
			a:        "hello",
			b:        "world",
			expected: false,
		},
		{
			name:     "empty string",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "empty string vs non-empty",
			a:        "",
			b:        "x",
			expected: false,
		},
		{
			name:     "bool equal true",
			a:        true,
			b:        true,
			expected: true,
		},
		{
			name:     "bool equal false",
			a:        false,
			b:        false,
			expected: true,
		},
		{
			name:     "bool not equal",
			a:        true,
			b:        false,
			expected: false,
		},
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil",
			a:        nil,
			b:        float64(10),
			expected: false,
		},
		{
			name:     "slice equal (DeepEqual fallback)",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3},
			expected: true,
		},
		{
			name:     "slice not equal (DeepEqual fallback)",
			a:        []int{1, 2},
			b:        []int{1, 2, 3},
			expected: false,
		},
		{
			name:     "map equal (DeepEqual fallback)",
			a:        map[string]any{"a": 1, "b": 2},
			b:        map[string]any{"a": 1, "b": 2},
			expected: true,
		},
		{
			name:     "map not equal (DeepEqual fallback)",
			a:        map[string]any{"a": 1},
			b:        map[string]any{"a": 2},
			expected: false,
		},
		{
			name:     "int vs float64 equal by value",
			a:        int(10),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "int64 vs float64 equal by value",
			a:        int64(10),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "float32 vs float64 equal by value",
			a:        float32(10),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "int vs float64 not equal",
			a:        int(10),
			b:        float64(20.0),
			expected: false,
		},
		{
			name:     "float64 equal within epsilon",
			a:        float64(0.1 + 0.2),
			b:        float64(0.3),
			expected: true,
		},
		{
			name:     "float64 not equal beyond epsilon",
			a:        float64(1.0000001),
			b:        float64(1.0),
			expected: false,
		},
		{
			name:     "float64 differ within epsilon - equal",
			a:        float64(1.0),
			b:        float64(1.0 + epsilon/2),
			expected: true,
		},
		{
			name:     "float64 differ by more than epsilon - not equal",
			a:        float64(1.0),
			b:        float64(1.0 + 2*epsilon),
			expected: false,
		},
		{
			name:     "NaN vs NaN - not equal",
			a:        math.NaN(),
			b:        math.NaN(),
			expected: false,
		},
		{
			name:     "+Inf vs +Inf - equal",
			a:        math.Inf(1),
			b:        math.Inf(1),
			expected: true,
		},
		{
			name:     "-Inf vs -Inf - equal",
			a:        math.Inf(-1),
			b:        math.Inf(-1),
			expected: true,
		},
		{
			name:     "+Inf vs -Inf - not equal",
			a:        math.Inf(1),
			b:        math.Inf(-1),
			expected: false,
		},
		{
			name:     "uint64 max vs int64 -1 - not equal (no wrap-around)",
			a:        uint64(math.MaxUint64),
			b:        int64(-1),
			expected: false,
		},
		{
			name:     "int64 -1 vs uint64 max - not equal (symmetric)",
			a:        int64(-1),
			b:        uint64(math.MaxUint64),
			expected: false,
		},
		{
			name:     "uint64 max vs uint64 max - equal",
			a:        uint64(math.MaxUint64),
			b:        uint64(math.MaxUint64),
			expected: true,
		},
		{
			name:     "uint64 max vs uint64 max minus one - not equal",
			a:        uint64(math.MaxUint64),
			b:        uint64(math.MaxUint64 - 1),
			expected: false,
		},
		{
			name:     "uint64 1<<63 vs int64 min - not equal",
			a:        uint64(1) << 63,
			b:        int64(math.MinInt64),
			expected: false,
		},
		{
			name:     "uint64 5 vs int64 -5 - not equal (sign matters)",
			a:        uint64(5),
			b:        int64(-5),
			expected: false,
		},
		{
			name:     "uint64 5 vs int64 5 - equal",
			a:        uint64(5),
			b:        int64(5),
			expected: true,
		},
		{
			name:     "int64 min vs int64 min - equal",
			a:        int64(math.MinInt64),
			b:        int64(math.MinInt64),
			expected: true,
		},
		{
			name:     "int64 max vs int64 max minus one - not equal (exact)",
			a:        int64(math.MaxInt64),
			b:        int64(math.MaxInt64 - 1),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Equal(tc.a, tc.b)
			assert.Equal(t, tc.expected, result, "Equal(%v, %v)", tc.a, tc.b)
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple path",
			input:    "a.b.c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Single segment",
			input:    "a",
			expected: []string{"a"},
		},
		{
			name:     "Empty string yields empty path (rejected by Set as ErrEmptyPath)",
			input:    "",
			expected: nil,
		},
		{
			name:     "Leading dot",
			input:    ".a.b",
			expected: []string{"", "a", "b"},
		},
		{
			name:     "Trailing dot",
			input:    "a.b.",
			expected: []string{"a", "b", ""},
		},
		{
			name:     "Multiple consecutive dots",
			input:    "a..b",
			expected: []string{"a", "", "b"},
		},
		{
			name:     "Only dots",
			input:    "..",
			expected: []string{"", "", ""},
		},
		{
			name:     "Multibyte segments split on rune boundaries",
			input:    "action.mod",
			expected: []string{"action", "mod"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := splitPath(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
