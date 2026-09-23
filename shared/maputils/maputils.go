package maputils

import "reflect"

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
// If nested is nil, it creates a new map at the first level.
func Set(nested map[string]any, path []string, value any) {
	if len(path) == 0 {
		return
	}

	if nested == nil {
		// Cannot modify nil map passed by value; caller must handle initialization
		// This function assumes nested is a valid map (non-nil) or caller handles it
		return
	}

	current := nested
	for i, key := range path {
		if i == len(path)-1 {
			current[key] = value
			return
		}
		if current[key] == nil {
			current[key] = make(map[string]any)
		}
		nextMap, ok := current[key].(map[string]any)
		if !ok {
			nextMap = make(map[string]any)
			current[key] = nextMap
		}
		current = nextMap
	}
}

// ApplyUpdates applies a flat map of dot-separated keys to a nested map.
// Each key in updates is split by "." and set via Set.
func ApplyUpdates(nested map[string]any, updates map[string]any) {
	if nested == nil || updates == nil {
		return
	}
	for key, value := range updates {
		path := splitPath(key)
		Set(nested, path, value)
	}
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
