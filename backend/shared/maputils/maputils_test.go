package maputils

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:      "Happy path: Single level - exists",
			nested:    map[string]any{"a": 1.0},
			path:      []string{"a"},
			wantVal:   1.0,
			wantFound: true,
		},
		{
			name:      "Error: Single level - not exists",
			nested:    map[string]any{"a": 1.0},
			path:      []string{"b"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Happy path: Two levels - exists",
			nested:    map[string]any{"a": map[string]any{"b": 2.0}},
			path:      []string{"a", "b"},
			wantVal:   2.0,
			wantFound: true,
		},
		{
			name:      "Happy path: Three levels - exists",
			nested:    map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
			path:      []string{"a", "b", "c"},
			wantVal:   3.0,
			wantFound: true,
		},
		{
			name:      "Happy path: Deep nesting - 5 levels",
			nested:    map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": map[string]any{"e": 5.0}}}}},
			path:      []string{"a", "b", "c", "d", "e"},
			wantVal:   5.0,
			wantFound: true,
		},
		{
			name:      "Error: Intermediate not a map",
			nested:    map[string]any{"a": "not a map"},
			path:      []string{"a", "b"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Edge: Nil map",
			nested:    nil,
			path:      []string{"a"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Edge: Empty path",
			nested:    map[string]any{"a": 1.0},
			path:      []string{},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Edge: Path with empty segment",
			nested:    map[string]any{"a": map[string]any{"": 42.0}},
			path:      []string{"a", ""},
			wantVal:   42.0,
			wantFound: true,
		},
		{
			name:      "Error: Non-existent intermediate",
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

func TestGetFlat(t *testing.T) {
	tests := []struct {
		name      string
		nested    map[string]any
		key       string
		wantVal   any
		wantFound bool
	}{
		{
			name:      "Happy path: Flat key resolves nested value",
			nested:    map[string]any{"stats": map[string]any{"str": 10.0}},
			key:       "stats.str",
			wantVal:   10.0,
			wantFound: true,
		},
		{
			name:      "Happy path: Deep flat key resolves deeply nested value",
			nested:    map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
			key:       "a.b.c",
			wantVal:   3.0,
			wantFound: true,
		},
		{
			name:      "Happy path: Key without separator reads the root entry",
			nested:    map[string]any{"a": 1.0},
			key:       "a",
			wantVal:   1.0,
			wantFound: true,
		},
		{
			name:      "Edge: Nil map",
			nested:    nil,
			key:       "stats.str",
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Edge: Empty key",
			nested:    map[string]any{"stats": map[string]any{"str": 10.0}},
			key:       "",
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Error: Missing intermediate",
			nested:    map[string]any{"stats": map[string]any{"str": 10.0}},
			key:       "mod.str",
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Error: Intermediate is not a map",
			nested:    map[string]any{"stats": 5.0},
			key:       "stats.str",
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Error: Missing leaf",
			nested:    map[string]any{"stats": map[string]any{"str": 10.0}},
			key:       "stats.dex",
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Edge: Present but nil value counts as found",
			nested:    map[string]any{"stats": map[string]any{"str": nil}},
			key:       "stats.str",
			wantVal:   nil,
			wantFound: true,
		},
		{
			name:      "Happy path: Nested map leaf value",
			nested:    map[string]any{"stats": map[string]any{"sub": map[string]any{"a": 1.0}}},
			key:       "stats.sub.a",
			wantVal:   1.0,
			wantFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, found := GetFlat(tc.nested, tc.key)
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
			name:       "Happy path: Single level - new key",
			nested:     map[string]any{},
			path:       []string{"a"},
			value:      1.0,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Single level - overwrite",
			nested:     map[string]any{"a": 1.0},
			path:       []string{"a"},
			value:      2.0,
			wantNested: map[string]any{"a": 2.0},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Two levels - creates intermediate",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      2.0,
			wantNested: map[string]any{"a": map[string]any{"b": 2.0}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Three levels - creates all intermediates",
			nested:     map[string]any{},
			path:       []string{"a", "b", "c"},
			value:      3.0,
			wantNested: map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Existing intermediate map - adds to it",
			nested:     map[string]any{"a": map[string]any{"x": 1.0}},
			path:       []string{"a", "b"},
			value:      2.0,
			wantNested: map[string]any{"a": map[string]any{"x": 1.0, "b": 2.0}},
			wantErr:    nil,
		},
		{
			name:       "Error: Intermediate exists but not a map - returns error without mutating",
			nested:     map[string]any{"a": "not a map"},
			path:       []string{"a", "b"},
			value:      2.0,
			wantNested: map[string]any{"a": "not a map"},
			wantErr:    ErrNotMap,
		},
		{
			name:       "Error: Nil map - returns error",
			nested:     nil,
			path:       []string{"a"},
			value:      1.0,
			wantNested: nil,
			wantErr:    ErrNilMap,
		},
		{
			name:       "Error: Empty path - returns error",
			nested:     map[string]any{"a": 1.0},
			path:       []string{},
			value:      2.0,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    ErrEmptyPath,
		},
		{
			name:       "Edge: Path with empty segment",
			nested:     map[string]any{},
			path:       []string{"a", "", "b"},
			value:      42.0,
			wantNested: map[string]any{"a": map[string]any{"": map[string]any{"b": 42.0}}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: String value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      "hello",
			wantNested: map[string]any{"a": map[string]any{"b": "hello"}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Bool value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      true,
			wantNested: map[string]any{"a": map[string]any{"b": true}},
			wantErr:    nil,
		},
		{
			name:       "Edge: Nil value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      nil,
			wantNested: map[string]any{"a": map[string]any{"b": nil}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Slice value",
			nested:     map[string]any{},
			path:       []string{"a", "b"},
			value:      []int{1, 2, 3},
			wantNested: map[string]any{"a": map[string]any{"b": []int{1, 2, 3}}},
			wantErr:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputCopy := DeepCopy(tc.nested)
			err := Set(inputCopy, tc.path, tc.value)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
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
			name:       "Happy path: Single update",
			nested:     map[string]any{"stats": map[string]any{"str": 10.0}},
			updates:    map[string]any{"stats.str": 16.0},
			wantNested: map[string]any{"stats": map[string]any{"str": 16.0}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Multiple updates",
			nested:     map[string]any{"stats": map[string]any{"str": 10.0, "dex": 12.0}},
			updates:    map[string]any{"stats.str": 16.0, "stats.dex": 14.0},
			wantNested: map[string]any{"stats": map[string]any{"str": 16.0, "dex": 14.0}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Update creates new nested path",
			nested:     map[string]any{},
			updates:    map[string]any{"character.attributes.str": 15.0},
			wantNested: map[string]any{"character": map[string]any{"attributes": map[string]any{"str": 15.0}}},
			wantErr:    nil,
		},
		{
			name:       "Edge: Empty updates - no change",
			nested:     map[string]any{"a": 1.0},
			updates:    map[string]any{},
			wantNested: map[string]any{"a": 1.0},
			wantErr:    nil,
		},
		{
			name:       "Edge: Nil updates - no change",
			nested:     map[string]any{"a": 1.0},
			updates:    nil,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    nil,
		},
		{
			name:       "Edge: Nil nested - no change, no error",
			nested:     nil,
			updates:    map[string]any{"a.b": 1.0},
			wantNested: nil,
			wantErr:    nil,
		},
		{
			name:       "Happy path: Updates with string values",
			nested:     map[string]any{},
			updates:    map[string]any{"name": "test", "nested.value": "hello"},
			wantNested: map[string]any{"name": "test", "nested": map[string]any{"value": "hello"}},
			wantErr:    nil,
		},
		{
			name:       "Happy path: Keys with multiple dots",
			nested:     map[string]any{},
			updates:    map[string]any{"a.b.c.d": 1.0},
			wantNested: map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": 1.0}}}},
			wantErr:    nil,
		},
		{
			name:       "Error: Intermediate not a map - returns error",
			nested:     map[string]any{"a": "not a map"},
			updates:    map[string]any{"a.b": 1.0},
			wantNested: map[string]any{"a": "not a map"},
			wantErr:    ErrNotMap,
		},
		{
			name:       "Error: Empty key - returns ErrEmptyPath",
			nested:     map[string]any{"a": 1.0},
			updates:    map[string]any{"": 2.0},
			wantNested: map[string]any{"a": 1.0},
			wantErr:    ErrEmptyPath,
		},
		{
			name:       "Edge: Trailing dot key keeps empty segment",
			nested:     map[string]any{},
			updates:    map[string]any{"a.": 1.0},
			wantNested: map[string]any{"a": map[string]any{"": 1.0}},
			wantErr:    nil,
		},
		{
			name:       "Edge: Multibyte segments split on dots",
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
				require.NoError(t, err)
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
			name:     "Happy path: Simple flat",
			nested:   map[string]any{"a": 1.0, "b": 2.0},
			wantFlat: map[string]any{"a": 1.0, "b": 2.0},
		},
		{
			name:     "Happy path: Two levels",
			nested:   map[string]any{"stats": map[string]any{"str": 10.0, "dex": 12.0}},
			wantFlat: map[string]any{"stats_str": 10.0, "stats_dex": 12.0},
		},
		{
			name:     "Happy path: Three levels",
			nested:   map[string]any{"character": map[string]any{"attributes": map[string]any{"str": 15.0}}},
			wantFlat: map[string]any{"character_attributes_str": 15.0},
		},
		{
			name:     "Happy path: Mixed leaf and branch",
			nested:   map[string]any{"base": 5.0, "nested": map[string]any{"val": 10.0}},
			wantFlat: map[string]any{"base": 5.0, "nested_val": 10.0},
		},
		{
			name:     "Edge: Empty nested map - skipped",
			nested:   map[string]any{"empty": map[string]any{}, "val": 1.0},
			wantFlat: map[string]any{"empty": map[string]any{}, "val": 1.0},
		},
		{
			name:     "Edge: Nil nested",
			nested:   nil,
			wantFlat: map[string]any{},
		},
		{
			name:     "Edge: Empty nested",
			nested:   map[string]any{},
			wantFlat: map[string]any{},
		},
		{
			name:     "Happy path: String values",
			nested:   map[string]any{"name": "hero", "stats": map[string]any{"class": "warrior"}},
			wantFlat: map[string]any{"name": "hero", "stats_class": "warrior"},
		},
		{
			name:     "Happy path: Bool values",
			nested:   map[string]any{"active": true, "config": map[string]any{"enabled": false}},
			wantFlat: map[string]any{"active": true, "config_enabled": false},
		},
		{
			name:     "Happy path: Keys with underscores preserved",
			nested:   map[string]any{"stats_dex": 10.0, "stats": map[string]any{"str_dex": 12.0}},
			wantFlat: map[string]any{"stats_dex": 10.0, "stats_str_dex": 12.0},
		},
		{
			name:     "Edge: Keys with dots in name",
			nested:   map[string]any{"stats.str": 10.0},
			wantFlat: map[string]any{"stats.str": 10.0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ToEngineEnv(tc.nested)
			require.NoError(t, err)
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
			name:     "Happy path: Simple flat",
			nested:   map[string]any{"a": 1.0, "b": 2.0},
			wantFlat: map[string]any{"a": 1.0, "b": 2.0},
		},
		{
			name:     "Happy path: Two levels",
			nested:   map[string]any{"stats": map[string]any{"str": 10.0, "dex": 12.0}},
			wantFlat: map[string]any{"stats.str": 10.0, "stats.dex": 12.0},
		},
		{
			name:     "Happy path: Three levels",
			nested:   map[string]any{"character": map[string]any{"attributes": map[string]any{"str": 15.0}}},
			wantFlat: map[string]any{"character.attributes.str": 15.0},
		},
		{
			name:     "Happy path: Mixed leaf and branch",
			nested:   map[string]any{"base": 5.0, "nested": map[string]any{"val": 10.0}},
			wantFlat: map[string]any{"base": 5.0, "nested.val": 10.0},
		},
		{
			name:     "Edge: Empty nested map - treated as leaf",
			nested:   map[string]any{"empty": map[string]any{}, "val": 1.0},
			wantFlat: map[string]any{"empty": map[string]any{}, "val": 1.0},
		},
		{
			name:     "Edge: Nil nested",
			nested:   nil,
			wantFlat: map[string]any{},
		},
		{
			name:     "Edge: Empty nested",
			nested:   map[string]any{},
			wantFlat: map[string]any{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := FlattenPathKeys(tc.nested)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFlat, result)
		})
	}
}

func TestFlattenPathKeys_MaxDepth(t *testing.T) {
	nestedAt := func(depth int) map[string]any {
		root := map[string]any{}
		current := root
		for i := 1; i < depth; i++ {
			next := map[string]any{}
			current["level"] = next
			current = next
		}
		current["leaf"] = 1.0
		return root
	}

	t.Run("Edge: exactly MaxNestingDepth levels is accepted", func(t *testing.T) {
		_, err := FlattenPathKeys(nestedAt(MaxNestingDepth))
		assert.NoError(t, err)
	})

	t.Run("Error: one level deeper than MaxNestingDepth is rejected", func(t *testing.T) {
		_, err := FlattenPathKeys(nestedAt(MaxNestingDepth + 1))
		assert.ErrorIs(t, err, ErrMaxDepthExceeded)
	})

	t.Run("Error: ToEngineEnv applies the same bound", func(t *testing.T) {
		_, err := ToEngineEnv(nestedAt(MaxNestingDepth + 1))
		assert.ErrorIs(t, err, ErrMaxDepthExceeded)
	})
}

func TestDeepCopy(t *testing.T) {
	tests := []struct {
		name     string
		src      map[string]any
		wantCopy map[string]any
	}{
		{
			name:     "Happy path: Simple map",
			src:      map[string]any{"a": 1.0, "b": "hello"},
			wantCopy: map[string]any{"a": 1.0, "b": "hello"},
		},
		{
			name:     "Happy path: Nested map",
			src:      map[string]any{"a": map[string]any{"b": 2.0}},
			wantCopy: map[string]any{"a": map[string]any{"b": 2.0}},
		},
		{
			name:     "Happy path: Deep nested",
			src:      map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
			wantCopy: map[string]any{"a": map[string]any{"b": map[string]any{"c": 3.0}}},
		},
		{
			name:     "Happy path: Mixed values",
			src:      map[string]any{"num": 1.0, "str": "x", "bool": true, "nil": nil, "nested": map[string]any{"x": 1.0}},
			wantCopy: map[string]any{"num": 1.0, "str": "x", "bool": true, "nil": nil, "nested": map[string]any{"x": 1.0}},
		},
		{
			name:     "Edge: Empty map",
			src:      map[string]any{},
			wantCopy: map[string]any{},
		},
		{
			name:     "Edge: Nil map",
			src:      nil,
			wantCopy: nil,
		},
		{
			name:     "Happy path: Slice values copied by reference",
			src:      map[string]any{"arr": []int{1, 2, 3}},
			wantCopy: map[string]any{"arr": []int{1, 2, 3}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DeepCopy(tc.src)
			assert.Equal(t, tc.wantCopy, result)

			if tc.src != nil {
				for k, v := range tc.src {
					if nestedMap, ok := v.(map[string]any); ok {
						if resultMap, ok := result[k].(map[string]any); ok {
							originalLen := len(nestedMap)
							nestedMap["__test_mutation__"] = "mutated"
							assert.Len(t, resultMap, originalLen, "nested map should be independent copy")
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
			name:     "Happy path: float64 equal",
			a:        float64(10.0),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "Happy path: float64 not equal",
			a:        float64(10.0),
			b:        float64(20.0),
			expected: false,
		},
		{
			name:     "Happy path: float64 with fractional part",
			a:        float64(10.5),
			b:        float64(10.5),
			expected: true,
		},
		{
			name:     "Happy path: string equal",
			a:        "hello",
			b:        "hello",
			expected: true,
		},
		{
			name:     "Happy path: string not equal",
			a:        "hello",
			b:        "world",
			expected: false,
		},
		{
			name:     "Edge: empty string",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "Edge: empty string vs non-empty",
			a:        "",
			b:        "x",
			expected: false,
		},
		{
			name:     "Happy path: bool equal true",
			a:        true,
			b:        true,
			expected: true,
		},
		{
			name:     "Happy path: bool equal false",
			a:        false,
			b:        false,
			expected: true,
		},
		{
			name:     "Happy path: bool not equal",
			a:        true,
			b:        false,
			expected: false,
		},
		{
			name:     "Edge: both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "Edge: one nil",
			a:        nil,
			b:        float64(10),
			expected: false,
		},
		{
			name:     "Happy path: slice equal (DeepEqual fallback)",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3},
			expected: true,
		},
		{
			name:     "Happy path: slice not equal (DeepEqual fallback)",
			a:        []int{1, 2},
			b:        []int{1, 2, 3},
			expected: false,
		},
		{
			name:     "Happy path: map equal (DeepEqual fallback)",
			a:        map[string]any{"a": 1, "b": 2},
			b:        map[string]any{"a": 1, "b": 2},
			expected: true,
		},
		{
			name:     "Happy path: map not equal (DeepEqual fallback)",
			a:        map[string]any{"a": 1},
			b:        map[string]any{"a": 2},
			expected: false,
		},
		{
			name:     "Happy path: int vs float64 equal by value",
			a:        int(10),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "Happy path: int64 vs float64 equal by value",
			a:        int64(10),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "Happy path: float32 vs float64 equal by value",
			a:        float32(10),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "Happy path: int vs float64 not equal",
			a:        int(10),
			b:        float64(20.0),
			expected: false,
		},
		{
			name:     "Edge: float64 equal within epsilon",
			a:        float64(0.1 + 0.2),
			b:        float64(0.3),
			expected: true,
		},
		{
			name:     "Edge: float64 not equal beyond epsilon",
			a:        float64(1.0000001),
			b:        float64(1.0),
			expected: false,
		},
		{
			name:     "Edge: float64 differ within epsilon - equal",
			a:        float64(1.0),
			b:        float64(1.0 + epsilon/2),
			expected: true,
		},
		{
			name:     "Edge: float64 differ by more than epsilon - not equal",
			a:        float64(1.0),
			b:        float64(1.0 + 2*epsilon),
			expected: false,
		},
		{
			name:     "Edge: NaN vs NaN - not equal",
			a:        math.NaN(),
			b:        math.NaN(),
			expected: false,
		},
		{
			name:     "Edge: +Inf vs +Inf - equal",
			a:        math.Inf(1),
			b:        math.Inf(1),
			expected: true,
		},
		{
			name:     "Edge: -Inf vs -Inf - equal",
			a:        math.Inf(-1),
			b:        math.Inf(-1),
			expected: true,
		},
		{
			name:     "Edge: +Inf vs -Inf - not equal",
			a:        math.Inf(1),
			b:        math.Inf(-1),
			expected: false,
		},
		{
			name:     "Edge: uint64 max vs int64 -1 - not equal (no wrap-around)",
			a:        uint64(math.MaxUint64),
			b:        int64(-1),
			expected: false,
		},
		{
			name:     "Edge: int64 -1 vs uint64 max - not equal (symmetric)",
			a:        int64(-1),
			b:        uint64(math.MaxUint64),
			expected: false,
		},
		{
			name:     "Edge: uint64 max vs uint64 max - equal",
			a:        uint64(math.MaxUint64),
			b:        uint64(math.MaxUint64),
			expected: true,
		},
		{
			name:     "Edge: uint64 max vs uint64 max minus one - not equal",
			a:        uint64(math.MaxUint64),
			b:        uint64(math.MaxUint64 - 1),
			expected: false,
		},
		{
			name:     "Edge: uint64 1<<63 vs int64 min - not equal",
			a:        uint64(1) << 63,
			b:        int64(math.MinInt64),
			expected: false,
		},
		{
			name:     "Happy path: uint64 5 vs int64 -5 - not equal (sign matters)",
			a:        uint64(5),
			b:        int64(-5),
			expected: false,
		},
		{
			name:     "Happy path: uint64 5 vs int64 5 - equal",
			a:        uint64(5),
			b:        int64(5),
			expected: true,
		},
		{
			name:     "Edge: int64 min vs int64 min - equal",
			a:        int64(math.MinInt64),
			b:        int64(math.MinInt64),
			expected: true,
		},
		{
			name:     "Edge: int64 max vs int64 max minus one - not equal (exact)",
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
			name:     "Happy path: Simple path",
			input:    "a.b.c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Happy path: Single segment",
			input:    "a",
			expected: []string{"a"},
		},
		{
			name:     "Edge: Empty string yields empty path (rejected by Set as ErrEmptyPath)",
			input:    "",
			expected: nil,
		},
		{
			name:     "Edge: Leading dot",
			input:    ".a.b",
			expected: []string{"", "a", "b"},
		},
		{
			name:     "Edge: Trailing dot",
			input:    "a.b.",
			expected: []string{"a", "b", ""},
		},
		{
			name:     "Edge: Multiple consecutive dots",
			input:    "a..b",
			expected: []string{"a", "", "b"},
		},
		{
			name:     "Edge: Only dots",
			input:    "..",
			expected: []string{"", "", ""},
		},
		{
			name:     "Edge: Multibyte segments split on rune boundaries",
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

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{name: "Happy path: int", in: 3, want: 3, ok: true},
		{name: "Happy path: int8", in: int8(-4), want: -4, ok: true},
		{name: "Happy path: int16", in: int16(5), want: 5, ok: true},
		{name: "Happy path: int32", in: int32(6), want: 6, ok: true},
		{name: "Happy path: int64", in: int64(7), want: 7, ok: true},
		{name: "Happy path: uint", in: uint(8), want: 8, ok: true},
		{name: "Happy path: uint8", in: uint8(9), want: 9, ok: true},
		{name: "Happy path: uint16", in: uint16(10), want: 10, ok: true},
		{name: "Happy path: uint32", in: uint32(11), want: 11, ok: true},
		{name: "Happy path: uint64", in: uint64(12), want: 12, ok: true},
		{name: "Happy path: float32", in: float32(1.5), want: 1.5, ok: true},
		{name: "Happy path: float64", in: 2.25, want: 2.25, ok: true},
		{
			name: "Edge: uint64 max keeps its magnitude",
			in:   uint64(math.MaxUint64),
			want: float64(math.MaxUint64),
			ok:   true,
		},
		{
			name: "Edge: int64 min keeps its sign and magnitude",
			in:   int64(math.MinInt64),
			want: float64(math.MinInt64),
			ok:   true,
		},
		{name: "Error: string is not numeric", in: "1", want: 0, ok: false},
		{name: "Error: bool is not numeric", in: true, want: 0, ok: false},
		{name: "Error: nil is not numeric", in: nil, want: 0, ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ToFloat64(tc.in)
			assert.Equal(t, tc.ok, ok)
			assert.InDelta(t, tc.want, got, 1e-9)
		})
	}
}
