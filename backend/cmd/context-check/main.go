package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"legnaris/backend/internal/contextdocs"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("context-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	base := flags.String("base", "", "git ref whose Makefile env key list is compared")
	label := flags.String("label", "", "pull request label that exempts the check")
	summaryPath := flags.String("summary", "", "file that receives the Markdown summary")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	changed, err := changedFiles(flags.Args(), stdin)
	if err != nil {
		printError(stderr, err)
		return 2
	}
	data, err := os.ReadFile(filepath.Join(*root, contextdocs.MapPath))
	if err != nil {
		printError(stderr, err)
		return 2
	}
	pathMap, err := contextdocs.ParsePathMap(data)
	if err != nil {
		printError(stderr, fmt.Errorf("%s: %w", contextdocs.MapPath, err))
		return 2
	}
	delta, err := envDelta(*root, *base)
	if err != nil {
		printError(stderr, err)
		return 2
	}

	report := contextdocs.Analyze(pathMap, changed, delta, *label == contextdocs.LabelNoContextChange)
	summary := report.Summary()
	_, _ = io.WriteString(stdout, summary)
	if *summaryPath != "" {
		if err := appendFile(*summaryPath, summary); err != nil {
			printError(stderr, err)
			return 2
		}
	}
	if !report.OK() {
		return 1
	}
	return 0
}

func changedFiles(args []string, stdin io.Reader) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	files := make([]string, 0)
	for scanner.Scan() {
		file := strings.TrimSpace(scanner.Text())
		if file == "" {
			continue
		}
		files = append(files, file)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read changed files: %w", err)
	}
	return files, nil
}

func envDelta(root, base string) (contextdocs.EnvDelta, error) {
	if base == "" {
		return contextdocs.EnvDelta{}, nil
	}
	head, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return contextdocs.EnvDelta{}, err
	}
	baseMakefile, err := gitShow(root, base, "Makefile")
	if err != nil {
		return contextdocs.EnvDelta{}, err
	}
	return contextdocs.DiffEnvKeys(contextdocs.ParseEnvKeyList(string(baseMakefile)), contextdocs.ParseEnvKeyList(string(head))), nil
}

func gitShow(root, ref, file string) ([]byte, error) {
	output, err := exec.Command("git", "-C", root, "show", ref+":"+file).Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", ref, file, err)
	}
	return output, nil
}

func appendFile(path, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(file, content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func printError(stderr io.Writer, err error) {
	_, _ = fmt.Fprintf(stderr, "context-check: %v\n", err)
}
