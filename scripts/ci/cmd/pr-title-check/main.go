package main

import (
	"fmt"
	"io"
	"os"

	"legnaris/scripts/ci/prtitle"
)

func main() {
	os.Exit(run(os.Getenv("PR_TITLE"), os.Stdout, os.Stderr))
}

func run(title string, stdout, stderr io.Writer) int {
	if err := prtitle.Validate(title); err != nil {
		_, _ = fmt.Fprintln(stderr, "pr-title-check:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "pr-title-check: %s\n", title)
	return 0
}
