package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countKind(findings []finding, kind string) int {
	count := 0
	for _, entry := range findings {
		if entry.kind == kind {
			count++
		}
	}
	return count
}

func matchesOf(findings []finding) []string {
	matches := make([]string, 0, len(findings))
	for _, entry := range findings {
		matches = append(matches, entry.match)
	}
	return matches
}

func TestScanText(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		wantDiacritic int
		wantStopword  int
		wantMatches   []string
	}{
		{
			name: "Happy path: English log text and identifiers are clean",
			text: "log.Error(ctx, \"user not found\")\nconst sheetLimit = 3",
		},
		{
			name: "Happy path: accented letters outside the pt-BR set are ignored",
			text: "Grüße naïve",
		},
		{
			name:          "Error: a pt-BR log string is reported",
			text:          "log.Error(ctx, \"usuário não encontrado\")",
			wantDiacritic: 2,
			wantStopword:  2,
			wantMatches:   []string{"usuário", "não"},
		},
		{
			name:          "Error: uppercase diacritics are reported",
			text:          "const AÇÃO = 1",
			wantDiacritic: 2,
			wantMatches:   []string{"Ç", "Ã"},
		},
		{
			name:          "Error: the crase accent is reported",
			text:          "sheet opens at às 10h",
			wantDiacritic: 1,
			wantMatches:   []string{"à"},
		},
		{
			name:          "Error: the uppercase crase accent is reported",
			text:          "À vista",
			wantDiacritic: 1,
			wantMatches:   []string{"À"},
		},
		{
			name:         "Error: an unaccented stopword is reported",
			text:         "const senha = 1",
			wantStopword: 1,
			wantMatches:  []string{"senha"},
		},
		{
			name:          "Error: adjacent stopwords are all reported",
			text:          "usuário não você",
			wantDiacritic: 3,
			wantStopword:  3,
			wantMatches:   []string{"usuário", "não", "você"},
		},
		{
			name:          "Error: a two-word stopword is reported",
			text:          "erro ao abrir a ficha",
			wantDiacritic: 0,
			wantStopword:  2,
			wantMatches:   []string{"erro ao", "ficha"},
		},
		{
			name: "Edge: empty text has no findings",
			text: "",
		},
		{
			name: "Edge: a stopword inside a longer word is not reported",
			text: "personagens e senhas",
		},
		{
			name:          "Edge: a stopword next to punctuation is reported",
			text:          "\"não, senha!\"",
			wantDiacritic: 1,
			wantStopword:  2,
			wantMatches:   []string{"não", "senha"},
		},
		{
			name:          "Edge: CRLF line endings do not hide a finding",
			text:          "clean line\r\nusuário\r\n",
			wantDiacritic: 1,
			wantStopword:  1,
			wantMatches:   []string{"usuário"},
		},
		{
			name:         "Edge: a finding without a trailing newline is reported",
			text:         "personagem",
			wantStopword: 1,
			wantMatches:  []string{"personagem"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := scanText("example.go", tc.text)

			assert.Len(t, findings, tc.wantDiacritic+tc.wantStopword)
			assert.Equal(t, tc.wantDiacritic, countKind(findings, "diacritic"))
			assert.Equal(t, tc.wantStopword, countKind(findings, "stopword"))
			for _, match := range tc.wantMatches {
				assert.Contains(t, matchesOf(findings), match)
			}
		})
	}
}

func TestScanTextReportsPosition(t *testing.T) {
	findings := scanText("example.go", "first\nsecond usuário")

	require.Len(t, findings, 2)
	assert.Equal(t, 2, findings[0].line)
	assert.Equal(t, 8, findings[0].column)
	assert.Equal(t, "usuário", findings[0].match)
	assert.Equal(t, 11, findings[1].column)
	assert.Equal(t, "á", findings[1].match)
}

func TestLoadAllowlist(t *testing.T) {
	t.Run("Happy path: comments, blanks and surrounding spaces are handled", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "allowlist.txt")
		content := "# a comment\n\n  backend/allowed.go  \n\tweb/src/l10n/strings.ts\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

		allow, err := loadAllowlist(path)

		require.NoError(t, err)
		assert.Len(t, allow, 2)
		assert.Contains(t, allow, "backend/allowed.go")
		assert.Contains(t, allow, "web/src/l10n/strings.ts")
	})

	t.Run("Edge: an empty allowlist file allows nothing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "allowlist.txt")
		require.NoError(t, os.WriteFile(path, nil, 0o600))

		allow, err := loadAllowlist(path)

		require.NoError(t, err)
		assert.Empty(t, allow)
	})

	t.Run("Error: a missing allowlist file fails", func(t *testing.T) {
		_, err := loadAllowlist(filepath.Join(t.TempDir(), "missing.txt"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read allowlist")
	})

	t.Run("Error: an absolute path is rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "allowlist.txt")
		require.NoError(t, os.WriteFile(path, []byte("/etc/passwd\n"), 0o600))

		_, err := loadAllowlist(path)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "repository-relative")
	})

	t.Run("Error: a parent path is rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "allowlist.txt")
		require.NoError(t, os.WriteFile(path, []byte("../outside.go\n"), 0o600))

		_, err := loadAllowlist(path)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "repository-relative")
	})
}

