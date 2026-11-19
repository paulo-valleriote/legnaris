package contextdocs

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const MarkdownAllowlistPath = "context/markdown-allowlist.txt"

var excludedMarkdownSegments = map[string]struct{}{
	"node_modules": {},
}

type MarkdownAllowlist struct {
	patterns []string
	matchers []*regexp.Regexp
}

type HookInput struct {
	ToolName  string        `json:"tool_name"`
	CWD       string        `json:"cwd"`
	ToolInput HookToolInput `json:"tool_input"`
}

type HookToolInput struct {
	FilePath string `json:"file_path"`
}

func ParseMarkdownAllowlist(data []byte) (*MarkdownAllowlist, error) {
	allowlist := &MarkdownAllowlist{}
	for index, line := range strings.Split(string(data), "\n") {
		pattern := strings.TrimSpace(line)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		if strings.HasPrefix(pattern, "/") {
			return nil, fmt.Errorf("line %d: pattern %q must be repository-relative", index+1, pattern)
		}
		if hasParentSegment(pattern) {
			return nil, fmt.Errorf("line %d: pattern %q must not leave the repository", index+1, pattern)
		}
		allowlist.patterns = append(allowlist.patterns, pattern)
		allowlist.matchers = append(allowlist.matchers, compileMarkdownGlob(pattern))
	}
	if len(allowlist.patterns) == 0 {
		return nil, errors.New("allow-list has no patterns")
	}
	return allowlist, nil
}

func (a *MarkdownAllowlist) Patterns() []string {
	if a == nil {
		return nil
	}
	return append([]string(nil), a.patterns...)
}

func (a *MarkdownAllowlist) Allows(file string) bool {
	if a == nil {
		return false
	}
	normalized := normalizeRepoPath(file)
	if normalized == "" {
		return false
	}
	for _, matcher := range a.matchers {
		if matcher.MatchString(normalized) {
			return true
		}
	}
	return false
}

func UnlistedMarkdown(allowlist *MarkdownAllowlist, files []string) []string {
	if allowlist == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(files))
	unlisted := make([]string, 0, len(files))
	for _, file := range files {
		normalized := normalizeRepoPath(file)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		if !IsMarkdownFile(normalized) || IsExcludedMarkdownPath(normalized) {
			continue
		}
		if allowlist.Allows(normalized) {
			continue
		}
		unlisted = append(unlisted, normalized)
	}
	sort.Strings(unlisted)
	return unlisted
}

func IsMarkdownFile(file string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(file)), ".md")
}

func IsExcludedMarkdownPath(file string) bool {
	normalized := normalizeRepoPath(file)
	if normalized == "" {
		return false
	}
	for _, segment := range strings.Split(normalized, "/") {
		if _, ok := excludedMarkdownSegments[segment]; ok {
			return true
		}
	}
	return false
}

func RepoRelativePath(root, base, target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(target) {
		workspace := strings.TrimSpace(base)
		if workspace == "" {
			workspace = rootPath
		} else if !filepath.IsAbs(workspace) {
			workspace = filepath.Join(rootPath, workspace)
		}
		target = filepath.Join(workspace, target)
	}
	relative, err := filepath.Rel(rootPath, target)
	if err != nil {
		return "", false
	}
	normalized := normalizeRepoPath(relative)
	if normalized == "" {
		return "", false
	}
	return normalized, true
}

func normalizeRepoPath(file string) string {
	file = strings.TrimSpace(filepath.ToSlash(file))
	if file == "" {
		return ""
	}
	file = path.Clean(file)
	file = strings.TrimPrefix(file, "./")
	if file == "." || file == "" || strings.HasPrefix(file, "../") || file == ".." {
		return ""
	}
	return file
}

func hasParentSegment(pattern string) bool {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func compileMarkdownGlob(pattern string) *regexp.Regexp {
	var builder strings.Builder
	builder.WriteString("^")
	for index := 0; index < len(pattern); index++ {
		switch pattern[index] {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				builder.WriteString(".*")
				index++
				continue
			}
			builder.WriteString("[^/]*")
		case '?':
			builder.WriteString("[^/]")
		default:
			builder.WriteString(regexp.QuoteMeta(pattern[index : index+1]))
		}
	}
	builder.WriteString("$")
	return regexp.MustCompile(builder.String())
}
