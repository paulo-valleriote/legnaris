package llmconsent_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"legnaris/backend/internal/llmconsent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionMatchesDisclosure(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	copied, err := os.ReadFile(filepath.Join(dir, "llm-consent.json"))
	require.NoError(t, err)

	var disclosure struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(copied, &disclosure))
	assert.Equal(t, "3", llmconsent.Version, "the consent version must stay unchanged")
	assert.Equal(t, llmconsent.Version, disclosure.Version, "backend constant must match the copied disclosure")

	source := filepath.Join(dir, "..", "..", "..", "docs", "legal", "llm-consent.json")
	original, err := os.ReadFile(source)
	if os.IsNotExist(err) {
		t.Skip("docs/legal/llm-consent.json is not in this checkout")
	}
	require.NoError(t, err)
	assert.True(t, bytes.Equal(copied, original), "backend copy of llm-consent.json has drifted from docs/legal; recopy it")
}
