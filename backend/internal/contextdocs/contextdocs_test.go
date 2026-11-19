package contextdocs_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/contextdocs"
)

var expectedFiles = []struct {
	path  string
	title string
}{
	{"context/product.md", "Product"},
	{"context/architecture.md", "Architecture"},
	{"context/auth.md", "Auth"},
	{"context/sheets.md", "Sheets"},
	{"context/dice.md", "Dice"},
	{"context/generation.md", "Generation"},
	{"context/billing.md", "Billing"},
	{"context/campaigns.md", "Campaigns"},
	{"context/events-and-email.md", "Events and email"},
	{"context/admin.md", "Admin"},
	{"context/web-client.md", "Web client"},
	{"context/platform.md", "Platform"},
}

func TestParseDocument(t *testing.T) {
	full := "## Purpose\n## Why\n## Flows\n## Contracts\n## Code map\n## Tools & dependencies\n## Known limits\n"
	tests := []struct {
		name     string
		markdown string
		title    string
		sections []string
		wantErr  bool
	}{
		{
			name:     "Happy path: title and every section",
			markdown: "# Auth\n\n" + full,
			title:    "Auth",
			sections: contextdocs.Sections,
		},
		{
			name:     "Edge: headings inside a fenced block are ignored",
			markdown: "# Auth\n\n```\n## Not a section\n```\n\n" + full,
			title:    "Auth",
			sections: contextdocs.Sections,
		},
		{
			name:     "Edge: CRLF endings and leading blank lines",
			markdown: "\r\n\r\n# Auth\r\n\r\n" + strings.ReplaceAll(full, "\n", "\r\n"),
			title:    "Auth",
			sections: contextdocs.Sections,
		},
		{
			name:     "Error: empty document",
			markdown: "",
			wantErr:  true,
		},
		{
			name:     "Error: sections without a title",
			markdown: full,
			wantErr:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			document, err := contextdocs.ParseDocument(tc.markdown)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, document.Title)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.title, document.Title)
			assert.Equal(t, tc.sections, document.Sections)
		})
	}
}

func TestDocumentValidate(t *testing.T) {
	valid := contextdocs.Sections
	tests := []struct {
		name     string
		document contextdocs.Document
		wantErr  bool
	}{
		{
			name:     "Happy path: exact headings",
			document: contextdocs.Document{Title: "Auth", Sections: valid},
			wantErr:  false,
		},
		{
			name:     "Error: empty title",
			document: contextdocs.Document{Sections: valid},
			wantErr:  true,
		},
		{
			name:     "Error: missing a heading",
			document: contextdocs.Document{Title: "Auth", Sections: valid[:6]},
			wantErr:  true,
		},
		{
			name:     "Error: extra heading",
			document: contextdocs.Document{Title: "Auth", Sections: append(append([]string{}, valid...), "Notes")},
			wantErr:  true,
		},
		{
			name:     "Error: out of order",
			document: contextdocs.Document{Title: "Auth", Sections: []string{"Why", "Purpose", "Flows", "Contracts", "Code map", "Tools & dependencies", "Known limits"}},
			wantErr:  true,
		},
		{
			name:     "Error: duplicate heading",
			document: contextdocs.Document{Title: "Auth", Sections: []string{"Purpose", "Why", "Flows", "Contracts", "Code map", "Known limits", "Known limits"}},
			wantErr:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.document.Validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParsePathMap(t *testing.T) {
	valid := `
version: 1
context_files:
  - context/auth.md
paths:
  - target: backend/internal/auth
    kind: directory
    context: auth
env:
  - key: JWT_SECRET
    context: auth
`
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{name: "Happy path: minimal map", yaml: valid},
		{name: "Error: malformed YAML", yaml: "version: [", wantErr: true},
		{name: "Error: empty input", yaml: "", wantErr: true},
		{name: "Error: unknown field", yaml: strings.Replace(valid, "kind: directory", "knd: directory", 1), wantErr: true},
		{name: "Error: missing version", yaml: "context_files:\n  - context/auth.md\n", wantErr: true},
		{name: "Error: no context files", yaml: "version: 1\ncontext_files: []\n", wantErr: true},
		{name: "Error: duplicate context file", yaml: "version: 1\ncontext_files:\n  - context/auth.md\n  - context/auth.md\npaths:\n  - target: backend/internal/auth\n    kind: directory\n    context: auth\n", wantErr: true},
		{name: "Error: unknown kind", yaml: strings.Replace(valid, "kind: directory", "kind: symlink", 1), wantErr: true},
		{name: "Error: duplicate target", yaml: "version: 1\ncontext_files:\n  - context/auth.md\npaths:\n  - target: backend/internal/auth\n    kind: directory\n    context: auth\n  - target: backend/internal/auth\n    kind: directory\n    context: auth\n", wantErr: true},
		{name: "Error: duplicate env key", yaml: valid + "  - key: JWT_SECRET\n    context: auth\n", wantErr: true},
		{name: "Error: empty target", yaml: strings.Replace(valid, "target: backend/internal/auth", `target: ""`, 1), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pathMap, err := contextdocs.ParsePathMap([]byte(tc.yaml))
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, pathMap)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, pathMap)
			assert.Equal(t, 1, pathMap.Version)
			assert.Len(t, pathMap.Paths, 1)
			assert.Len(t, pathMap.Env, 1)
		})
	}
}

