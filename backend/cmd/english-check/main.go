package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultAllowlistPath = "scripts/ci/english-allowlist.txt"
	l10nDirectory        = "web/src/l10n/"
	exitClean            = 0
	exitFindings         = 1
	exitFailure          = 2
)

var diacritics = map[rune]struct{}{
	'ã': {}, 'õ': {}, 'ç': {}, 'á': {}, 'é': {}, 'í': {}, 'ó': {}, 'ú': {}, 'â': {}, 'ê': {}, 'ô': {}, 'à': {},
	'Ã': {}, 'Õ': {}, 'Ç': {}, 'Á': {}, 'É': {}, 'Í': {}, 'Ó': {}, 'Ú': {}, 'Â': {}, 'Ê': {}, 'Ô': {}, 'À': {},
}

var stopwords = []string{
	"usuário",
	"não",
	"você",
	"erro ao",
	"senha",
	"ficha",
	"personagem",
	"então",
	"também",
	"obrigatório",
}

type stopwordPattern struct {
	expression *regexp.Regexp
}

type finding struct {
	path   string
	line   int
	column int
	kind   string
	match  string
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d:%d: %s %q", f.path, f.line, f.column, f.kind, f.match)
}

func newStopwordPatterns(words []string) []stopwordPattern {
	patterns := make([]stopwordPattern, 0, len(words))
	for _, word := range words {
		patterns = append(patterns, stopwordPattern{expression: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(word))})
	}
	return patterns
}

var stopwordPatterns = newStopwordPatterns(stopwords)

func isSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ts", ".tsx":
		return true
	}
	return strings.HasPrefix(strings.ToLower(path), l10nDirectory)
}

func loadAllowlist(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read allowlist: %w", err)
	}
	allow := make(map[string]struct{})
	for number, line := range strings.Split(string(data), "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		if filepath.IsAbs(entry) || strings.HasPrefix(entry, "../") || entry == ".." {
			return nil, fmt.Errorf("allowlist line %d: %q must be a repository-relative path", number+1, entry)
		}
		allow[filepath.ToSlash(entry)] = struct{}{}
	}
	return allow, nil
}

func trackedFiles(root string) ([]string, error) {
	command := exec.CommandContext(context.Background(), "git", "-C", root, "ls-files", "-z")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	files := make([]string, 0, 64)
	for _, name := range strings.Split(string(output), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}

func filterSources(files []string) []string {
	sources := make([]string, 0, len(files))
	for _, file := range files {
		if isSource(file) {
			sources = append(sources, file)
		}
	}
	return sources
}

func boundaryAt(line string, start, end int) bool {
	if start > 0 {
		previous, _ := utf8.DecodeLastRuneInString(line[:start])
		if unicode.IsLetter(previous) {
			return false
		}
	}
	if end < len(line) {
		next, _ := utf8.DecodeRuneInString(line[end:])
		if unicode.IsLetter(next) {
			return false
		}
	}
	return true
}

func stopwordFindings(path string, number int, line string) []finding {
	var findings []finding
	for _, pattern := range stopwordPatterns {
		offset := 0
		for offset < len(line) {
			location := pattern.expression.FindStringIndex(line[offset:])
			if location == nil {
				break
			}
			start := offset + location[0]
			end := offset + location[1]
			if boundaryAt(line, start, end) {
				findings = append(findings, finding{
					path:   path,
					line:   number,
					column: utf8.RuneCountInString(line[:start]) + 1,
					kind:   "stopword",
					match:  line[start:end],
				})
			}
			offset = end
		}
	}
	return findings
}

func diacriticFindings(path string, number int, line string) []finding {
	var findings []finding
	for column, character := range []rune(line) {
		if _, ok := diacritics[character]; !ok {
			continue
		}
		findings = append(findings, finding{
			path:   path,
			line:   number,
			column: column + 1,
			kind:   "diacritic",
			match:  string(character),
		})
	}
	return findings
}

func scanText(path, text string) []finding {
	var findings []finding
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		findings = append(findings, diacriticFindings(path, index+1, line)...)
		findings = append(findings, stopwordFindings(path, index+1, line)...)
	}
	sortFindings(findings)
	return findings
}

func scanFiles(root string, files []string, allow map[string]struct{}) ([]finding, error) {
	var findings []finding
	for _, name := range files {
		slashName := filepath.ToSlash(name)
		if _, ok := allow[slashName]; ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(slashName)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", slashName, err)
		}
		if !utf8.Valid(data) {
			continue
		}
		findings = append(findings, scanText(slashName, string(data))...)
	}
	return findings, nil
}

func sortFindings(findings []finding) {
	sort.Slice(findings, func(left, right int) bool {
		if findings[left].path != findings[right].path {
			return findings[left].path < findings[right].path
		}
		if findings[left].line != findings[right].line {
			return findings[left].line < findings[right].line
		}
		if findings[left].column != findings[right].column {
			return findings[left].column < findings[right].column
		}
		return findings[left].kind < findings[right].kind
	})
}

func scanRepository(root, allowlistPath string) ([]finding, int, error) {
	allow, err := loadAllowlist(allowlistPath)
	if err != nil {
		return nil, 0, err
	}
	files, err := trackedFiles(root)
	if err != nil {
		return nil, 0, err
	}
	sources := filterSources(files)
	findings, err := scanFiles(root, sources, allow)
	if err != nil {
		return nil, 0, err
	}
	return findings, len(sources), nil
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("english-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	allowlist := flags.String("allowlist", "", "allowlist file (default <root>/"+defaultAllowlistPath+")")
	if err := flags.Parse(args); err != nil {
		return exitFailure
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "english-check: unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return exitFailure
	}
	path := *allowlist
	if path == "" {
		path = filepath.Join(*root, defaultAllowlistPath)
	}
	findings, scanned, err := scanRepository(*root, path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "english-check: %v\n", err)
		return exitFailure
	}
	for _, entry := range findings {
		_, _ = fmt.Fprintln(stdout, entry.String())
	}
	if len(findings) > 0 {
		_, _ = fmt.Fprintf(stdout, "english-check: %d finding(s); allow-list a file only when it must hold non-English text by design (%s)\n", len(findings), path)
		return exitFindings
	}
	_, _ = fmt.Fprintf(stdout, "english-check: %d source file(s) clean\n", scanned)
	return exitClean
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
