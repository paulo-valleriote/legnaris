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

const validSHA = "3f9a1c2b4d5e6f708192a3b4c5d6e7f809a0b1c2"

const usageExit = 64

type shaCase struct {
	name string
	sha  string
}

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

const fakeLogPrelude = `log() { printf '%s' "$(basename "$0")" >>"$FAKE_LOG"; printf ' %s' "$@" >>"$FAKE_LOG"; printf '\n' >>"$FAKE_LOG"; }
`

type scriptResult struct {
	output string
	code   int
}

func fakeBin(t *testing.T, fakes map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range fakes {
		src := "#!/usr/bin/env bash\nset -eu\n" + fakeLogPrelude + "log \"$@\"\n" + body + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o755))
	}
	return dir
}

func scriptEnv(t *testing.T, fakes map[string]string) (env []string, logPath string) {
	t.Helper()
	logPath = filepath.Join(t.TempDir(), "calls.log")
	return []string{fakePATH(fakeBin(t, fakes)), "FAKE_LOG=" + logPath}, logPath
}

func fakePATH(bin string) string {
	return "PATH=" + bin + ":" + os.Getenv("PATH")
}

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

func readLog(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	require.NoError(t, err)
	return string(raw)
}
