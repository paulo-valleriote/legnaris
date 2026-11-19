package contextdocs_test

import (
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

func mustAllowlist(t *testing.T, content string) *contextdocs.MarkdownAllowlist {
	t.Helper()
	allowlist, err := contextdocs.ParseMarkdownAllowlist([]byte(content))
	require.NoError(t, err)
	return allowlist
}

func TestParseMarkdownAllowlist(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "Happy path: comments and blank lines are skipped",
			content:  "# tracked markdown\n\nAGENTS.md\ncontext/*.md\n",
			patterns: []string{"AGENTS.md", "context/*.md"},
		},
		{
			name:     "Edge: CRLF line endings and surrounding whitespace",
			content:  "  AGENTS.md \r\ncontext/*.md\r\n",
			patterns: []string{"AGENTS.md", "context/*.md"},
		},
		{
			name:     "Edge: duplicate patterns are kept",
			content:  "README.md\nREADME.md\n",
			patterns: []string{"README.md", "README.md"},
		},
		{
			name:    "Error: empty allow-list",
			content: "",
			wantErr: true,
		},
		{
			name:    "Error: comments only",
			content: "# nothing\n\n",
			wantErr: true,
		},
		{
			name:    "Error: absolute pattern",
			content: "/etc/notes.md\n",
			wantErr: true,
		},
		{
			name:    "Error: pattern escapes the repository",
			content: "../notes.md\n",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			allowlist, err := contextdocs.ParseMarkdownAllowlist([]byte(tc.content))
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, allowlist)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.patterns, allowlist.Patterns())
		})
	}
}

func TestMarkdownAllowlistAllows(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		file    string
		want    bool
	}{
		{name: "Happy path: exact path", pattern: "deploy/README.md", file: "deploy/README.md", want: true},
		{name: "Edge: exact path rejects another file", pattern: "deploy/README.md", file: "README.md", want: false},
		{name: "Happy path: single star stays in one segment", pattern: "context/*.md", file: "context/auth.md", want: true},
		{name: "Edge: single star does not cross directories", pattern: "context/*.md", file: "context/nested/auth.md", want: false},
		{name: "Happy path: double star crosses directories", pattern: "**/AGENTS.md", file: "backend/internal/AGENTS.md", want: true},
		{name: "Edge: double star still needs the directory separator", pattern: "**/AGENTS.md", file: "AGENTS.md", want: false},
		{name: "Happy path: directory prefix", pattern: ".opencode/**", file: ".opencode/agents/review/leaks.md", want: true},
		{name: "Edge: directory prefix requires a child", pattern: ".opencode/**", file: ".opencode", want: false},
		{name: "Happy path: nested skill markdown", pattern: ".claude/skills/**", file: ".claude/skills/jira-assistant/README.md", want: true},
		{name: "Edge: sibling directory is not matched", pattern: ".claude/skills/**", file: ".claude/settings.md", want: false},
		{name: "Edge: leading dot slash is normalized", pattern: "context/auth.md", file: "./context/auth.md", want: true},
		{name: "Edge: empty path", pattern: "README.md", file: "", want: false},
		{name: "Edge: traversal path", pattern: "**/*.md", file: "../notes.md", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			allowlist := mustAllowlist(t, tc.pattern+"\n")
			assert.Equal(t, tc.want, allowlist.Allows(tc.file))
		})
	}
}

