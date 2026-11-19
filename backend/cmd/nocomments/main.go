package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	files, err := resolveFiles(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	violations, err := scanPaths(files)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	for _, violation := range violations {
		_, _ = fmt.Fprintln(stdout, violation.String())
	}
	if len(violations) > 0 {
		return 1
	}
	return 0
}

func resolveFiles(args []string) ([]string, error) {
	var explicit []string
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		explicit = append(explicit, arg)
	}
	if len(explicit) > 0 {
		return explicit, nil
	}
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	tracked, err := gitTrackedFiles(root)
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(root); err != nil {
		return nil, fmt.Errorf("enter repository root: %w", err)
	}
	return filterScanTargets(tracked), nil
}

func repoRoot() (string, error) {
	out, err := exec.CommandContext(context.Background(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitTrackedFiles(root string) ([]string, error) {
	out, err := exec.CommandContext(context.Background(), "git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	trimmed := strings.TrimSuffix(string(out), "\x00")
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\x00"), nil
}

var excludedDirNames = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	"dist":         true,
}

func filterScanTargets(tracked []string) []string {
	var files []string
	for _, rel := range tracked {
		if !isScanTarget(rel) || hasExcludedDir(rel) {
			continue
		}
		files = append(files, filepath.FromSlash(rel))
	}
	return files
}

func isScanTarget(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

func hasExcludedDir(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if excludedDirNames[part] {
			return true
		}
	}
	return false
}

func scanPaths(files []string) ([]Violation, error) {
	violations := make([]Violation, 0, len(files))
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".go":
			violations = append(violations, scanGo(path, src)...)
		case ".ts", ".tsx":
			violations = append(violations, scanTS(path, src)...)
		}
	}
	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].Path != violations[j].Path {
			return violations[i].Path < violations[j].Path
		}
		if violations[i].Line != violations[j].Line {
			return violations[i].Line < violations[j].Line
		}
		return violations[i].Column < violations[j].Column
	})
	return violations, nil
}
