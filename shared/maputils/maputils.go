package maputils

import (
	"errors"
	"reflect"
)

var (
	// ErrNilMap is returned when attempting to set a value on a nil map.
	ErrNilMap = errors.New("cannot set on nil map")
	// ErrEmptyPath is returned when the path is empty.
	ErrEmptyPath = errors.New("empty path")
	// ErrNotMap is returned when an intermediate value is not a map.
	ErrNotMap = errors.New("intermediate value is not a map")
)

// Get retrieves a value from a nested map using a path of keys.
// Returns the value and true if found, nil and false otherwise.
func Get(nested map[string]any, path []string) (any, bool) {
	if nested == nil || len(path) == 0 {
		return nil, false
	}

	current := nested
	for i, key := range path {
		if i == len(path)-1 {
			val, ok := current[key]
			return val, ok
		}
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

// Set sets a value in a nested map using a path of keys.
// Creates intermediate maps as needed.
// Returns error if nested is nil, path is empty, or type assertion fails at intermediate levels.
func Set(nested map[string]any, path []string, value any) error {
	if len(path) == 0 {
		return ErrEmptyPath
	}

	if nested == nil {
		return ErrNilMap
	}

	current := nested
	for i, key := range path {
		if i == len(path)-1 {
			current[key] = value
			return nil
		}
		if current[key] == nil {
			current[key] = make(map[string]any)
		}
		nextMap, ok := current[key].(map[string]any)
		if !ok {
			return ErrNotMap
		}
		current = nextMap
	}
	return nil
}

// ApplyUpdates applies a flat map of dot-separated keys to a nested map.
// Each key in updates is split by "." and set via Set.
// Returns the first error encountered, or nil if all updates succeed.
func ApplyUpdates(nested map[string]any, updates map[string]any) error {
	if nested == nil || updates == nil {
		return nil
	}
	for key, value := range updates {
		path := splitPath(key)
		if err := Set(nested, path, value); err != nil {
			return err
		}
	}
	return nil
}

// splitPath splits a dot-separated path into segments.
// Unused leading/trailing dots produce empty segments which are preserved.
func splitPath(key string) []string {
	if key == "" {
		return []string{""}
	}
	// Manual split to avoid strings.Split allocation for simple cases
	// But strings.Split is fine here for clarity
	return splitString(key, '.')
}

func splitString(s string, sep rune) []string {
	var result []string
	start := 0
	for i, r := range s {
		if r == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// ToEngineEnv converts a nested map to a flat map with underscore-separated keys
// for use with the expr-lang formula engine.
// Example: {"stats": {"str": 10}} → {"stats_str": 10}
func ToEngineEnv(nested map[string]any) map[string]any {
	flat := make(map[string]any)
	if nested == nil {
		return flat
	}
	FlattenRecursive([]string{}, nested, flat)
	return flat
}

// FlattenRecursive is a helper for flattening a nested map with a given prefix.
// It appends results to the provided flat map.
// Exported for use by template.Path.Flatten.
func FlattenRecursive(prefix []string, nested map[string]any, flat map[string]any) {
	for key, value := range nested {
		newPrefix := append(append([]string{}, prefix...), key)
		if nestedMap, ok := value.(map[string]any); ok && len(nestedMap) > 0 {
			FlattenRecursive(newPrefix, nestedMap, flat)
		} else {
			flat[joinWithUnderscore(newPrefix)] = value
		}
	}
}

func joinWithUnderscore(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	// Estimate capacity to avoid reallocations
	totalLen := 0
	for _, p := range parts {
		totalLen += len(p)
	}
	totalLen += len(parts) - 1 // underscores

	b := make([]byte, 0, totalLen)
	for i, p := range parts {
		if i > 0 {
			b = append(b, '_')
		}
		b = append(b, p...)
	}
	return string(b)
}

// DeepCopy creates a deep copy of a nested map[string]any.
// Returns nil if input is nil.
func DeepCopy(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if nestedMap, ok := v.(map[string]any); ok {
			dst[k] = DeepCopy(nestedMap)
		} else {
			dst[k] = v
		}
	}
	return dst
}

// Equal compares two values for equality with type strictness.
// Supports: float64, string, bool, nil, slices, maps (via DeepEqual fallback).
// Returns false for different concrete types (e.g., int vs float64).
func Equal(a, b any) bool {
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	}
	return reflect.DeepEqual(a, b)
}