func TestUnlistedMarkdown(t *testing.T) {
	allowlist := mustAllowlist(t, "AGENTS.md\n**/AGENTS.md\ncontext/*.md\nGLOSSARY.md\n")

	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name:  "Happy path: every markdown file is allowed",
			files: []string{"AGENTS.md", "backend/internal/AGENTS.md", "context/auth.md"},
			want:  []string{},
		},
		{
			name:  "Error: new markdown file outside the allow-list",
			files: []string{"AGENTS.md", "docs/notes.md"},
			want:  []string{"docs/notes.md"},
		},
		{
			name:  "Edge: non-markdown files are ignored",
			files: []string{"backend/cmd/markdown-check/main.go", "web/src/app.tsx"},
			want:  []string{},
		},
		{
			name:  "Edge: node_modules is excluded",
			files: []string{"node_modules/pkg/README.md", "web/node_modules/pkg/notes.md"},
			want:  []string{},
		},
		{
			name:  "Edge: duplicates and dot slash are normalized",
			files: []string{"./docs/notes.md", "docs/notes.md", " GLOSSARY.md "},
			want:  []string{"docs/notes.md"},
		},
		{
			name:  "Edge: empty and relative escape entries are dropped",
			files: []string{"", "   ", "../outside.md", "."},
			want:  []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, contextdocs.UnlistedMarkdown(allowlist, tc.files))
		})
	}
}

func TestUnlistedMarkdownWithoutAllowlist(t *testing.T) {
	assert.Empty(t, contextdocs.UnlistedMarkdown(nil, []string{"docs/notes.md"}))
}

func TestIsMarkdownFile(t *testing.T) {
	tests := []struct {
		name string
		file string
		want bool
	}{
		{name: "Happy path: lowercase extension", file: "docs/notes.md", want: true},
		{name: "Edge: uppercase extension", file: "NOTES.MD", want: true},
		{name: "Edge: mixed case extension", file: "notes.Md", want: true},
		{name: "Edge: other extension", file: "notes.mdx", want: false},
		{name: "Edge: no extension", file: "README", want: false},
		{name: "Edge: empty name", file: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, contextdocs.IsMarkdownFile(tc.file))
		})
	}
}

func TestIsExcludedMarkdownPath(t *testing.T) {
	tests := []struct {
		name string
		file string
		want bool
	}{
		{name: "Happy path: node_modules segment", file: "web/node_modules/pkg/README.md", want: true},
		{name: "Edge: allowed path", file: "context/auth.md", want: false},
		{name: "Edge: node_modules substring is not a segment", file: "docs/node_modules-notes.md", want: false},
		{name: "Edge: empty path", file: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, contextdocs.IsExcludedMarkdownPath(tc.file))
		})
	}
}

func TestRepoRelativePath(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name   string
		base   string
		target string
		want   string
		ok     bool
	}{
		{name: "Happy path: relative to the repository root", base: root, target: "docs/notes.md", want: "docs/notes.md", ok: true},
		{name: "Happy path: relative to the tool cwd", base: filepath.Join(root, "web"), target: "notes.md", want: "web/notes.md", ok: true},
		{name: "Happy path: absolute inside the repository", target: filepath.Join(root, "context", "auth.md"), want: "context/auth.md", ok: true},
		{name: "Edge: empty cwd falls back to the root", target: "notes.md", want: "notes.md", ok: true},
		{name: "Edge: empty target", base: root, target: "", ok: false},
		{name: "Edge: repository root itself", base: root, target: ".", ok: false},
		{name: "Edge: target outside the repository", base: root, target: "../notes.md", ok: false},
		{name: "Edge: absolute target outside the repository", target: filepath.Join(filepath.Dir(root), "notes.md"), ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			relative, ok := contextdocs.RepoRelativePath(root, tc.base, tc.target)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, relative)
		})
	}
}

func TestRepositoryAllowlistCoversTrackedMarkdown(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, contextdocs.MarkdownAllowlistPath))
	require.NoError(t, err)
	allowlist, err := contextdocs.ParseMarkdownAllowlist(data)
	require.NoError(t, err)

	output, err := exec.CommandContext(context.Background(), "git", "-C", root, "ls-files", "--", "*.md").Output()
	if err != nil {
		t.Skipf("git ls-files unavailable: %v", err)
	}
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.NotEmpty(t, files)
	assert.Empty(t, contextdocs.UnlistedMarkdown(allowlist, files))
}
