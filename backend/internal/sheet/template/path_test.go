package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"legnaris/backend/shared/maputils"
)

func TestPath_String(t *testing.T) {
	tests := []struct {
		name     string
		path     Path
		expected string
	}{
		{
			name:     "Edge: Empty path",
			path:     Path{},
			expected: "",
		},
		{
			name:     "Happy path: Single segment",
			path:     Path{"stats"},
			expected: "stats",
		},
		{
			name:     "Happy path: Two segments",
			path:     Path{"stats", "str"},
			expected: "stats.str",
		},
		{
			name:     "Happy path: Three segments",
			path:     Path{"character", "attributes", "strength"},
			expected: "character.attributes.strength",
		},
		{
			name:     "Edge: With empty segment",
			path:     Path{"a", "", "b"},
			expected: "a..b",
		},
		{
			name:     "Edge: With dot in segment",
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
			name:     "Edge: Empty path",
			path:     Path{},
			expected: "",
		},
		{
			name:     "Happy path: Single segment",
			path:     Path{"stats"},
			expected: "stats",
		},
		{
			name:     "Happy path: Two segments",
			path:     Path{"stats", "str"},
			expected: "stats_str",
		},
		{
			name:     "Happy path: Three segments",
			path:     Path{"character", "attributes", "strength"},
			expected: "character_attributes_strength",
		},
		{
			name:     "Edge: With empty segment",
			path:     Path{"a", "", "b"},
			expected: "a__b",
		},
		{
			name:     "Edge: With underscore in segment",
			path:     Path{"stats_str", "dex"},
			expected: "stats_str_dex",
		},
		{
			name:     "Edge: With dot in segment",
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

func TestPath_Get(t *testing.T) {
	tests := []struct {
		name      string
		nested    map[string]any
		path      Path
		wantVal   any
		wantFound bool
	}{
		{
			name:      "Happy path: Exists at root",
			nested:    map[string]any{"stats": 10.0},
			path:      Path{"stats"},
			wantVal:   10.0,
			wantFound: true,
		},
		{
			name:      "Happy path: Exists nested",
			nested:    map[string]any{"stats": map[string]any{"str": 16.0}},
			path:      Path{"stats", "str"},
			wantVal:   16.0,
			wantFound: true,
		},
		{
			name:      "Error: Not found",
			nested:    map[string]any{"stats": map[string]any{"str": 16.0}},
			path:      Path{"stats", "dex"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Edge: Nil map",
			nested:    nil,
			path:      Path{"stats"},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Edge: Empty path",
			nested:    map[string]any{"stats": 10.0},
			path:      Path{},
			wantVal:   nil,
			wantFound: false,
		},
		{
			name:      "Error: Intermediate not a map",
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
			name:       "Happy path: Single level",
			nested:     map[string]any{},
			path:       Path{"stats"},
			value:      10.0,
			wantNested: map[string]any{"stats": 10.0},
		},
		{
			name:       "Happy path: Two levels - creates intermediate",
			nested:     map[string]any{},
			path:       Path{"stats", "str"},
			value:      16.0,
			wantNested: map[string]any{"stats": map[string]any{"str": 16.0}},
		},
		{
			name:       "Happy path: Three levels - creates all intermediates",
			nested:     map[string]any{},
			path:       Path{"character", "attributes", "strength"},
			value:      18.0,
			wantNested: map[string]any{"character": map[string]any{"attributes": map[string]any{"strength": 18.0}}},
		},
		{
			name:       "Happy path: Existing intermediate - adds to it",
			nested:     map[string]any{"stats": map[string]any{"str": 10.0}},
			path:       Path{"stats", "dex"},
			value:      14.0,
			wantNested: map[string]any{"stats": map[string]any{"str": 10.0, "dex": 14.0}},
		},
		{
			name:       "Error: Overwrites non-map intermediate",
			nested:     map[string]any{"stats": "not a map"},
			path:       Path{"stats", "str"},
			value:      16.0,
			wantNested: nil,
			wantErr:    maputils.ErrNotMap,
		},
		{
			name:       "Error: Nil map - returns error",
			nested:     nil,
			path:       Path{"stats"},
			value:      10.0,
			wantNested: nil,
			wantErr:    maputils.ErrNilMap,
		},
		{
			name:       "Error: Empty path - returns error",
			nested:     map[string]any{"a": 1.0},
			path:       Path{},
			value:      2.0,
			wantNested: map[string]any{"a": 1.0},
			wantErr:    maputils.ErrEmptyPath,
		},
		{
			name:       "Happy path: String value",
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
