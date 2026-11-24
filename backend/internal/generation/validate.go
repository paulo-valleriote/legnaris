package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
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
	c := &checker{tmpl: tmpl, kind: kind}
	if denylist, ok := template.DefaultGameSystems().Denylist(tmpl.GameSystemID); ok {
		c.denylist = denylist
	}
	values := c.group(tmpl.Fields, output, "")
	c.classSubclass(values)
	c.saveProficiencies(values)
	if len(c.problems) > 0 {
		return nil, &OutputError{Problems: c.problems}
	}
	return values, nil
}

type checker struct {
	tmpl     *template.Template
	kind     plan.SheetKind
	denylist []string
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
	for _, name := range slices.Sorted(maps.Keys(lv.lists)) {
		c.list(name, lv.lists[name], data, prefix, values)
	}
	for _, exclusiveGroup := range slices.Sorted(maps.Keys(lv.exclusive)) {
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
	for name := range lv.lists {
		allowed[name] = true
	}
	unknown := 0
	for key := range data {
		if !allowed[key] {
			unknown++
		}
	}
	if unknown == 0 {
		return
	}
	path := prefix
	if path == "" {
		path = "output"
	}
	if unknown == 1 {
		c.add(path, "has 1 field the template does not allow")
		return
	}
	c.add(path, fmt.Sprintf("has %d fields the template does not allow", unknown))
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
	if schema.Flag && value != 0 && value != 1 {
		c.add(path, "must be 0 or 1")
		return nil, false
	}
	if len(schema.GenerationValues) > 0 && !slices.Contains(schema.GenerationValues, value) {
		c.add(path, "must be one of the listed options")
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
	if len(schema.GenerationEnum) > 0 && !slices.Contains(schema.GenerationEnum, value) {
		c.add(path, "must be one of the listed options")
		return nil, false
	}
	if term, denied := containsDeniedTerm(value, c.denylist); denied {
		c.add(path, fmt.Sprintf("contains the denied term %q", term))
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

func (c *checker) list(name string, list *template.ListSchema, data map[string]any, prefix string, values map[string]any) {
	path := join(prefix, name)
	raw, present := data[name]
	if !present {
		c.add(path, "is required")
		return
	}
	items, ok := raw.([]any)
	if !ok {
		c.add(path, "must be an array")
		return
	}
	if len(items) > list.MaxRecords {
		c.add(path, fmt.Sprintf("has more than %d records", list.MaxRecords))
		return
	}
	records := make([]any, 0, len(items))
	for index, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			c.add(recordPath(path, index), "must be an object")
			continue
		}
		records = append(records, c.record(path, index, list.Shape, object))
	}
	values[name] = records
}

func recordPath(listPath string, index int) string {
	return fmt.Sprintf("%s[%d]", listPath, index)
}

func (c *checker) record(listPath string, index int, shape template.FieldGroup, data map[string]any) map[string]any {
	path := recordPath(listPath, index)
	lv := shapeLevel(shape)
	c.rejectUnknown(lv, data, path)
	values := map[string]any{}
	for _, name := range slices.Sorted(maps.Keys(lv.leaves)) {
		fieldPath := join(path, name)
		raw, present := data[name]
		if !present {
			c.add(fieldPath, "is required")
			continue
		}
		if value, ok := c.leaf(fieldPath, lv.leaves[name], raw); ok {
			values[name] = value
		}
	}
	for _, exclusiveGroup := range slices.Sorted(maps.Keys(lv.exclusive)) {
		c.exclusive(exclusiveGroup, lv, data, path, values)
	}
	return values
}

func (c *checker) classSubclass(values map[string]any) {
	rule := c.tmpl.ClassSubclass
	if rule == nil {
		return
	}
	class, classOK := lookupString(values, rule.Class)
	subclass, subclassOK := lookupString(values, rule.Subclass)
	level, levelOK := lookupNumber(values, rule.Level)
	if !classOK || !subclassOK || !levelOK {
		return
	}
	names, ok := template.DefaultGameSystems().AllowedNames(c.tmpl.GameSystemID)
	if !ok {
		return
	}
	index := slices.Index(names.Classes, class)
	if index < 0 {
		return
	}
	if level < float64(rule.SubclassLevel) {
		if subclass != "" {
			c.add(strings.Join(rule.Subclass, "."), fmt.Sprintf("must be empty below level %d", rule.SubclassLevel))
		}
		return
	}
	if index >= len(names.Subclasses) {
		c.add(strings.Join(rule.Class, "."), "has no subclass in the game system")
		return
	}
	expected := names.Subclasses[index]
	if subclass != expected {
		c.add(strings.Join(rule.Subclass, "."), fmt.Sprintf("must be %q for class %q", expected, class))
	}
}

func (c *checker) saveProficiencies(values map[string]any) {
	rule := c.tmpl.SaveProficiencies
	if rule == nil {
		return
	}
	count := 0
	for _, path := range rule.Paths {
		value, ok := lookupNumber(values, path)
		if !ok {
			return
		}
		if value != 0 {
			count++
		}
	}
	if count == rule.Required {
		return
	}
	c.add(saveProficiencyPath(rule), fmt.Sprintf("must have exactly %d save proficiencies", rule.Required))
}

func saveProficiencyPath(rule *template.SaveProficiencyRule) string {
	if len(rule.Paths) == 0 {
		return "saves"
	}
	return strings.Join(rule.Paths[0][:len(rule.Paths[0])-1], ".")
}

func containsDeniedTerm(text string, denylist []string) (string, bool) {
	if text == "" || len(denylist) == 0 {
		return "", false
	}
	lowered := strings.ToLower(text)
	for _, term := range denylist {
		needle := strings.ToLower(term)
		if needle == "" {
			continue
		}
		if containsWholeWord(lowered, needle) {
			return term, true
		}
	}
	return "", false
}

func containsWholeWord(text, needle string) bool {
	for offset := 0; offset <= len(text)-len(needle); {
		index := strings.Index(text[offset:], needle)
		if index < 0 {
			return false
		}
		start := offset + index
		end := start + len(needle)
		if !wordRuneBefore(text, start) && !wordRuneAfter(text, end) {
			return true
		}
		offset = start + 1
	}
	return false
}

func wordRuneBefore(text string, index int) bool {
	if index <= 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(text[:index])
	return isWordRune(r)
}

func wordRuneAfter(text string, index int) bool {
	if index >= len(text) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(text[index:])
	return isWordRune(r)
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func lookupString(values map[string]any, path []string) (string, bool) {
	value, ok := lookupValue(values, path)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func lookupNumber(values map[string]any, path []string) (float64, bool) {
	value, ok := lookupValue(values, path)
	if !ok {
		return 0, false
	}
	number, ok := value.(float64)
	return number, ok
}

func lookupValue(values map[string]any, path []string) (any, bool) {
	var current any = values
	for _, name := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[name]
		if !ok {
			return nil, false
		}
	}
	return current, len(path) > 0
}

func (c *checker) exclusive(name string, lv level, data map[string]any, prefix string, values map[string]any) {
	path := join(prefix, name)
	options := lv.exclusive[name]
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
	for _, option := range options {
		values[option] = 0.0
		if option == picked {
			values[option] = 1.0
		}
	}
}