func TestIsSource(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "Happy path: a Go source is scanned", path: "backend/cmd/api.go", want: true},
		{name: "Happy path: a TypeScript source is scanned", path: "web/src/api/client.ts", want: true},
		{name: "Happy path: a TSX source is scanned", path: "web/src/screens/Sheet.tsx", want: true},
		{name: "Happy path: a non-source file under l10n is scanned", path: "web/src/l10n/strings.json", want: true},
		{name: "Edge: the extension match is case-insensitive", path: "backend/CMD/API.GO", want: true},
		{name: "Edge: docs are not scanned", path: "docs/TDD.md", want: false},
		{name: "Edge: a JSON file outside l10n is not scanned", path: "web/src/assets/data.json", want: false},
		{name: "Edge: the l10n directory name alone is not a source", path: "web/src/l10n", want: false},
		{name: "Edge: a path that merely prefixes l10n is not scanned", path: "web/src/l10n-extra/data.json", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isSource(tc.path))
		})
	}
}

func TestFilterSources(t *testing.T) {
	files := []string{"backend/main.go", "docs/TDD.md", "web/src/l10n/strings.ts", "README.md"}

	sources := filterSources(files)

	assert.Equal(t, []string{"backend/main.go", "web/src/l10n/strings.ts"}, sources)
}

func TestScanFiles(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		full := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}

	write("backend/bad.go", "log.Error(ctx, \"usuário não encontrado\")\n")
	write("backend/allowed.go", "log.Error(ctx, \"usuário não encontrado\")\n")
	write("backend/clean.go", "log.Error(ctx, \"user not found\")\n")
	write("backend/binary.go", string([]byte{0xff, 0xfe, 0x00}))
	write("web/src/l10n/strings.ts", "export const message = 'senha';\n")

	allow := map[string]struct{}{"backend/allowed.go": {}}
	files := []string{
		"backend/bad.go",
		"backend/allowed.go",
		"backend/clean.go",
		"backend/binary.go",
		"backend/missing.go",
		"web/src/l10n/strings.ts",
	}

	findings, err := scanFiles(root, files, allow)

	require.NoError(t, err)
	paths := make(map[string]int)
	for _, entry := range findings {
		paths[entry.path]++
	}
	assert.NotZero(t, paths["backend/bad.go"])
	assert.NotZero(t, paths["web/src/l10n/strings.ts"])
	assert.NotContains(t, paths, "backend/allowed.go")
	assert.NotContains(t, paths, "backend/clean.go")
	assert.NotContains(t, paths, "backend/binary.go")
	assert.Len(t, findings, paths["backend/bad.go"]+paths["web/src/l10n/strings.ts"])
	assert.True(t, sortedByPosition(findings))
}

func sortedByPosition(findings []finding) bool {
	for index := 1; index < len(findings); index++ {
		previous := findings[index-1]
		current := findings[index]
		if previous.path > current.path {
			return false
		}
		if previous.path == current.path && previous.line > current.line {
			return false
		}
	}
	return true
}

func TestRunReportsArguments(t *testing.T) {
	t.Run("Error: unexpected arguments are rejected", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := run([]string{"extra"}, &stdout, &stderr)

		assert.Equal(t, exitFailure, code)
		assert.Contains(t, stderr.String(), "unexpected arguments")
	})

	t.Run("Error: an unknown flag is rejected", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := run([]string{"-nope"}, &stdout, &stderr)

		assert.Equal(t, exitFailure, code)
		assert.Contains(t, stderr.String(), "-nope")
	})

	t.Run("Error: a missing allowlist is reported", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := run([]string{"-root", t.TempDir()}, &stdout, &stderr)

		assert.Equal(t, exitFailure, code)
		assert.Contains(t, stderr.String(), "read allowlist")
	})
}

func TestRunOnRepository(t *testing.T) {
	root := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
		require.NoError(t, command.Run())
	}
	runGit("init", "-q")

	allowlist := filepath.Join(root, defaultAllowlistPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(allowlist), 0o755))
	require.NoError(t, os.WriteFile(allowlist, []byte("backend/allowed.go\n"), 0o600))

	source := filepath.Join(root, "backend", "main.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
	require.NoError(t, os.WriteFile(source, []byte("log.Info(ctx, \"user not found\")\n"), 0o600))
	allowed := filepath.Join(root, "backend", "allowed.go")
	require.NoError(t, os.WriteFile(allowed, []byte("log.Error(ctx, \"usuário\")\n"), 0o600))
	runGit("add", "backend/main.go", "backend/allowed.go")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-root", root, "-allowlist", allowlist}, &stdout, &stderr)

	assert.Equal(t, exitClean, code, stderr.String())
	assert.Contains(t, stdout.String(), "2 source file(s) clean")

	require.NoError(t, os.WriteFile(source, []byte("log.Error(ctx, \"não encontrado\")\n"), 0o600))
	runGit("add", "backend/main.go")

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-root", root, "-allowlist", allowlist}, &stdout, &stderr)

	assert.Equal(t, exitFindings, code, stderr.String())
	assert.Contains(t, stdout.String(),
		"backend/main.go:1:17: stopword \"não\"\nbackend/main.go:1:18: diacritic \"ã\"\n")
	assert.NotContains(t, stdout.String(), "backend/allowed.go")
}

func TestRepositoryIsClean(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	_, err := os.Stat(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Skip("repository root is not reachable from the package directory")
	}

	findings, scanned, err := scanRepository(root, filepath.Join(root, defaultAllowlistPath))

	require.NoError(t, err)
	require.Positive(t, scanned)
	for _, entry := range findings {
		t.Errorf("non-English text: %s", entry.String())
	}
}

func TestFindingString(t *testing.T) {
	entry := finding{path: "backend/main.go", line: 3, column: 7, kind: "stopword", match: "não"}

	assert.Equal(t, `backend/main.go:3:7: stopword "não"`, entry.String())
	assert.True(t, strings.HasPrefix(entry.String(), "backend/main.go:"))
}
