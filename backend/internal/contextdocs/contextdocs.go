package contextdocs

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	IndexPath     = "context.md"
	MapPath       = "context/paths.yaml"
	KindDirectory = "directory"
	KindFile      = "file"
)

var Sections = []string{
	"Purpose",
	"Why",
	"Flows",
	"Contracts",
	"Code map",
	"Tools & dependencies",
	"Known limits",
}

type PathEntry struct {
	Target  string `yaml:"target"`
	Kind    string `yaml:"kind"`
	Context string `yaml:"context"`
}

type EnvEntry struct {
	Key     string `yaml:"key"`
	Context string `yaml:"context"`
}

type PathMap struct {
	Version      int         `yaml:"version"`
	ContextFiles []string    `yaml:"context_files"`
	Paths        []PathEntry `yaml:"paths"`
	Env          []EnvEntry  `yaml:"env"`
}

type Document struct {
	Title    string
	Sections []string
}

func ParsePathMap(data []byte) (*PathMap, error) {
	var pathMap PathMap
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&pathMap); err != nil {
		return nil, fmt.Errorf("parse path map: %w", err)
	}
	if err := pathMap.Validate(); err != nil {
		return nil, err
	}
	return &pathMap, nil
}

func (m *PathMap) Validate() error {
	if m.Version < 1 {
		return errors.New("path map version must be at least 1")
	}
	if len(m.ContextFiles) == 0 {
		return errors.New("path map has no context files")
	}
	if err := validateUnique("context file", m.ContextFiles); err != nil {
		return err
	}
	if len(m.Paths) == 0 {
		return errors.New("path map has no paths")
	}
	targets := make([]string, 0, len(m.Paths))
	for _, entry := range m.Paths {
		if entry.Target == "" {
			return errors.New("path entry has an empty target")
		}
		if entry.Context == "" {
			return fmt.Errorf("path entry %q has an empty context", entry.Target)
		}
		if entry.Kind != KindDirectory && entry.Kind != KindFile {
			return fmt.Errorf("path entry %q has unknown kind %q", entry.Target, entry.Kind)
		}
		targets = append(targets, entry.Target)
	}
	if err := validateUnique("path target", targets); err != nil {
		return err
	}
	keys := make([]string, 0, len(m.Env))
	for _, entry := range m.Env {
		if entry.Key == "" {
			return errors.New("env entry has an empty key")
		}
		if entry.Context == "" {
			return fmt.Errorf("env entry %q has an empty context", entry.Key)
		}
		keys = append(keys, entry.Key)
	}
	return validateUnique("env key", keys)
}

func validateUnique(label string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return fmt.Errorf("duplicate %s %q", label, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func ParseDocument(markdown string) (Document, error) {
	var document Document
	inFence := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			document.Sections = append(document.Sections, strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") && document.Title == "" {
			document.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	if document.Title == "" {
		return Document{}, errors.New("document has no level-one heading")
	}
	return document, nil
}

func (d Document) Validate() error {
	if d.Title == "" {
		return errors.New("document has no title")
	}
	if len(d.Sections) != len(Sections) {
		return fmt.Errorf("document has %d sections, want %d", len(d.Sections), len(Sections))
	}
	for i, want := range Sections {
		if d.Sections[i] != want {
			return fmt.Errorf("section %d is %q, want %q", i+1, d.Sections[i], want)
		}
	}
	return nil
}

func Subdirectories(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func CountTargets(entries []PathEntry) map[string]int {
	counts := make(map[string]int, len(entries))
	for _, entry := range entries {
		counts[entry.Target]++
	}
	return counts
}

func NotMappedOnce(targets []string, counts map[string]int) []string {
	var problems []string
	for _, target := range targets {
		if counts[target] != 1 {
			problems = append(problems, target)
		}
	}
	return problems
}
