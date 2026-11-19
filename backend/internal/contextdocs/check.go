package contextdocs

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

const (
	LabelNoContextChange = "no-context-change"
	ContextDirectory     = "context/"
	ArchitectureContext  = "architecture"
	PlatformContext      = "platform"
)

var triggerRoots = []string{"backend/internal/", "backend/shared/", "web/src/"}

var testPrefixes = []string{"web/src/test/", "web/e2e/", "web/tests/", "testdata/", "test-results/"}

var testSuffixes = []string{"_test.go", ".test.ts", ".test.tsx", ".test.js", ".test.jsx", ".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx"}

type EnvDelta struct {
	Added   []string
	Removed []string
}

type Trigger struct {
	Path    string
	Context string
	Reason  string
}

type Report struct {
	Exempt   bool
	Triggers []Trigger
	Required []string
	Missing  []string
}

func ContextFile(feature string) string {
	return ContextDirectory + feature + ".md"
}

func (r Report) OK() bool {
	return r.Exempt || len(r.Missing) == 0
}

func Analyze(pathMap *PathMap, changed []string, delta EnvDelta, exempt bool) Report {
	report := Report{Exempt: exempt}
	if pathMap == nil {
		return report
	}

	touched := make(map[string]struct{})
	for _, file := range normalizePaths(changed) {
		if IsContextPath(file) {
			if strings.HasPrefix(file, ContextDirectory) {
				touched[file] = struct{}{}
			}
			continue
		}
		if IsTestPath(file) {
			continue
		}
		if owner, ok := ContextOwner(pathMap, file); ok {
			report.Triggers = append(report.Triggers, Trigger{Path: file, Context: ContextFile(owner), Reason: "mapped path"})
			continue
		}
		if folder, ok := newFolder(pathMap, file); ok {
			report.Triggers = append(report.Triggers, Trigger{Path: folder, Context: ContextFile(ArchitectureContext), Reason: "unmapped folder, add it to " + MapPath})
		}
	}

	report.Triggers = append(report.Triggers, envTriggers(pathMap, delta)...)
	sortTriggers(report.Triggers)

	required := make(map[string]struct{}, len(report.Triggers))
	for _, trigger := range report.Triggers {
		required[trigger.Context] = struct{}{}
	}
	report.Required = sortedUnique(mapKeys(required))
	if exempt {
		return report
	}
	for _, feature := range report.Required {
		if _, ok := touched[feature]; !ok {
			report.Missing = append(report.Missing, feature)
		}
	}
	return report
}

func ContextOwner(pathMap *PathMap, file string) (string, bool) {
	owner := ""
	ownerLength := -1
	for _, entry := range pathMap.Paths {
		if entry.Kind == KindFile {
			if entry.Target == file && len(entry.Target) > ownerLength {
				owner, ownerLength = entry.Context, len(entry.Target)
			}
			continue
		}
		if strings.HasPrefix(file, entry.Target+"/") && len(entry.Target) > ownerLength {
			owner, ownerLength = entry.Context, len(entry.Target)
		}
	}
	return owner, ownerLength >= 0
}

func EnvOwner(pathMap *PathMap, key string) string {
	for _, entry := range pathMap.Env {
		if entry.Key == key {
			return entry.Context
		}
	}
	return PlatformContext
}

func IsContextPath(file string) bool {
	return file == IndexPath || strings.HasPrefix(file, ContextDirectory)
}

func IsTestPath(file string) bool {
	for _, prefix := range testPrefixes {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	for _, suffix := range testSuffixes {
		if strings.HasSuffix(file, suffix) {
			return true
		}
	}
	return false
}

func ParseEnvKeyList(makefile string) []string {
	lines := strings.Split(makefile, "\n")
	keys := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "APP_ENV_KEYS") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "APP_ENV_KEYS"))
		if !strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, ":=") {
			continue
		}
		rest = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
		for strings.HasSuffix(rest, "\\") {
			rest = strings.TrimSuffix(rest, "\\")
			i++
			if i >= len(lines) {
				break
			}
			rest += " " + strings.TrimSpace(lines[i])
		}
		if comment := strings.Index(rest, "#"); comment >= 0 {
			rest = rest[:comment]
		}
		keys = append(keys, strings.Fields(rest)...)
	}
	return sortedUnique(keys)
}

