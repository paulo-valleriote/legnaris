package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"legnaris/shared/maputils"
)

func TestPath_String(t *testing.T) {
	tests := []struct {
		name     string
		path     Path
		expected string
	}{
		{
			name:     "Empty path",
			path:     Path{},
			expected: "",
		},
		{
			name:     "Single segment",
			path:     Path{"stats"},
			expected: "stats",
		},
		{
			name:     "Two segments",
			path:     Path{"stats", "str"},
			expected: "stats.str",
		},
		{
			name:     "Three segments",
			path:     Path{"character", "attributes", "strength"},
			expected: "character.attributes.strength",
		},
		{
			name:     "With empty segment",
			path:     Path{"a", "", "b"},
			expected: "a..b",
		},
		{
			name:     "With dot in segment",
			path:     Path{"stats.str", "dex"},
			expected: "stats.str.dex",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.path.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPath_EngineKey(t *testing.T) {
	tests := []struct {
		name     string
		path     Path
		expected string
	}{
		{
			name:     "Empty path",
			path:     Path{},
			expected: "",
		},
		{
			name:     "Single segment",
			path:     Path{"stats"},
			expected: "stats",
		},
		{
			name:     "Two segments",
			path:     Path{"stats", "str"},
			expected: "stats_str",
		},
		{
			name:     "Three segments",
			path:     Path{"character", "attributes", "strength"},
			expected: "character_attributes_strength",
		},
		{
			name:     "With empty segment",
			path:     Path{"a", "", "b"},
			expected: "a__b",
		},
		{
			name:     "With underscore in segment",
			path:     Path{"stats_str", "dex"},
			expected: "stats_str_dex",
		},
		{
			name:     "With dot in segment",
			path:     Path{"stats.str", "dex"},
			expected: "stats.str_dex",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.path.EngineKey()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPath_Parent(t *testing.T) {
	tests := []struct {
		name     string
		path     Path
		expected Path
	}{
		{
			name:     "Empty path",
			path:     Path{},
			expected: Path{},
		},
		{
			name:     "Single segment (root)",
			path:     Path{"stats"},
			expected: Path{},
		},
		{
			name:     "Two segments",
			path:     Path{"stats", "str"},
			expected: Path{"stats"},
		},
		{
			name:     "Three segments",
			path:     Path{"character", "attributes", "strength"},
			expected: Path{"character", "attributes"},
		},
		{
			name:     "With empty segment",
			path:     Path{"a", "", "b"},
			expected: Path{"a", ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.path.Parent()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPath_Last(t *testing.T) {
	tests := []struct {
		name     string
		path     Path
		expected string
	}{
		{
			name:     "Empty path",
			path:     Path{},
			expected: "",
		},
		{
			name:     "Single segment",
			path:     Path{"stats"},
			expected: "stats",
		},
		{
			name:     "Two segments",
			path:     Path{"stats", "str"},
			expected: "str",
		},
		{
			name:     "With empty segment",
			path:     Path{"a", "", "b"},
			expected: "b",
		},
		{
			name:     "Trailing empty segment",
			path:     Path{"a", "b", ""},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.path.Last()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPath_Get(t *testing.T) {
	tests := []struct {
		name      string
		nested    map[string]any
		path      Path
		wantVal   any
		wantFound bool
	}{
		{
			name:      "Exists at root",
			nested:    map[string]any{"stats": 10.0},
			path:      Path{"stats"},
			wantVal:   10.0,
			wantFound: true,
		},
		{
			name:      "Exists nested",
			nested:    map[string]any{"stats": map[string]any{"str": 16.0}},
			path:      Path{"stats", "str"},
			wantVal:   16.0,
			wantFound: true,
		},
		{
			name:      "Not found",
			nested:    map[string]any{"stats": map[string]any{"str": 16.0}},
			path:      Path{"stats", "dex"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Nil map",
			nested:    nil,
			path:      Path{"stats"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Empty path",
			nested:    map[string]any{"stats": 10.0},
			path:      Path{},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Intermediate not a map",
			nested:    map[string]any{"stats": "not a map"},
			path:      Path{"stats", "str"},
			wantVal:   nil,
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, found := tc.path.Get(tc.nested)
			assert.Equal(t, tc.wantFound, found)
			if tc.wantFound {
				assert.Equal(t, tc.wantVal, val)
			}
		})
	}
}

func TestPath_Set(t *testing.T) {
	tests := []struct {
		name       string
		nested     map[string]any
		path       Path
		value      any
		wantNested map[string]any
		wantErr    error
	}{
		{
			name:       "Single level",
			nested:     map[string]any{},
			path:       Path{"stats"},
			value:      10.0,
			wantNested: map[string]any{"stats": 10.0},
		},
		{
			name:       "Two levels - creates intermediate",
			nested:     map[string]any{},
			path:       Path{"stats", "str"},
			value:      16.0,
			wantNested: map[string]any{"stats": map[string]any{"str": 16.0}},
		},
		{
			name:       "Three levels - creates all intermediates",
			nested:     map[string]any{},
			path:       Path{"character", "attributes", "strength"},
			value:      18.0,
			wantNested: map[string]any{"character": map[string]any{"attributes": map[string]any{"strength": 18.0}}},
		},
		{
			name:       "Existing intermediate - adds to it",
			nested:     map[string]any{"stats": map[string]any{"str": 10.0}},
			path:       Path{"stats", "dex"},
			value:      14.0,
			wantNested: map[string]any{"stats": map[string]any{"str": 10.0, "dex": 14.0}},
		},
		{
			name:       "Overwrites non-map intermediate",
			nested:     map[string]any{"stats": "not a map"},
			path:       Path{"stats", "str"},
			value:      16.0,
			wantNested: nil,
			wantErr:    maputils.ErrNotMap,
		},
		{
			name:       "Nil map - returns error",
			nested:     nil,
			path:       Path{"stats"},
			value:      10.0,
			wantNested: nil,
			wantErr:    maputils.ErrNilMap,
		},
		{
			name:       "Empty path - returns error",
			nested:     map[string]any{"a": 1.0},
			path:       Path{},
			value:      2.0,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    maputils.ErrEmptyPath,
		},
		{
			name:       "String value",
			nested:     map[string]any{},
			path:       Path{"name"},
			value:      "hero",
			wantNested: map[string]any{"name": "hero"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputCopy := DeepCopyForTest(tc.nested)
			err := tc.path.Set(inputCopy, tc.value)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantNested, inputCopy)
			}
		})
	}
}

func TestPath_Flatten(t *testing.T) {
	tests := []struct {
		name     string
		nested   map[string]any
		path     Path
		wantFlat map[string]any
	}{
		{
			name:     "Empty path - flattens entire map",
			nested:   map[string]any{"stats": map[string]any{"str": 10.0}, "hp": 100.0},
			path:     Path{},
			wantFlat: map[string]any{"stats_str": 10.0, "hp": 100.0},
		},
		{
			name:     "Path to nested object",
			nested:   map[string]any{"character": map[string]any{"attributes": map[string]any{"str": 15.0, "dex": 12.0}}},
			path:     Path{"character", "attributes"},
			wantFlat: map[string]any{"character_attributes_str": 15.0, "character_attributes_dex": 12.0},
		},
		{
			name:     "Path to leaf - returns empty",
			nested:   map[string]any{"stats": map[string]any{"str": 10.0}},
			path:     Path{"stats", "str"},
			wantFlat: map[string]any{},
		},
		{
			name:     "Path not found - returns empty",
			nested:   map[string]any{"stats": map[string]any{"str": 10.0}},
			path:     Path{"attributes", "str"},
			wantFlat: map[string]any{},
		},
		{
			name:     "Path points to non-map - returns empty",
			nested:   map[string]any{"stats": "not a map"},
			path:     Path{"stats"},
			wantFlat: map[string]any{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.path.Flatten(tc.nested)
			assert.Equal(t, tc.wantFlat, result)
		})
	}
}

func TestPath_IsRoot(t *testing.T) {
	tests := []struct {
		name     string
		path     Path
		expected bool
	}{
		{
			name:     "Empty path",
			path:     Path{},
			expected: false,
		},
		{
			name:     "Single segment",
			path:     Path{"stats"},
			expected: true,
		},
		{
			name:     "Two segments",
			path:     Path{"stats", "str"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.path.IsRoot()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPath_Segments(t *testing.T) {
	tests := []struct {
		name     string
		path     Path
		expected []string
	}{
		{
			name:     "Empty path",
			path:     Path{},
			expected: []string{},
		},
		{
			name:     "Single segment",
			path:     Path{"stats"},
			expected: []string{"stats"},
		},
		{
			name:     "Multiple segments",
			path:     Path{"stats", "str"},
			expected: []string{"stats", "str"},
		},
		{
			name:     "Nil path",
			path:     nil,
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.path.Segments()
			assert.Equal(t, tc.expected, result)
			// Verify it's a copy
			if tc.path != nil && len(tc.path) > 0 {
				result[0] = "mutated"
				assert.NotEqual(t, "mutated", tc.path[0], "Segments should return a copy")
			}
		})
	}
}

func TestPath_Equal(t *testing.T) {
	tests := []struct {
		name     string
		a        Path
		b        Path
		expected bool
	}{
		{
			name:     "Both empty",
			a:        Path{},
			b:        Path{},
			expected: true,
		},
		{
			name:     "Both single, same",
			a:        Path{"stats"},
			b:        Path{"stats"},
			expected: true,
		},
		{
			name:     "Both single, different",
			a:        Path{"stats"},
			b:        Path{"dex"},
			expected: false,
		},
		{
			name:     "Both multi, same",
			a:        Path{"stats", "str"},
			b:        Path{"stats", "str"},
			expected: true,
		},
		{
			name:     "Both multi, different last",
			a:        Path{"stats", "str"},
			b:        Path{"stats", "dex"},
			expected: false,
		},
		{
			name:     "Different lengths",
			a:        Path{"stats"},
			b:        Path{"stats", "str"},
			expected: false,
		},
		{
			name:     "Nil vs empty",
			a:        nil,
			b:        Path{},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.a.Equal(tc.b)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// DeepCopyForTest is a test helper that mimics maputils.DeepCopy
// without importing maputils (to avoid import cycle in tests).
func DeepCopyForTest(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if nestedMap, ok := v.(map[string]any); ok {
			dst[k] = DeepCopyForTest(nestedMap)
		} else {
			dst[k] = v
		}
	}
	return dst
}
