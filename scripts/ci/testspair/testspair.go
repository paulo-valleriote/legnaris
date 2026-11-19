package testspair

import (
	sharedjson "legnaris/backend/shared/json"
	"path"
	"sort"
	"strings"
)

const ExceptionLabel = "no-tests:refactor"

const (
	kindGo  = "go"
	kindWeb = "web"
)

const (
	webSourcePrefix = "web/src/"
	webTestPrefix   = "web/src/test/"
)

var webTestSuffixes = []string{
	".test.ts",
	".test.tsx",
	".test.js",
	".test.jsx",
	".spec.ts",
	".spec.tsx",
	".spec.js",
	".spec.jsx",
	".d.ts",
}

var generatedSuffixes = []string{
	"_gen.go",
	".gen.go",
	".gen.ts",
	".gen.tsx",
	".pb.go",
}

type Violation struct {
	Kind string
	Name string
}

func (v Violation) String() string {
	if v.Kind == kindWeb {
		return v.Name + " changed without a change under " + webTestPrefix
	}
	return v.Kind + " " + v.Name + " changed without a *_test.go change"
}

func Violations(changed []string, generated func(string) bool) []Violation {
	var out []Violation
	sources := map[string]bool{}
	tested := map[string]bool{}
	webSource := false
	webTested := false

	for _, raw := range changed {
		p := normalize(raw)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, webSourcePrefix) {
			switch {
			case strings.HasPrefix(p, webTestPrefix):
				webTested = true
			case isIgnored(p, generated), isWebTestFile(p):
			default:
				webSource = true
			}
			continue
		}
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		if isIgnored(p, generated) {
			continue
		}
		dir := path.Dir(p)
		if strings.HasSuffix(p, "_test.go") {
			tested[dir] = true
			continue
		}
		sources[dir] = true
	}

	for dir := range sources {
		if tested[dir] {
			continue
		}
		out = append(out, Violation{Kind: kindGo, Name: dir})
	}
	if webSource && !webTested {
		out = append(out, Violation{Kind: kindWeb, Name: webSourcePrefix})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func HasLabel(labelsJSON, label string) bool {
	if strings.TrimSpace(labelsJSON) == "" || label == "" {
		return false
	}
	var labels []string
	if err := sharedjson.DecodeBytes([]byte(labelsJSON), &labels); err != nil {
		return false
	}
	for _, candidate := range labels {
		if candidate == label {
			return true
		}
	}
	return false
}

func normalize(raw string) string {
	p := path.Clean(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

func isIgnored(p string, generated func(string) bool) bool {
	if generated != nil && generated(p) {
		return true
	}
	if strings.HasSuffix(p, ".md") {
		return true
	}
	if strings.HasPrefix(p, ".github/") {
		return true
	}
	base := path.Base(p)
	segments := strings.Split(p, "/")
	for _, segment := range segments[:len(segments)-1] {
		if segment == "mocks" || segment == "testdata" {
			return true
		}
	}
	if strings.HasPrefix(base, "zz_generated") {
		return true
	}
	for _, suffix := range generatedSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func isWebTestFile(p string) bool {
	base := path.Base(p)
	for _, suffix := range webTestSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return strings.Contains(p, "/__tests__/") ||
		strings.Contains(p, "/__mocks__/") ||
		strings.Contains(p, "/__snapshots__/")
}
