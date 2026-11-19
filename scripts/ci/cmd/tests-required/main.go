package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"legnaris/scripts/ci/testspair"
)

const generatedHeaderBytes = 4096

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

func run(stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	changed := readPaths(stdin)

	if testspair.HasLabel(getenv("PR_LABELS"), testspair.ExceptionLabel) {
		_, _ = fmt.Fprintf(stdout, "tests-required: skipped by the %q label\n", testspair.ExceptionLabel)
		if err := appendSummary(getenv("GITHUB_STEP_SUMMARY")); err != nil {
			_, _ = fmt.Fprintln(stderr, "tests-required:", err)
		}
		return 0
	}

	violations := testspair.Violations(changed, generatedFile)
	if len(violations) == 0 {
		_, _ = fmt.Fprintf(stdout, "tests-required: %d changed paths, every one that needs a test has one\n", len(changed))
		return 0
	}
	for _, violation := range violations {
		_, _ = fmt.Fprintln(stderr, "tests-required:", violation)
	}
	return 1
}

func readPaths(r io.Reader) []string {
	var paths []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if p := strings.TrimSpace(scanner.Text()); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func generatedFile(p string) bool {
	content, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	if len(content) > generatedHeaderBytes {
		content = content[:generatedHeaderBytes]
	}
	return bytes.Contains(content, []byte("Code generated")) &&
		bytes.Contains(content, []byte("DO NOT EDIT."))
}

func appendSummary(summaryPath string) error {
	if summaryPath == "" {
		return nil
	}
	file, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "The %q label skipped the tests-alongside-behavior-changes check.\n", testspair.ExceptionLabel); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
