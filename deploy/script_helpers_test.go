package deploy_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// validSHA is a well-formed 40-character lowercase commit SHA used as the happy
// path input by the deploy scripts tests.
const validSHA = "3f9a1c2b4d5e6f708192a3b4c5d6e7f809a0b1c2"

// usageExit is the exit status the deploy scripts use for rejected input.
const usageExit = 64

// shaCase is one malformed commit SHA input with a readable name.
type shaCase struct {
	name string
	sha  string
}

// malformedSHACases lists inputs that every script taking a commit SHA must
// reject before running any external tool.
func malformedSHACases() []shaCase {
	return []shaCase{
		{name: "Edge: empty SHA", sha: ""},
		{name: "Edge: 39-character SHA", sha: validSHA[:39]},
		{name: "Edge: 41-character SHA", sha: validSHA + "0"},
		{name: "Edge: uppercase hex", sha: strings.ToUpper(validSHA)},
		{name: "Edge: non-hex character", sha: "g" + validSHA[1:]},
		{name: "Edge: shell metacharacters", sha: validSHA + ";id"},
		{name: "Edge: command substitution", sha: "$(id)"},
		{name: "Edge: injected git option", sha: validSHA + " --all"},
		{name: "Edge: a branch name is not a SHA", sha: "main"},
	}
}

// fakeLogPrelude records every fake command invocation as one line in $FAKE_LOG,
// so tests can assert which system tools a script called and in what order.
const fakeLogPrelude = `log() { printf '%s' "$(basename "$0")" >>"$FAKE_LOG"; printf ' %s' "$@" >>"$FAKE_LOG"; printf '\n' >>"$FAKE_LOG"; }
`

type scriptResult struct {
	output string
	code   int
}

// fakeBin writes one executable per entry of fakes into a fresh directory and
// returns it. The directory goes first on PATH, so the scripts pick up the fakes
// instead of the host's system tools.
func fakeBin(t *testing.T, fakes map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range fakes {
		src := "#!/usr/bin/env bash\nset -eu\n" + fakeLogPrelude + "log \"$@\"\n" + body + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o755))
	}
	return dir
}

// scriptEnv returns an environment that runs the fakes first on PATH and logs
// their invocations to a fresh file, which is returned alongside the env.
func scriptEnv(t *testing.T, fakes map[string]string) (env []string, logPath string) {
	t.Helper()
	logPath = filepath.Join(t.TempDir(), "calls.log")
	return []string{fakePATH(fakeBin(t, fakes)), "FAKE_LOG=" + logPath}, logPath
}

// fakePATH returns the PATH value that resolves fakes first and then the host's
// PATH for the tools the scripts use that are not faked (mktemp, grep, cat, ...).
func fakePATH(bin string) string {
	return "PATH=" + bin + ":" + os.Getenv("PATH")
}

// runScript executes a script with bash as a child process. The environment is
// exactly env, so nothing from the test runner leaks into the script.
func runScript(t *testing.T, script string, env []string, stdin string, args ...string) scriptResult {
	t.Helper()
	abs, err := filepath.Abs(script)
	require.NoError(t, err)
	cmd := exec.Command("bash", append([]string{abs}, args...)...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return scriptResult{output: string(out), code: 0}
	case errors.As(err, &exitErr):
		return scriptResult{output: string(out), code: exitErr.ExitCode()}
	default:
		require.NoError(t, err)
		return scriptResult{}
	}
}

// readLog returns the fake command log, or an empty string when nothing ran.
func readLog(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	require.NoError(t, err)
	return string(raw)
}
