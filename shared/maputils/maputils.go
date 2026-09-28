package maputils

import (
	"errors"
	"math"
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

const (
	// epsilon is the tolerance for float64 comparisons in Equal.
	// Values differing by less than this relative to their magnitude are considered equal.
	epsilon = 1e-9
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

// ApplyUpdates applies a flat map of path keys (e.g., "stats.str") to a nested map.
// Each key in updates is split by "." and set via Set.
// Returns the first error encountered, or nil if all updates succeed.
func ApplyUpdates(nested map[string]any, updates map[string]any) error {
	if nested == nil || updates == nil {
		return nil
	}
	for key, value := range updates {
		path := SplitPath(key)
		if err := Set(nested, path, value); err != nil {
			return err
		}
	}
	return nil
}

// SplitPath splits a flat path key into segments.
// An empty key yields nil (which Get/Set reject with ErrEmptyPath).
// Unused leading/trailing separators produce empty segments which are preserved.
func SplitPath(key string) []string {
	if key == "" {
		return nil
	}
	return splitString(key, '.')
}

// GetFlat retrieves a value from a nested map using a flat path key (e.g., "stats.str").
// Returns the value and true if found, nil and false otherwise.
func GetFlat(nested map[string]any, key string) (any, bool) {
	if nested == nil || key == "" {
		return nil, false
	}
	path := SplitPath(key)
	return Get(nested, path)
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

// FlattenPathKeys converts a nested map to a flat map of path keys.
// Example: {"stats": {"str": 10}} → {"stats.str": 10}
func FlattenPathKeys(nested map[string]any) map[string]any {
	flat := make(map[string]any)
	if nested == nil {
		return flat
	}
	flattenRecursive([]string{}, nested, flat, '.')
	return flat
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
	flattenRecursive(prefix, nested, flat, '_')
}

// flattenRecursive flattens a nested map into flat keys joined by the given separator.
func flattenRecursive(prefix []string, nested map[string]any, flat map[string]any, sep rune) {
	for key, value := range nested {
		newPrefix := append(append([]string{}, prefix...), key)
		if nestedMap, ok := value.(map[string]any); ok && len(nestedMap) > 0 {
			flattenRecursive(newPrefix, nestedMap, flat, sep)
		} else {
			flat[joinWithSeparator(newPrefix, sep)] = value
		}
	}
}

func joinWithSeparator(parts []string, sep rune) string {
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
	totalLen += len(parts) - 1 // separators

	b := make([]byte, 0, totalLen)
	for i, p := range parts {
		if i > 0 {
			b = append(b, byte(sep))
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

// Equal compares two values for equality with tolerance for numeric types.
// For float64, uses a scaled epsilon tolerance to avoid spurious recalculations
// caused by floating-point rounding differences (e.g., 0.1+0.2 != 0.3).
// All integer types (int, int8-64, uint, uint8-64) are compared exactly.
// float32 is compared exactly when both operands are float32; otherwise promoted to float64.
// Mixed integer/float comparisons use float64 with scaled tolerance.
// Returns false for different concrete non-numeric types and for NaN.
func Equal(a, b any) bool {
	// Fast path: identical types
	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			return float64Equal(av, bv)
		}
		return numericEqualFloat64(b, av)
	case float32:
		if bv, ok := b.(float32); ok {
			return av == bv // exact comparison for same-type float32
		}
		// float32 vs other: promote to float64 with tolerance
		return numericEqualFloat64(b, float64(av))
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	// Exact integer comparison fast-path (covers all signed/unsigned sizes)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		if avInt, ok := toInt64(a); ok {
			if bvInt, ok := toInt64(b); ok {
				return avInt == bvInt
			}
		}
		// a is integer but b is not: fall through to numericEqualFloat64 if b is float
		if bv, ok := b.(float64); ok {
			return numericEqualFloat64(a, bv)
		}
		if bv, ok := b.(float32); ok {
			return numericEqualFloat64(a, float64(bv))
		}
		return reflect.DeepEqual(a, b)
	}
	// a is not in the fast-path types; check if b is float64/float32
	// (handles int, int32, int64, float32, etc. on the left side)
	if bv, ok := b.(float64); ok {
		return numericEqualFloat64(a, bv)
	}
	if bv, ok := b.(float32); ok {
		return numericEqualFloat64(a, float64(bv))
	}
	return reflect.DeepEqual(a, b)
}

// toInt64 converts supported integer types to int64 for exact comparison.
// Returns false for non-integer types.
func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int8:
		return int64(val), true
	case int16:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return val, true
	case uint:
		return int64(val), true
	case uint8:
		return int64(val), true
	case uint16:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint64:
		return int64(val), true
	}
	return 0, false
}

// float64Equal compares two float64 values with scaled epsilon tolerance.
// NaN is never equal to anything, including itself.
// Uses relative tolerance: |a-b| <= epsilon * max(1, |a|, |b|)
func float64Equal(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return a == b
	}
	diff := math.Abs(a - b)
	scale := math.Max(1.0, math.Max(math.Abs(a), math.Abs(b)))
	return diff <= epsilon*scale
}

// numericEqualFloat64 converts a non-float64 numeric value to float64
// and compares it with bv using epsilon tolerance.
// Returns false if a is not a numeric type.
func numericEqualFloat64(a any, bv float64) bool {
	av, ok := toFloat64(a)
	if !ok {
		return false
	}
	return float64Equal(av, bv)
}

// toFloat64 converts supported numeric types to float64.
// Returns false for non-numeric types and complex numbers.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	}
	return 0, false
}