func TestNotMappedOnce(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		counts  map[string]int
		want    []string
	}{
		{
			name:    "Happy path: every target mapped once",
			targets: []string{"a", "b"},
			counts:  map[string]int{"a": 1, "b": 1},
			want:    nil,
		},
		{
			name:    "Edge: empty target list",
			targets: nil,
			counts:  map[string]int{"a": 1},
			want:    nil,
		},
		{
			name:    "Error: unmapped target",
			targets: []string{"a", "b"},
			counts:  map[string]int{"a": 1},
			want:    []string{"b"},
		},
		{
			name:    "Error: target mapped twice",
			targets: []string{"a", "b"},
			counts:  map[string]int{"a": 2, "b": 1},
			want:    []string{"a"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, contextdocs.NotMappedOnce(tc.targets, tc.counts))
		})
	}
}

func TestContextFilesFollowTemplate(t *testing.T) {
	root := repoRoot(t)
	pathMap := loadPathMap(t, root)
	require.Len(t, pathMap.ContextFiles, len(expectedFiles))

	for _, tc := range expectedFiles {
		assert.Contains(t, pathMap.ContextFiles, tc.path)
		t.Run(tc.path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, tc.path))
			require.NoError(t, err)
			document, err := contextdocs.ParseDocument(string(data))
			require.NoError(t, err)
			require.NoError(t, document.Validate())
			assert.Equal(t, tc.title, document.Title)
		})
	}
}

func TestPathMapTargetsExist(t *testing.T) {
	root := repoRoot(t)
	pathMap := loadPathMap(t, root)

	for _, entry := range pathMap.Paths {
		t.Run(entry.Target, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(root, entry.Target))
			require.NoError(t, err, "mapped target must exist")
			if entry.Kind == contextdocs.KindDirectory {
				assert.True(t, info.IsDir(), "mapped target must be a directory")
			} else {
				assert.False(t, info.IsDir(), "mapped target must be a file")
			}
			assert.Contains(t, pathMap.ContextFiles, "context/"+entry.Context+".md")
		})
	}
	for _, entry := range pathMap.Env {
		assert.Contains(t, pathMap.ContextFiles, "context/"+entry.Context+".md", "env key %s", entry.Key)
	}
}

