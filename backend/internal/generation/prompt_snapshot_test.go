package generation_test

import (
	"os"
	"path/filepath"
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFantasyV1PromptSnapshot(t *testing.T) {
	tmpl := fantasyV1Fixture(t)

	tests := []struct {
		name string
		kind plan.SheetKind
	}{
		{name: "Happy path: the frozen character prompt is byte-identical", kind: plan.KindCharacter},
		{name: "Happy path: the frozen NPC prompt is byte-identical", kind: plan.KindNPC},
		{name: "Happy path: the frozen enemy prompt is byte-identical", kind: plan.KindEnemy},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prompt, err := generation.SystemPrompt(tmpl, tc.kind)
			require.NoError(t, err)

			path := filepath.Join("testdata", "fantasy_v1_"+string(tc.kind)+"_prompt.golden")
			want, err := os.ReadFile(path)
			require.NoError(t, err)

			assert.Equal(t, string(want), prompt)
		})
	}
}
