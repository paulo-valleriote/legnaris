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
