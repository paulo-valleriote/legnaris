package generation_test

import (
	"strings"
	"testing"

	"legnaris/backend/internal/generation"

	"github.com/stretchr/testify/assert"
)

func TestPromptVersion(t *testing.T) {
	first := generation.PromptVersion("You fill in a sheet.")
	second := generation.PromptVersion("You fill in a sheet.")
	changed := generation.PromptVersion("You fill in a sheet. Reply with JSON.")

	t.Run("Happy path: the same prompt text yields the same version", func(t *testing.T) {
		assert.Equal(t, first, second)
		assert.True(t, strings.HasPrefix(first, "v1+"))
		assert.Len(t, strings.TrimPrefix(first, "v1+"), 12)
	})
	t.Run("Edge: changing the prompt text changes the hash", func(t *testing.T) {
		assert.NotEqual(t, first, changed)
	})
	t.Run("Edge: an empty prompt still has a stable version", func(t *testing.T) {
		empty := generation.PromptVersion("")
		again := generation.PromptVersion("")
		assert.Equal(t, empty, again)
	})
}
