package template

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNameLeafMissing reports a NameLeaf that does not resolve to a leaf of the Template.
	ErrNameLeafMissing = errors.New("name leaf does not resolve to a leaf")

	// ErrNameLeafNotText reports a NameLeaf that is a leaf but not a text field.
	ErrNameLeafNotText = errors.New("name leaf must be a text leaf")
)

// ValidateNameLeaf checks that a Template's NameLeaf, when declared, resolves to
// a text leaf. A rename or move of that leaf fails at boot rather than leaving
// clients with an empty suggested name. An empty NameLeaf is valid.
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

// SuggestName derives the Sheet name from generated values: the trimmed string
// at NameLeaf. It returns the empty string when the Template declares no
// NameLeaf, the leaf is absent from values, or the value is not a string or is
// blank, so the caller leaves the name for the player to type.
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

// lookupNode resolves segments through nested groups and reports the node at the
// end of the path, or false when a segment is missing or an intermediate is a leaf.
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
