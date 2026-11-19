package mongouri

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

var schemes = []string{"mongodb+srv://", "mongodb://"}

var (
	replicaSetParam  = regexp.MustCompile(`[?&;]replicaSet=([^&\s"',;)]+)`)
	retryWritesParam = regexp.MustCompile(`[?&;]retryWrites=([^&\s"',;)]+)`)
	authSourceParam  = regexp.MustCompile(`[?&;]authSource=([^&\s"',;)]+)`)
)

type Finding struct {
	Path    string
	Line    int
	Segment string
	Missing []string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: missing %s", f.Path, f.Line, strings.Join(f.Missing, ", "))
}

func IsConfigPath(p string) bool {
	if strings.HasSuffix(p, ".md") {
		return false
	}
	if strings.HasPrefix(p, "deploy/mongodb/") {
		return false
	}
	base := path.Base(p)
	switch {
	case base == ".env", strings.HasPrefix(base, ".env."), strings.HasSuffix(base, ".env"):
		return true
	case strings.HasSuffix(base, ".yml"), strings.HasSuffix(base, ".yaml"):
		return true
	case strings.HasSuffix(base, ".sh"):
		return true
	case base == "Dockerfile":
		return true
	}
	return false
}

func Extract(line string) []string {
	var starts []int
	for _, scheme := range schemes {
		for from := 0; from < len(line); {
			index := strings.Index(line[from:], scheme)
			if index < 0 {
				break
			}
			starts = append(starts, from+index)
			from += index + len(scheme)
		}
	}
	sort.Ints(starts)
	segments := make([]string, 0, len(starts))
	for i, start := range starts {
		end := len(line)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		segments = append(segments, strings.TrimRight(line[start:end], " \t\"',;"))
	}
	return segments
}

func Missing(segment string) []string {
	var missing []string
	if !nonEmpty(replicaSetParam, segment) {
		missing = append(missing, "replicaSet=")
	}
	if match := retryWritesParam.FindStringSubmatch(segment); match == nil || match[1] != "true" {
		missing = append(missing, "retryWrites=true")
	}
	if !nonEmpty(authSourceParam, segment) {
		missing = append(missing, "authSource=")
	}
	return missing
}

func Findings(p string, content []byte) []Finding {
	var findings []Finding
	for number, line := range strings.Split(string(content), "\n") {
		for _, segment := range Extract(line) {
			if missing := Missing(segment); len(missing) > 0 {
				findings = append(findings, Finding{
					Path:    p,
					Line:    number + 1,
					Segment: segment,
					Missing: missing,
				})
			}
		}
	}
	return findings
}

func nonEmpty(pattern *regexp.Regexp, segment string) bool {
	match := pattern.FindStringSubmatch(segment)
	return match != nil && match[1] != ""
}