func DiffEnvKeys(base, head []string) EnvDelta {
	baseSet := toSet(base)
	headSet := toSet(head)
	delta := EnvDelta{}
	for key := range headSet {
		if _, ok := baseSet[key]; !ok {
			delta.Added = append(delta.Added, key)
		}
	}
	for key := range baseSet {
		if _, ok := headSet[key]; !ok {
			delta.Removed = append(delta.Removed, key)
		}
	}
	sort.Strings(delta.Added)
	sort.Strings(delta.Removed)
	return delta
}

func (r Report) Summary() string {
	var builder strings.Builder
	builder.WriteString("### Context docs check\n\n")
	if r.Exempt {
		builder.WriteString("Skipped: the `" + LabelNoContextChange + "` label is set.\n")
		return builder.String()
	}
	if len(r.Triggers) == 0 {
		builder.WriteString("No mapped area changed; no context update is required.\n")
		return builder.String()
	}

	builder.WriteString("| Changed | Owning context | Why |\n| --- | --- | --- |\n")
	for _, trigger := range r.Triggers {
		_, _ = fmt.Fprintf(&builder, "| `%s` | `%s` | %s |\n", trigger.Path, trigger.Context, trigger.Reason)
	}
	if len(r.Missing) == 0 {
		builder.WriteString("\nEvery mapped change has a context update.\n")
		return builder.String()
	}

	builder.WriteString("\nMissing context updates:\n")
	for _, missing := range r.Missing {
		_, _ = fmt.Fprintf(&builder, "- `%s` — required by %s\n", missing, quotedPaths(r.Triggers, missing))
	}
	builder.WriteString("\nUpdate the listed file(s), or add the `" + LabelNoContextChange + "` label when the change genuinely has no context impact.\n")
	return builder.String()
}

func envTriggers(pathMap *PathMap, delta EnvDelta) []Trigger {
	triggers := make([]Trigger, 0, len(delta.Added)+len(delta.Removed))
	for _, key := range sortedUnique(delta.Added) {
		triggers = append(triggers, Trigger{Path: "Makefile", Context: ContextFile(EnvOwner(pathMap, key)), Reason: "env key added: " + key})
	}
	for _, key := range sortedUnique(delta.Removed) {
		triggers = append(triggers, Trigger{Path: "Makefile", Context: ContextFile(EnvOwner(pathMap, key)), Reason: "env key removed: " + key})
	}
	return triggers
}

func newFolder(pathMap *PathMap, file string) (string, bool) {
	for _, root := range triggerRoots {
		if !strings.HasPrefix(file, root) {
			continue
		}
		name, _, ok := strings.Cut(strings.TrimPrefix(file, root), "/")
		if !ok || name == "" {
			continue
		}
		folder := root + name
		if isMappedFolder(pathMap, folder) {
			continue
		}
		return folder, true
	}
	return "", false
}

func isMappedFolder(pathMap *PathMap, folder string) bool {
	for _, entry := range pathMap.Paths {
		if entry.Kind == KindDirectory && entry.Target == folder {
			return true
		}
	}
	return false
}

func normalizePaths(files []string) []string {
	seen := make(map[string]struct{}, len(files))
	normalized := make([]string, 0, len(files))
	for _, file := range files {
		file = path.Clean(strings.TrimSpace(file))
		if file == "" || file == "." || file == ".." || strings.HasPrefix(file, "../") {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		normalized = append(normalized, file)
	}
	sort.Strings(normalized)
	return normalized
}

func sortTriggers(triggers []Trigger) {
	sort.Slice(triggers, func(i, j int) bool {
		if triggers[i].Path != triggers[j].Path {
			return triggers[i].Path < triggers[j].Path
		}
		if triggers[i].Context != triggers[j].Context {
			return triggers[i].Context < triggers[j].Context
		}
		return triggers[i].Reason < triggers[j].Reason
	})
}

func quotedPaths(triggers []Trigger, context string) string {
	var paths []string
	for _, trigger := range triggers {
		if trigger.Context == context {
			paths = append(paths, "`"+trigger.Path+"`")
		}
	}
	return strings.Join(paths, ", ")
}

func sortedUnique(values []string) []string {
	seen := toSet(values)
	unique := mapKeys(seen)
	sort.Strings(unique)
	return unique
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