func TestPathMapCoversEveryFolder(t *testing.T) {
	root := repoRoot(t)
	pathMap := loadPathMap(t, root)
	counts := contextdocs.CountTargets(pathMap.Paths)

	tests := []struct {
		name     string
		relative string
	}{
		{name: "backend internal packages", relative: "backend/internal"},
		{name: "backend shared packages", relative: "backend/shared"},
		{name: "web src folders", relative: "web/src"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names, err := contextdocs.Subdirectories(filepath.Join(root, tc.relative))
			require.NoError(t, err)
			require.NotEmpty(t, names)

			targets := make([]string, 0, len(names))
			for _, name := range names {
				targets = append(targets, tc.relative+"/"+name)
			}
			assert.Empty(t, contextdocs.NotMappedOnce(targets, counts), "every current folder must be mapped exactly once")
		})
	}

	for _, entry := range pathMap.Paths {
		if entry.Kind != contextdocs.KindDirectory {
			continue
		}
		for _, prefix := range []string{"backend/internal/", "backend/shared/", "web/src/"} {
			if !strings.HasPrefix(entry.Target, prefix) {
				continue
			}
			info, err := os.Stat(filepath.Join(root, entry.Target))
			require.NoError(t, err)
			assert.True(t, info.IsDir(), "%s must be a current folder", entry.Target)
		}
	}
}

func TestIndexLinksResolve(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, contextdocs.IndexPath))
	require.NoError(t, err)

	links := markdownLinks(string(data))
	require.NotEmpty(t, links)
	for _, link := range links {
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "mailto:") {
			continue
		}
		target, _, _ := strings.Cut(link, "#")
		if target == "" {
			continue
		}
		_, err := os.Stat(filepath.Join(root, target))
		require.NoError(t, err, "index link %q must resolve", link)
	}
	assert.Contains(t, links, contextdocs.MapPath)
	for _, tc := range expectedFiles {
		assert.Contains(t, links, tc.path)
	}
}

func TestPathMapCoversEnvKeys(t *testing.T) {
	root := repoRoot(t)
	pathMap := loadPathMap(t, root)
	keys := make([]string, 0, len(pathMap.Env))
	for _, entry := range pathMap.Env {
		keys = append(keys, entry.Key)
	}
	counts := make(map[string]int, len(keys))
	for _, key := range keys {
		counts[key]++
	}

	declared := append(envKeysFromGo(t, filepath.Join(root, "backend/shared/env/env.go")), viteEnvKeys(t, filepath.Join(root, "web/src/vite-env.d.ts"))...)
	sort.Strings(declared)
	assert.Empty(t, contextdocs.NotMappedOnce(declared, counts), "every declared env key must be mapped exactly once")

	sort.Strings(keys)
	assert.Equal(t, declared, keys, "the env map must not carry stale keys")
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found above the test working directory")
		dir = parent
	}
}

func loadPathMap(t *testing.T, root string) *contextdocs.PathMap {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, contextdocs.MapPath))
	require.NoError(t, err)
	pathMap, err := contextdocs.ParsePathMap(data)
	require.NoError(t, err)
	return pathMap
}

func markdownLinks(markdown string) []string {
	matches := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`).FindAllStringSubmatch(markdown, -1)
	links := make([]string, 0, len(matches))
	for _, match := range matches {
		links = append(links, match[1])
	}
	return links
}

func envKeysFromGo(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	matchers := []*regexp.Regexp{
		regexp.MustCompile(`lookup[A-Za-z]*Env\("([A-Z][A-Z0-9_]+)"`),
		regexp.MustCompile(`key[A-Za-z]+\s*=\s*"([A-Z][A-Z0-9_]+)"`),
	}
	keys := map[string]struct{}{}
	for _, matcher := range matchers {
		for _, match := range matcher.FindAllStringSubmatch(string(data), -1) {
			keys[match[1]] = struct{}{}
		}
	}
	require.NotEmpty(t, keys)
	return sortedSet(keys)
}

func viteEnvKeys(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	keys := map[string]struct{}{}
	for _, match := range regexp.MustCompile(`\bVITE_[A-Z0-9_]+`).FindAllString(string(data), -1) {
		keys[match] = struct{}{}
	}
	return sortedSet(keys)
}

func sortedSet(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
