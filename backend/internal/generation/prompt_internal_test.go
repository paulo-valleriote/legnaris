package generation

import (
	"strings"
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemPrompt_EmptyGameSystemBrief(t *testing.T) {
	systems, err := template.BuildGameSystems([]template.GameSystem{{
		ID:                 "custom",
		Name:               "Custom",
		CompatibilityLabel: "Custom compatible",
		Notice:             template.GameSystemNotice{Label: "Custom", Path: "/custom"},
	}})
	require.NoError(t, err)

	tmpl := &template.Template{ID: "custom_v1", GameSystemID: "custom", PromptGuidance: "Custom guidance."}

	prompt, err := systemPrompt(tmpl, plan.KindCharacter, systems)
	require.NoError(t, err)

	assert.NotContains(t, prompt, "game system is")
	assert.NotContains(t, prompt, "\n\n\n")
	assert.Contains(t, prompt, "Custom guidance.")
	assert.Contains(t, prompt, "\n\nFields:\n")
}

func TestSystemPrompt_LayerOrder(t *testing.T) {
	tmpl := &template.Template{
		ID:             "custom_v1",
		GameSystemID:   template.GameSystemFifth,
		PromptGuidance: "Custom guidance.",
	}

	prompt, err := systemPrompt(tmpl, plan.KindCharacter, template.DefaultGameSystems())
	require.NoError(t, err)

	preamble := strings.Index(prompt, "You fill in the values of")
	brief := strings.Index(prompt, "fifth edition, as written in SRD 5.2.1")
	guidance := strings.Index(prompt, "Custom guidance.")
	fields := strings.Index(prompt, "Fields:")

	require.NotEqual(t, -1, preamble)
	require.NotEqual(t, -1, brief)
	require.NotEqual(t, -1, guidance)
	require.NotEqual(t, -1, fields)
	assert.Less(t, preamble, brief)
	assert.Less(t, brief, guidance)
	assert.Less(t, guidance, fields)
}
