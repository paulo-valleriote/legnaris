package template

import (
	"legnaris/shared/maputils"
	"strings"
)

// Path represents a field path as a slice of string segments.
// Example: []string{"stats", "str"} → "stats.str" (MapKey) / "stats_str" (EngineKey)
type Path []string

// String returns the path joined into a flat key for use as DAG map keys.
// Example: []string{"stats", "str"} → "stats.str"
func (p Path) String() string {
	return strings.Join(p, ".")
}

// EngineKey returns the underscore-separated path for use with expr-lang formula engine.
// Example: []string{"stats", "str"} → "stats_str"
func (p Path) EngineKey() string {
	return strings.Join(p, "_")
}

// Parent returns the path without the last segment.
// Example: []string{"stats", "str"} → []string{"stats"}
// Returns empty Path if already at root (len == 1).
func (p Path) Parent() Path {
	if len(p) <= 1 {
		return Path{}
	}
	return p[:len(p)-1]
}

// Last returns the last segment of the path.
// Example: []string{"stats", "str"} → "str"
// Returns empty string if path is empty.
func (p Path) Last() string {
	if len(p) == 0 {
		return ""
	}
	return p[len(p)-1]
}

// Get retrieves a value from a nested map using this path.
// Delegates to maputils.Get.
func (p Path) Get(nested map[string]any) (any, bool) {
	return maputils.Get(nested, p)
}

// Set sets a value in a nested map using this path.
// Delegates to maputils.Set.
// Returns error if path is invalid or type assertion fails.
func (p Path) Set(nested map[string]any, value any) error {
	return maputils.Set(nested, p, value)
}

// Flatten converts a nested map to a flat map with underscore-separated keys
// using this path's segments as the prefix.
// Delegates to maputils.ToEngineEnv but with a prefix.
func (p Path) Flatten(nested map[string]any) map[string]any {
	if len(p) == 0 {
		return maputils.ToEngineEnv(nested)
	}
	// Get the nested map at this path, then flatten it with this path as prefix
	val, ok := p.Get(nested)
	if !ok {
		return map[string]any{}
	}
	subMap, ok := val.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	flat := make(map[string]any)
	maputils.FlattenRecursive(p, subMap, flat)
	return flat
}

// IsRoot returns true if the path has exactly one segment.
func (p Path) IsRoot() bool {
	return len(p) == 1
}

// Segments returns a copy of the underlying slice.
func (p Path) Segments() []string {
	if p == nil {
		return nil
	}
	result := make([]string, len(p))
	copy(result, p)
	return result
}

// Equal compares two paths for equality.
func (p Path) Equal(other Path) bool {
	if len(p) != len(other) {
		return false
	}
	for i := range p {
		if p[i] != other[i] {
			return false
		}
	}
	return true
}
