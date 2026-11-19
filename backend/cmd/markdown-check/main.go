package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"legnaris/backend/internal/contextdocs"

	sharedjson "legnaris/backend/shared/json"
)

type hookResponse struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("markdown-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	allowlistPath := flags.String("allowlist", contextdocs.MarkdownAllowlistPath, "allow-list path relative to the root")
	hook := flags.Bool("hook", false, "read a Claude Code hook payload from stdin and answer a permission decision")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	allowlist, err := loadAllowlist(*root, *allowlistPath)
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if *hook {
		return runHook(*root, *allowlistPath, allowlist, stdin, stdout, stderr)
	}

	files := flags.Args()
	if len(files) == 0 {
		files, err = trackedMarkdown(*root)
		if err != nil {
			printError(stderr, err)
			return 2
		}
	}

	unlisted := contextdocs.UnlistedMarkdown(allowlist, files)
	if len(unlisted) == 0 {
		_, _ = io.WriteString(stdout, "Every tracked markdown file is on the versioned allow-list.\n")
		return 0
	}
	for _, file := range unlisted {
		_, _ = fmt.Fprintf(stdout, "markdown file %q is not allowed.\n", file)
	}
	_, _ = fmt.Fprintf(stdout, "\nAdd each path to %s in the same pull request: editing the allow-list is the authorization for a new markdown file.\n", *allowlistPath)
	return 1
}

func loadAllowlist(root, allowlistPath string) (*contextdocs.MarkdownAllowlist, error) {
	data, err := os.ReadFile(filepath.Join(root, allowlistPath))
	if err != nil {
		return nil, err
	}
	allowlist, err := contextdocs.ParseMarkdownAllowlist(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", allowlistPath, err)
	}
	return allowlist, nil
}

func trackedMarkdown(root string) ([]string, error) {
	output, err := exec.CommandContext(context.Background(), "git", "-C", root, "ls-files", "--", "*.md").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	lines := strings.Split(string(output), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		if file := strings.TrimSpace(line); file != "" {
			files = append(files, file)
		}
	}
	return files, nil
}

func runHook(root, allowlistPath string, allowlist *contextdocs.MarkdownAllowlist, stdin io.Reader, stdout, stderr io.Writer) int {
	var input contextdocs.HookInput
	payload, err := io.ReadAll(stdin)
	if err == nil {
		err = sharedjson.DecodeBytes(payload, &input)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "markdown-check: read hook payload: %v\n", err)
		return 0
	}

	relative, ok := contextdocs.RepoRelativePath(root, input.CWD, input.ToolInput.FilePath)
	if !ok {
		return 0
	}
	if !contextdocs.IsMarkdownFile(relative) || contextdocs.IsExcludedMarkdownPath(relative) || allowlist.Allows(relative) {
		return 0
	}
	if ignoredByGit(root, relative) {
		return 0
	}

	response := hookResponse{HookSpecificOutput: hookSpecificOutput{
		HookEventName:      "PreToolUse",
		PermissionDecision: "ask",
		PermissionDecisionReason: fmt.Sprintf(
			"%s is not on %s. Add the path to the allow-list in the same change; editing the allow-list is the authorization for a new markdown file.",
			relative, allowlistPath,
		),
	}}
	if err := json.NewEncoder(stdout).Encode(response); err != nil {
		_, _ = fmt.Fprintf(stderr, "markdown-check: write hook response: %v\n", err)
	}
	return 0
}

func ignoredByGit(root, relative string) bool {
	return exec.CommandContext(context.Background(), "git", "-C", root, "check-ignore", "-q", "--", relative).Run() == nil
}

func printError(stderr io.Writer, err error) {
	_, _ = fmt.Fprintf(stderr, "markdown-check: %v\n", err)
}
