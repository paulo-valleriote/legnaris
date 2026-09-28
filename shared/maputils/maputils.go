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
	// epsilon is the tolerance for numeric comparisons in Equal, applied as a
	// *relative* bound: |a-b| <= epsilon * max(1, |a|, |b|).
	// Because it scales with magnitude, the absolute slack grows with the value:
	// up to 1e-9 at magnitude 1, but up to 1.0 at magnitude 1e9. That is the
	// intent for character stats (a rounding difference must not cascade), but
	// it makes Equal unsuitable for fields needing exact large values, such as
	// currency or XP counters.
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
		path := splitPath(key)
		if err := Set(nested, path, value); err != nil {
			return err
		}
	}
	return nil
}

// splitPath splits a flat path key into segments.
// An empty key yields nil: Get returns (nil, false) for it, while Set rejects
// it with ErrEmptyPath.
// Unused leading/trailing separators produce empty segments which are preserved.
func splitPath(key string) []string {
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
	path := splitPath(key)
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
	totalLen := 0
	for _, p := range parts {
		totalLen += len(p)
	}
	totalLen += len(parts) - 1

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

// Equal compares two values with strict equality for non-numeric types and a
// scaled epsilon tolerance for numeric ones.
//
// Strings, booleans and nil are compared strictly against the same type only;
// slices and maps fall back to reflect.DeepEqual. The epsilon tolerance
// therefore applies to scalar numeric values only — a map or slice operand is
// always compared exactly, no matter how its float members differ.
//
// Numeric operands (every int/uint size, float32 and float64) are compared
// numerically: when both sides are integers the comparison is exact, with no
// precision loss and no wrap-around (uint64 values above math.MaxInt64 stay
// unsigned). Otherwise both sides are converted to float64 and compared with
// the relative tolerance `epsilon` (|a-b| <= epsilon * max(1, |a|, |b|)), which
// absorbs floating-point rounding differences such as 0.1+0.2 != 0.3.
// A float32 pair keeps an exact comparison. NaN is never equal to anything.
//
// Returns false when a numeric and a non-numeric value are compared, and for
// different concrete non-numeric types.
func Equal(a, b any) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	}
	// A numeric operand on either side routes to the tolerant comparison;
	// toFloat64 doubles as the numeric-type check.
	if _, ok := toFloat64(a); ok {
		return equalNumeric(a, b)
	}
	if _, ok := toFloat64(b); ok {
		return equalNumeric(a, b)
	}
	return reflect.DeepEqual(a, b)
}

// equalNumeric compares two numeric operands.
// Integer pairs are compared exactly through intParts; every other pairing
// (mixed integer/float, float32 vs float64) goes through float64 with the
// scaled epsilon tolerance.
// Returns false if either operand is not numeric.
func equalNumeric(a, b any) bool {
	if av, ok := a.(float32); ok {
		if bv, ok := b.(float32); ok {
			return av == bv
		}
	}
	amag, aneg, aInt := intParts(a)
	bmag, bneg, bInt := intParts(b)
	if aInt && bInt {
		return aneg == bneg && amag == bmag
	}
	af, aok := toFloat64(a)
	bf, bok := toFloat64(b)
	if !aok || !bok {
		return false
	}
	return float64Equal(af, bf)
}

// intParts normalises any integer value into an unsigned magnitude plus a sign
// flag, so integers of every size compare exactly without wrapping on
// conversion: uint64(math.MaxUint64) stays (math.MaxUint64, false) instead of
// collapsing to int64(-1).
// Returns ok=false for non-integer types.
func intParts(v any) (mag uint64, neg bool, ok bool) {
	switch val := v.(type) {
	case int:
		return signedIntParts(int64(val))
	case int8:
		return signedIntParts(int64(val))
	case int16:
		return signedIntParts(int64(val))
	case int32:
		return signedIntParts(int64(val))
	case int64:
		return signedIntParts(val)
	case uint:
		return uint64(val), false, true
	case uint8:
		return uint64(val), false, true
	case uint16:
		return uint64(val), false, true
	case uint32:
		return uint64(val), false, true
	case uint64:
		return val, false, true
	}
	return 0, false, false
}

// signedIntParts splits a signed integer into magnitude and sign.
// Negating math.MinInt64 wraps to itself and converting that bit pattern to
// uint64 yields the exact magnitude 1<<63, so no special case is needed.
func signedIntParts(v int64) (uint64, bool, bool) {
	if v < 0 {
		return uint64(-v), true, true
	}
	return uint64(v), false, true
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

// toFloat64 converts any numeric value (every int/uint size, float32 and
// float64) to float64. Returns false for non-numeric types.
// Integer conversion goes through intParts so the magnitude is carried over
// before the lossy float conversion.
func toFloat64(v any) (float64, bool) {
	if mag, neg, ok := intParts(v); ok {
		f := float64(mag)
		if neg {
			return -f, true
		}
		return f, true
	}
	switch val := v.(type) {
	case float32:
		return float64(val), true
	case float64:
		return val, true
	}
	return 0, false
}
