package maputils

import (
	"errors"
	"fmt"
	"math"
	"reflect"
)

var (
	ErrNilMap           = errors.New("cannot set on nil map")
	ErrEmptyPath        = errors.New("empty path")
	ErrNotMap           = errors.New("intermediate value is not a map")
	ErrMaxDepthExceeded = errors.New("map nesting exceeds the maximum depth")
)

const (
	MaxNestingDepth = 5

	epsilon = 1e-9
)

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

func splitPath(key string) []string {
	if key == "" {
		return nil
	}
	return splitString(key, '.')
}

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

func FlattenPathKeys(nested map[string]any) (map[string]any, error) {
	flat := make(map[string]any)
	if nested == nil {
		return flat, nil
	}
	if err := flattenRecursive([]string{}, nested, flat, '.'); err != nil {
		return nil, err
	}
	return flat, nil
}

func ToEngineEnv(nested map[string]any) (map[string]any, error) {
	flat := make(map[string]any)
	if nested == nil {
		return flat, nil
	}
	if err := FlattenRecursive([]string{}, nested, flat); err != nil {
		return nil, err
	}
	return flat, nil
}

func FlattenRecursive(prefix []string, nested map[string]any, flat map[string]any) error {
	return flattenRecursive(prefix, nested, flat, '_')
}

func flattenRecursive(prefix []string, nested map[string]any, flat map[string]any, sep rune) error {
	if len(prefix) >= MaxNestingDepth {
		return fmt.Errorf("%w: max %d levels", ErrMaxDepthExceeded, MaxNestingDepth)
	}
	for key, value := range nested {
		newPrefix := append(append([]string{}, prefix...), key)
		if nestedMap, ok := value.(map[string]any); ok && len(nestedMap) > 0 {
			if err := flattenRecursive(newPrefix, nestedMap, flat, sep); err != nil {
				return err
			}
		} else {
			flat[joinWithSeparator(newPrefix, sep)] = value
		}
	}
	return nil
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
	if _, ok := ToFloat64(a); ok {
		return equalNumeric(a, b)
	}
	if _, ok := ToFloat64(b); ok {
		return equalNumeric(a, b)
	}
	return reflect.DeepEqual(a, b)
}

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
	af, aok := ToFloat64(a)
	bf, bok := ToFloat64(b)
	if !aok || !bok {
		return false
	}
	return float64Equal(af, bf)
}

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

func signedIntParts(v int64) (uint64, bool, bool) {
	if v < 0 {
		return uint64(-v), true, true
	}
	return uint64(v), false, true
}

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

func ToFloat64(v any) (float64, bool) {
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
