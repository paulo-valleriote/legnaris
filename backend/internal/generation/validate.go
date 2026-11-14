package generation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strconv"
	"unicode/utf8"

	"legnaris/backend/internal/sheet/template"
)

// ErrInvalidOutput is the sentinel of a model output the Template rejects. Every
// rejection is an *OutputError, which carries the problems to feed back.
var ErrInvalidOutput = errors.New("generation: model output is invalid")

// maxProblems caps the problems one OutputError carries, so a retry prompt stays
// short whatever the model produced.
const maxProblems = 10

// OutputError reports every problem found in one model output. Each problem names
// a field and the rule it broke, never the value the model produced, so the list
// is safe to feed back to the model and to keep out of the logs.
type OutputError struct {
	Problems []string
}

// Error describes the rejection by its first problem and the count.
func (e *OutputError) Error() string {
	if len(e.Problems) == 0 {
		return ErrInvalidOutput.Error()
	}
	return fmt.Sprintf("%s: %s (%d problems)", ErrInvalidOutput, e.Problems[0], len(e.Problems))
}

// Unwrap lets errors.Is match ErrInvalidOutput.
func (e *OutputError) Unwrap() error {
	return ErrInvalidOutput
}

// ParseOutput decodes raw as exactly one JSON object. Numbers are kept as
// json.Number, so Expand can reject a number that does not fit a float64 instead of
// letting the decoder round it.
func ParseOutput(raw json.RawMessage) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, &OutputError{Problems: []string{"output is not valid JSON"}}
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, &OutputError{Problems: []string{"output has data after the JSON object"}}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, &OutputError{Problems: []string{"output must be a JSON object"}}
	}
	return object, nil
}

// Expand checks output against the Template and returns the nested leaf values it
// sets: each generated number, text or flag, and the 0/1 flags of every exclusive
// group, where the chosen member is 1 and its siblings are 0. Unknown keys, missing
// keys, wrong types, out-of-range numbers and over-long texts are all reported in
// one *OutputError.
func Expand(tmpl *template.Template, output map[string]any) (map[string]any, error) {
	if tmpl == nil {
		return nil, ErrNoTemplate
	}
	c := &checker{}
	values := c.group(tmpl.Fields, output, "")
	if len(c.problems) > 0 {
		return nil, &OutputError{Problems: c.problems}
	}
	return values, nil
}

// checker collects the problems of one output.
type checker struct {
	problems []string
}

// add records one problem as "path: rule", stopping after maxProblems.
func (c *checker) add(path, rule string) {
	if len(c.problems) >= maxProblems {
		return
	}
	c.problems = append(c.problems, path+": "+rule)
}

// group checks one object against one FieldGroup and returns the values it sets.
func (c *checker) group(fields template.FieldGroup, data map[string]any, prefix string) map[string]any {
	lv := levelOf(fields)
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

// rejectUnknown reports every key of data that the level does not declare. Keys of
// the level's members are checked as the exclusive group they belong to.
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

// leaf checks one generated value against its schema and returns it converted to
// the stored type.
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

// number checks a JSON number against its finite float64 range and its bounds.
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

// text checks a JSON string against its length bound, counted in characters.
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

// child checks a nested object and stores its values under name.
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

// exclusive checks the option picked for one exclusive group and sets the 0/1 flag
// of every member of the group, generatable or not, so the group keeps at most one
// active member.
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
