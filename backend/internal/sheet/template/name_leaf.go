package template

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNameLeafMissing = errors.New("name leaf does not resolve to a leaf")

	ErrNameLeafNotText = errors.New("name leaf must be a text leaf")
)

func ValidateNameLeaf(tmpl Template) error {
	if tmpl.NameLeaf == "" {
		return nil
	}
	node, ok := lookupNode(tmpl.Fields, strings.Split(tmpl.NameLeaf, pathSeparator))
	if !ok || !node.IsLeaf() {
		return fmt.Errorf("%w: %q", ErrNameLeafMissing, tmpl.NameLeaf)
	}
	if node.Schema.Type != fieldTypeText {
		return fmt.Errorf("%w: %q", ErrNameLeafNotText, tmpl.NameLeaf)
	}
	return nil
}

func (t *Template) SuggestName(values map[string]any) string {
	if t.NameLeaf == "" {
		return ""
	}
	raw, ok := Path(strings.Split(t.NameLeaf, pathSeparator)).Get(values)
	if !ok {
		return ""
	}
	name, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(name)
}

func lookupNode(group FieldGroup, segments []string) (FieldNode, bool) {
	node, ok := group[segments[0]]
	if !ok {
		return FieldNode{}, false
	}
	if len(segments) == 1 {
		return node, true
	}
	if !node.IsGroup() {
		return FieldNode{}, false
	}
	return lookupNode(node.Children, segments[1:])
}
