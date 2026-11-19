package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/contextdocs"
)

const testAllowlist = "# tracked markdown\nAGENTS.md\ncontext/*.md\ndocs/legal/*.md\n"

func testRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, contextdocs.MarkdownAllowlistPath), []byte(testAllowlist), 0o644))
	return root
}

func initGit(t *testing.T, root string) {
	t.Helper()
	git(t, root, "init", "-q")
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	require.NoError(t, err, string(output))
}

func runCLI(t *testing.T, root, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"-root", root}, args...), strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func hookPayload(t *testing.T, cwd, file string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"tool_name":  "Write",
		"cwd":        cwd,
		"tool_input": map[string]string{"file_path": file, "content": "# Notes\n"},
	})
	require.NoError(t, err)
	return string(data)
}

func TestRunWithExplicitFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		wantCode int
		contains string
	}{
		{
			name:     "Happy path: every file is allowed",
			files:    []string{"AGENTS.md", "context/auth.md", "docs/legal/privacy-policy.en.md"},
			wantCode: 0,
			contains: "versioned allow-list",
		},
		{
			name:     "Error: new markdown file",
			files:    []string{"AGENTS.md", "docs/notes.md"},
			wantCode: 1,
			contains: `"docs/notes.md" is not allowed`,
		},
		{
			name:     "Edge: non-markdown files are ignored",
			files:    []string{"backend/cmd/main.go", "web/src/app.tsx"},
			wantCode: 0,
			contains: "versioned allow-list",
		},
		{
			name:     "Edge: node_modules is excluded",
			files:    []string{"node_modules/pkg/README.md"},
			wantCode: 0,
			contains: "versioned allow-list",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := testRepo(t)
			code, stdout, stderr := runCLI(t, root, "", tc.files...)
			assert.Equal(t, tc.wantCode, code, stderr)
			assert.Contains(t, stdout, tc.contains)
		})
	}
}

func TestRunFailureSaysTheAllowlistEditIsTheAuthorization(t *testing.T) {
	root := testRepo(t)
	code, stdout, stderr := runCLI(t, root, "", "notes.md")
	require.Equal(t, 1, code, stderr)
	assert.Contains(t, stdout, contextdocs.MarkdownAllowlistPath)
	assert.Contains(t, stdout, "editing the allow-list is the authorization")
}

func TestRunWithoutArgumentsChecksTrackedFiles(t *testing.T) {
	root := testRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "context", "auth.md"), []byte("# Auth\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "notes.md"), []byte("# Notes\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scratch.md"), []byte("# Scratch\n"), 0o644))
	initGit(t, root)
	git(t, root, "add", "context/auth.md", "docs/notes.md")
	git(t, root, "-c", "user.name=Legnaris", "-c", "user.email=test@legnaris.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "tracked markdown")

	code, stdout, stderr := runCLI(t, root, "")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stdout, "docs/notes.md")
	assert.NotContains(t, stdout, "scratch.md", "untracked and gitignored output must not be reported")

	require.NoError(t, os.WriteFile(filepath.Join(root, contextdocs.MarkdownAllowlistPath), []byte(testAllowlist+"docs/*.md\n"), 0o644))
	code, stdout, stderr = runCLI(t, root, "")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "versioned allow-list")
}

func TestRunRejectsMissingAllowlist(t *testing.T) {
	code, stdout, stderr := runCLI(t, t.TempDir(), "")
	assert.Equal(t, 2, code)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, contextdocs.MarkdownAllowlistPath)
}

func TestRunRejectsInvalidAllowlist(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, contextdocs.MarkdownAllowlistPath), []byte("../notes.md\n"), 0o644))
	code, stdout, stderr := runCLI(t, root, "")
	assert.Equal(t, 2, code)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, contextdocs.MarkdownAllowlistPath)
}

func TestRunHook(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		wantAsk  bool
		wantName string
	}{
		{name: "Happy path: edit to an allowed file stays silent", file: "context/auth.md"},
		{name: "Happy path: new allowed file stays silent", file: "docs/legal/new-policy.md"},
		{name: "Edge: non-markdown file stays silent", file: "backend/cmd/main.go"},
		{name: "Edge: node_modules file stays silent", file: "node_modules/pkg/README.md"},
		{name: "Error: new markdown file is not allowed", file: "notes.md", wantAsk: true, wantName: "notes.md"},
		{name: "Error: uppercase new markdown file is not allowed", file: "NOTES.MD", wantAsk: true, wantName: "NOTES.MD"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := testRepo(t)
			code, stdout, stderr := runCLI(t, root, hookPayload(t, root, tc.file), "-hook")
			require.Equal(t, 0, code, stderr)
			if !tc.wantAsk {
				assert.Empty(t, stdout)
				assert.Empty(t, stderr)
				return
			}

			var response hookResponse
			require.NoError(t, json.Unmarshal([]byte(stdout), &response))
			assert.Equal(t, "PreToolUse", response.HookSpecificOutput.HookEventName)
			assert.Equal(t, "ask", response.HookSpecificOutput.PermissionDecision)
			assert.Contains(t, response.HookSpecificOutput.PermissionDecisionReason, tc.wantName)
			assert.Contains(t, response.HookSpecificOutput.PermissionDecisionReason, contextdocs.MarkdownAllowlistPath)
		})
	}
}

func TestRunHookResolvesPathsAgainstTheToolCwd(t *testing.T) {
	root := testRepo(t)
	code, stdout, stderr := runCLI(t, root, hookPayload(t, filepath.Join(root, "web"), "notes.md"), "-hook")
	require.Equal(t, 0, code, stderr)

	var response hookResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &response))
	assert.Equal(t, "ask", response.HookSpecificOutput.PermissionDecision)
	assert.Contains(t, response.HookSpecificOutput.PermissionDecisionReason, "web/notes.md")
}

func TestRunHookStaysSilentOutsideTheRepository(t *testing.T) {
	root := testRepo(t)
	outside := filepath.Join(filepath.Dir(root), "notes.md")
	code, stdout, stderr := runCLI(t, root, hookPayload(t, root, outside), "-hook")
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stdout)
}

func TestRunHookStaysSilentForGitignoredOutput(t *testing.T) {
	root := testRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("test-results/\n"), 0o644))
	initGit(t, root)
	code, stdout, stderr := runCLI(t, root, hookPayload(t, root, "test-results/run.md"), "-hook")
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stdout)
}

func TestRunHookIgnoresMalformedPayloads(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		wantErr bool
	}{
		{name: "Error: invalid JSON", stdin: "{", wantErr: true},
		{name: "Edge: empty payload", stdin: "", wantErr: true},
		{name: "Edge: missing tool input", stdin: `{"tool_name":"Write","cwd":"/"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := testRepo(t)
			code, stdout, stderr := runCLI(t, root, tc.stdin, "-hook")
			assert.Equal(t, 0, code)
			assert.Empty(t, stdout)
			if tc.wantErr {
				assert.Contains(t, stderr, "markdown-check:")
				return
			}
			assert.Empty(t, stderr)
		})
	}
}
