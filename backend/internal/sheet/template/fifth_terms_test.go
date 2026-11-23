package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fifthBannedTerms = []string{"d&d", "dnd", "dungeons"}

func fifthTemplateTexts(tmpl Template) []string {
	texts := []string{tmpl.ID, tmpl.Name}
	for _, action := range tmpl.Actions {
		texts = append(texts, action.ID, action.Label)
	}
	var walk func(group FieldGroup)
	walk = func(group FieldGroup) {
		for key, node := range group {
			texts = append(texts, key, node.Label)
			texts = append(texts, node.Columns...)
			if node.Schema != nil {
				texts = append(texts, node.Schema.Label, node.Schema.OptionLabel, node.Schema.ColumnLabel)
			}
			if node.List != nil {
				texts = append(texts, node.List.Label)
				texts = append(texts, node.List.Columns...)
				walk(node.List.Shape)
			}
			walk(node.Children)
		}
	}
	walk(tmpl.Fields)
	return texts
}

func fifthBannedTermIn(texts []string) string {
	for _, text := range texts {
		lower := strings.ToLower(text)
		for _, term := range fifthBannedTerms {
			if strings.Contains(lower, term) {
				return term
			}
		}
	}
	return ""
}

func TestFifthTemplatesUseOnlySRDTerms(t *testing.T) {
	templates := map[string]Template{
		fifthCharacterID: fifthCharacterTemplate,
		fifthStatBlockID: fifthStatBlockTemplate,
	}
	tests := []struct {
		name string
		id   string
	}{
		{name: "Happy path: the 5E Character carries no publisher trademark", id: fifthCharacterID},
		{name: "Happy path: the 5E Stat Block carries no publisher trademark", id: fifthStatBlockID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, fifthBannedTermIn(fifthTemplateTexts(templates[tc.id])), "template %s", tc.id)
		})
	}
}

func TestFifthTemplatesUseOnlySRDTerms_Scanner(t *testing.T) {
	t.Run("Error: a trademark in a template id or name is found", func(t *testing.T) {
		texts := fifthTemplateTexts(Template{ID: "dnd_v1", Name: "Dungeons"})
		assert.Equal(t, "dnd", fifthBannedTermIn(texts))
	})

	t.Run("Error: a trademark in a leaf label, option label or column label is found", func(t *testing.T) {
		cases := map[string]FieldSchema{
			"leaf label":   {Label: "D&D spell"},
			"option label": {Label: "Choice", OptionLabel: "DnD class"},
			"column label": {Label: "Total", ColumnLabel: "Dungeons"},
		}
		for name, schema := range cases {
			t.Run("Edge: "+name, func(t *testing.T) {
				tmpl := Template{Fields: FieldGroup{"spell": {Schema: &schema}}}
				assert.NotEmpty(t, fifthBannedTermIn(fifthTemplateTexts(tmpl)))
			})
		}
	})

	t.Run("Error: a trademark in a list or action label is found", func(t *testing.T) {
		tmpl := Template{
			Fields: FieldGroup{
				"attacks": {List: &ListSchema{Label: "D&D attacks", Capacity: 1, Shape: FieldGroup{}}},
			},
			Actions: []ResetAction{{ID: "rest", Label: "Dungeons rest"}},
		}
		assert.NotEmpty(t, fifthBannedTermIn(fifthTemplateTexts(tmpl)))
	})

	t.Run("Edge: a plain template scans clean", func(t *testing.T) {
		tmpl := Template{
			ID:     "plain_v1",
			Name:   "Plain Template",
			Fields: FieldGroup{"stats": {Schema: &FieldSchema{Label: "Dexterity"}}},
		}
		require.Empty(t, fifthBannedTermIn(fifthTemplateTexts(tmpl)))
	})
}
