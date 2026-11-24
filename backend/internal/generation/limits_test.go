package generation

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	draftLongTextRunes  = 300
	draftNotesTextRunes = 120
	draftShortTextRunes = 40
	draftCharsPerToken  = 3.5
	draftSentence       = "the bearer gains a short paraphrase of the rule, applied on each turn while the condition holds and resolved with a saving throw against the stated dc "
)

var draftTextRunesByLabel = map[string]int{
	"Name":          draftShortTextRunes,
	"Action name":   draftShortTextRunes,
	"Resource name": draftShortTextRunes,
	"Damage":        draftShortTextRunes,
	"Recharge":      draftShortTextRunes,
	"Source":        draftShortTextRunes,
	"Class":         draftShortTextRunes,
	"Subclass":      draftShortTextRunes,
	"Species":       draftShortTextRunes,
	"Background":    draftShortTextRunes,
	"Notes":         draftNotesTextRunes,
}

func TestLeaseOutlastsTimeoutWithSettleMargin(t *testing.T) {
	const leaseMargin = 20 * time.Second

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "Happy path: the default LLM timeout", timeout: env.DefaultLLMTimeout},
		{name: "Happy path: a longer configured LLM timeout", timeout: 300 * time.Second},
		{name: "Edge: the shortest configured LLM timeout", timeout: time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lease := LeaseDurationFor(tc.timeout)
			budget := jobBudgetFor(tc.timeout)
			calls := time.Duration(1+MaxSchemaRetries) * time.Duration(1+TransportRetries) * tc.timeout
			backoffs := time.Duration(1+MaxSchemaRetries) * TransportBackoff

			assert.GreaterOrEqual(t, budget, calls+backoffs,
				"the job context must cover every schema retry and transport retry at the LLM timeout")
			assert.Equal(t, budget+settleTimeout+leaseMargin, lease,
				"the lease must cover the job context, the settle window and the margin")
			assert.GreaterOrEqual(t, lease-budget-settleTimeout, leaseMargin,
				"the settle write must finish before the lease expires")
		})
	}

	t.Run("Edge: the default job context fits the advertised schema retries", func(t *testing.T) {
		schemaAttempts := time.Duration(1+MaxSchemaRetries) * env.DefaultLLMTimeout
		assert.GreaterOrEqual(t, jobBudgetFor(env.DefaultLLMTimeout), schemaAttempts)
	})
}

func TestMaxSizeDraftFitsOutputTokenCap(t *testing.T) {
	tests := []struct {
		name       string
		templateID string
	}{
		{name: "Happy path: the maximum-size 5E Character draft fits", templateID: "fifth_character_v1"},
		{name: "Happy path: the maximum-size 5E Stat Block draft fits", templateID: "fifth_stat_block_v1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := draftTemplate(t, tc.templateID)
			require.NotEmpty(t, tmpl.Kinds)

			for _, kind := range tmpl.Kinds {
				draft, caps := maxSizeDraft(t, tmpl, kind)
				require.NotEmpty(t, caps)
				for path, count := range caps {
					assert.Positive(t, count, path)
				}

				raw, err := json.Marshal(draft)
				require.NoError(t, err)

				estimate := float64(len(raw)) / draftCharsPerToken
				t.Logf("%s %s draft: %d chars, about %.0f tokens at chars/%.1f",
					tmpl.ID, kind, len(raw), estimate, draftCharsPerToken)
				assert.LessOrEqualf(t, estimate, float64(MaxOutputTokens),
					"%s %s draft is %d chars, about %.0f tokens at chars/%.1f",
					tmpl.ID, kind, len(raw), estimate, draftCharsPerToken)
			}
		})
	}
}

func draftTemplate(t *testing.T, id string) template.Template {
	t.Helper()
	for _, tmpl := range template.DefaultTemplates() {
		if tmpl.ID == id {
			return tmpl
		}
	}
	require.FailNow(t, "missing default template", id)
	return template.Template{}
}

func maxSizeDraft(t *testing.T, tmpl template.Template, kind plan.SheetKind) (map[string]any, map[string]int) {
	t.Helper()
	caps := map[string]int{}
	draft := draftGroup(t, tmpl.Fields, kind, template.Path{}, caps)
	return draft, caps
}

func draftGroup(
	t *testing.T,
	group template.FieldGroup,
	kind plan.SheetKind,
	prefix template.Path,
	caps map[string]int,
) map[string]any {
	t.Helper()
	out := map[string]any{}
	exclusive := map[string][]string{}

	for _, name := range slices.Sorted(maps.Keys(group)) {
		node := group[name]
		path := append(append(template.Path{}, prefix...), name)
		if path.String() == "rules" || path.String() == "identity.playerName" {
			continue
		}
		switch {
		case node.IsLeaf():
			draftLeaf(out, exclusive, name, node.Schema)
		case node.IsList():
			require.NotNil(t, node.List, path.String())
			require.Positive(t, node.List.MaxRecords, "list %s must declare max records", path.String())
			require.LessOrEqual(t, node.List.MaxRecords, node.List.Capacity, path.String())

			records := make([]map[string]any, 0, node.List.MaxRecords)
			for range node.List.MaxRecords {
				records = append(records, draftGroup(t, node.List.Shape, kind, path, nil))
			}
			out[name] = records
			if caps != nil {
				caps[path.String()] = len(records)
			}
		case node.IsGroup():
			child := draftGroup(t, node.Children, kind, path, caps)
			if len(child) > 0 {
				out[name] = child
			}
		}
	}

	for name, members := range exclusive {
		out[name] = slices.Sorted(slices.Values(members))[0]
	}
	return out
}

func draftLeaf(out map[string]any, exclusive map[string][]string, name string, schema *template.FieldSchema) {
	if schema == nil || !schema.IsEditable {
		return
	}
	if schema.ExclusiveGroup != "" {
		exclusive[schema.ExclusiveGroup] = append(exclusive[schema.ExclusiveGroup], name)
		return
	}
	out[name] = draftValue(schema)
}

func draftValue(schema *template.FieldSchema) any {
	switch schema.Type {
	case fieldTypeText:
		return draftText(schema)
	case fieldTypeNumber:
		return schema.Default
	case fieldTypeBool:
		return false
	default:
		return ""
	}
}

func draftText(schema *template.FieldSchema) string {
	length, ok := draftTextRunesByLabel[schema.Label]
	if !ok {
		length = draftLongTextRunes
	}
	if schema.MaxLength != nil && *schema.MaxLength < length {
		length = *schema.MaxLength
	}
	return strings.Repeat(draftSentence, 3)[:length]
}
