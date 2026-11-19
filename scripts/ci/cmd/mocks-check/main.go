package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"legnaris/scripts/ci/mockscheck"
)

const goModPath = "go.mod"

func main() {
	files, err := trackedGoFiles()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "mocks-check:", err)
		os.Exit(2)
	}
	os.Exit(run(goModPath, files, os.Stdout, os.Stderr))
}

func run(gomod string, files []string, stdout, stderr io.Writer) int {
	gomodContent, err := os.ReadFile(gomod)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "mocks-check:", err)
		return 2
	}

	issues := 0
	if err := mockscheck.MockeryPinned(gomodContent); err != nil {
		_, _ = fmt.Fprintln(stderr, "mocks-check:", err)
		issues++
	}
	for _, p := range files {
		content, err := os.ReadFile(p)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "mocks-check:", err)
			issues++
			continue
		}
		for _, issue := range mockscheck.Issues(p, content) {
			_, _ = fmt.Fprintln(stderr, "mocks-check:", issue)
			issues++
		}
	}
	if issues > 0 {
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "mocks-check: %d Go files, every mock carries the header and none lives outside mocks/\n", len(files))
	return 0
}

func trackedGoFiles() ([]string, error) {
	output, err := exec.Command("git", "ls-files", "-z", "--", "*.go").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, name := range strings.Split(string(output), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}
