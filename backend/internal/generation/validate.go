package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"unicode/utf8"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
	sharedjson "legnaris/backend/shared/json"
)

var ErrInvalidOutput = errors.New("generation: model output is invalid")

const maxProblems = 10

type OutputError struct {
	Problems []string
}

func (e *OutputError) Error() string {
	if len(e.Problems) == 0 {
		return ErrInvalidOutput.Error()
	}
	return fmt.Sprintf("%s: %s (%d problems)", ErrInvalidOutput, e.Problems[0], len(e.Problems))
}

func (e *OutputError) Unwrap() error {
	return ErrInvalidOutput
}

func ParseOutput(raw json.RawMessage) (map[string]any, error) {
	var value any
	if err := sharedjson.DecodeBytesUseNumber(raw, &value); err != nil {
		return nil, &OutputError{Problems: []string{outputParseProblem(err)}}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, &OutputError{Problems: []string{"output must be a JSON object"}}
	}
	return object, nil
}

func outputParseProblem(err error) string {
	switch {
	case errors.Is(err, sharedjson.ErrBodyTooLarge):
		return "output exceeds the size bound"
	case errors.Is(err, sharedjson.ErrTrailingData):
		return "output has data after the JSON object"
	default:
		return "output is not valid JSON"
	}
}

func Expand(tmpl *template.Template, kind plan.SheetKind, output map[string]any) (map[string]any, error) {
	if tmpl == nil {
		return nil, ErrNoTemplate
	}
	c := &checker{kind: kind}
	values := c.group(tmpl.Fields, output, "")
	if len(c.problems) > 0 {
		return nil, &OutputError{Problems: c.problems}
	}
	return values, nil
}

type checker struct {
	kind     plan.SheetKind
	problems []string
}

func (c *checker) add(path, rule string) {
	if len(c.problems) >= maxProblems {
		return
	}
	c.problems = append(c.problems, path+": "+rule)
}

func (c *checker) group(fields template.FieldGroup, data map[string]any, prefix string) map[string]any {
	lv := levelOf(fields, c.kind)
	c.rejectUnknown(lv, data, prefix)
	values := map[string]any{}
	for _, name := range slices.Sorted(maps.Keys(lv.leaves)) {
		raw, present := data[name]
		if !present {
			c.add(join(prefix, name), "is required")
			continue
		}
		if value, ok := c.leaf(join(prefix, name), lv.leaves[name], raw); ok {
			values[name] = value
		}
	}
	for _, name := range slices.Sorted(maps.Keys(lv.groups)) {
		c.child(name, lv.groups[name], data, prefix, values)
	}
	for _, exclusiveGroup := range slices.Sorted(maps.Keys(lv.members)) {
		c.exclusive(exclusiveGroup, lv, data, prefix, values)
	}
	return values
}

func (c *checker) rejectUnknown(lv level, data map[string]any, prefix string) {
	allowed := map[string]bool{}
	for name := range lv.leaves {
		allowed[name] = true
	}
	for name := range lv.groups {
		allowed[name] = true
	}
	for name := range lv.exclusive {
		allowed[name] = true
	}
	for _, key := range slices.Sorted(maps.Keys(data)) {
		if !allowed[key] {
			c.add(join(prefix, key), "is not a field the template allows")
		}
	}
}

func (c *checker) leaf(path string, schema *template.FieldSchema, raw any) (any, bool) {
	switch schema.Type {
	case fieldTypeNumber:
		return c.number(path, schema, raw)
	case fieldTypeText:
		return c.text(path, schema, raw)
	case fieldTypeBool:
		value, ok := raw.(bool)
		if !ok {
			c.add(path, "must be a boolean")
		}
		return value, ok
	default:
		c.add(path, "has a type the generation cannot fill")
		return nil, false
	}
}

func (c *checker) number(path string, schema *template.FieldSchema, raw any) (any, bool) {
	num, ok := raw.(json.Number)
	if !ok {
		c.add(path, "must be a number")
		return nil, false
	}
	value, err := strconv.ParseFloat(num.String(), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		c.add(path, "must be a finite number")
		return nil, false
	}
	if schema.Min != nil && value < *schema.Min {
		c.add(path, "is below the minimum")
		return nil, false
	}
	if schema.Max != nil && value > *schema.Max {
		c.add(path, "is above the maximum")
		return nil, false
	}
	return value, true
}

func (c *checker) text(path string, schema *template.FieldSchema, raw any) (any, bool) {
	value, ok := raw.(string)
	if !ok {
		c.add(path, "must be a string")
		return nil, false
	}
	if schema.MaxLength != nil && utf8.RuneCountInString(value) > *schema.MaxLength {
		c.add(path, "is longer than the maximum length")
		return nil, false
	}
	return value, true
}

func (c *checker) child(name string, fields template.FieldGroup, data map[string]any, prefix string, values map[string]any) {
	path := join(prefix, name)
	raw, present := data[name]
	if !present {
		c.add(path, "is required")
		return
	}
	object, ok := raw.(map[string]any)
	if !ok {
		c.add(path, "must be an object")
		return
	}
	values[name] = c.group(fields, object, path)
}

func (c *checker) exclusive(name string, lv level, data map[string]any, prefix string, values map[string]any) {
	path := join(prefix, name)
	members := lv.members[name]
	options := lv.exclusive[name]
	if len(options) == 0 {
		return
	}
	raw, present := data[name]
	if !present {
		c.add(path, "is required")
		return
	}
	picked, ok := raw.(string)
	if !ok || !slices.Contains(options, picked) {
		c.add(path, "must be one of the listed options")
		return
	}
	for _, member := range members {
		values[member] = 0.0
		if member == picked {
			values[member] = 1.0
		}
	}
}
