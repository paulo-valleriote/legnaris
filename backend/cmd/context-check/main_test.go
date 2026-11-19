package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/contextdocs"
)

const testPathMap = `version: 1
context_files:
  - context/architecture.md
  - context/platform.md
paths:
  - target: web/package.json
    kind: file
    context: platform
  - target: backend/internal/contextdocs
    kind: directory
    context: architecture
env:
  - key: PORT
    context: platform
  - key: JWT_SECRET
    context: architecture
`

func testRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, contextdocs.MapPath), []byte(testPathMap), 0o644))
	return root
}

func runCLI(t *testing.T, root, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"-root", root}, args...), strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		files      []string
		stdin      string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{
			name:       "Happy path: owning context touched",
			files:      []string{"web/package.json", "context/platform.md"},
			wantCode:   0,
			wantOutput: "Every mapped change has a context update.",
		},
		{
			name:       "Error: owning context missing",
			files:      []string{"web/package.json"},
			wantCode:   1,
			wantOutput: "Missing context updates",
		},
		{
			name:       "Happy path: exemption label",
			files:      []string{"web/package.json"},
			args:       []string{"-label", contextdocs.LabelNoContextChange},
			wantCode:   0,
			wantOutput: "Skipped",
		},
		{
			name:       "Happy path: unchanged tests",
			files:      []string{"backend/internal/contextdocs/check_test.go"},
			wantCode:   0,
			wantOutput: "No mapped area changed",
		},
		{
			name:       "Happy path: files arrive on stdin",
			stdin:      "web/package.json\ncontext/platform.md\n",
			wantCode:   0,
			wantOutput: "Every mapped change has a context update.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := testRepo(t)
			args := append([]string{}, tc.args...)
			args = append(args, tc.files...)
			code, stdout, stderr := runCLI(t, root, tc.stdin, args...)
			assert.Equal(t, tc.wantCode, code, stderr)
			assert.Contains(t, stdout, tc.wantOutput)
		})
	}
}

func TestRunWritesSummaryFile(t *testing.T) {
	root := testRepo(t)
	summary := filepath.Join(t.TempDir(), "summary.md")
	code, _, stderr := runCLI(t, root, "web/package.json\n", "-summary", summary)
	require.Equal(t, 1, code, stderr)

	data, err := os.ReadFile(summary)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Missing context updates")
	assert.Contains(t, string(data), "context/platform.md")
}

func TestRunRejectsInvalidPathMap(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, contextdocs.MapPath), []byte("version: ["), 0o644))

	code, _, stderr := runCLI(t, root, "web/package.json\n")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, contextdocs.MapPath)
}

func TestRunRejectsMissingPathMap(t *testing.T) {
	code, _, stderr := runCLI(t, t.TempDir(), "web/package.json\n")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, contextdocs.MapPath)
}

func TestRunDetectsMakefileEnvKeyChange(t *testing.T) {
	root := testRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte("APP_ENV_KEYS = PORT\n"), 0o644))
	git(t, root, "init", "-q")
	git(t, root, "add", "Makefile")
	git(t, root, "-c", "user.name=Legnaris", "-c", "user.email=test@legnaris.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base")
	require.NoError(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte("APP_ENV_KEYS = PORT JWT_SECRET\n"), 0o644))

	code, stdout, stderr := runCLI(t, root, "", "-base", "HEAD", "Makefile")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stdout, "env key added: JWT_SECRET")
	assert.Contains(t, stdout, "context/architecture.md")

	code, stdout, stderr = runCLI(t, root, "", "-base", "HEAD", "Makefile", "context/architecture.md")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "Every mapped change has a context update.")
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	output, err := exec.CommandContext(context.Background(), "git", append([]string{"-C", root}, args...)...).CombinedOutput()
	require.NoError(t, err, string(output))
}
