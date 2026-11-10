package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	entryScript     = "vps/ssh-entry.sh"
	bootstrapScript = "vps/bootstrap.sh"
	usageExit       = 64
)

// stageEntryTree copies the forced-command script into a throwaway checkout
// layout (deploy/vps/) next to a stub deploy.sh that records its arguments, so
// the test observes whether the entry script reaches the deploy step.
func stageEntryTree(t *testing.T) (entry, deployDir, stubOut string) {
	t.Helper()
	root := t.TempDir()
	vps := filepath.Join(root, "deploy", "vps")
	require.NoError(t, os.MkdirAll(vps, 0o755))

	src, err := os.ReadFile(entryScript)
	require.NoError(t, err)
	entry = filepath.Join(vps, "ssh-entry.sh")
	require.NoError(t, os.WriteFile(entry, src, 0o755))

	stub := "#!/usr/bin/env bash\nprintf '%s\\n' \"$@\" >\"$STUB_OUT\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(vps, "deploy.sh"), []byte(stub), 0o755))

	return entry, filepath.Join(root, "deploy"), filepath.Join(root, "stub-args")
}

func entryEnv(command, stubOut string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"SSH_ORIGINAL_COMMAND=" + command,
		"STUB_OUT=" + stubOut,
	}
}

func TestSSHEntryRefusesInvalidRequests(t *testing.T) {
	sha := validSHA
	tests := []struct {
		name    string
		command string
	}{
		{name: "Edge: missing request", command: ""},
		{name: "Edge: whitespace only", command: "   "},
		{name: "Error: request type without a SHA", command: "deploy"},
		{name: "Error: unknown request type", command: "shell " + sha},
		{name: "Error: rollback is not a request type", command: "rollback " + sha},
		{name: "Edge: 39-character SHA", command: "deploy " + sha[:39]},
		{name: "Edge: 41-character SHA", command: "deploy " + sha + "0"},
		{name: "Edge: uppercase hex SHA", command: "deploy " + strings.ToUpper(sha)},
		{name: "Edge: non-hex character", command: "deploy g" + sha[1:]},
		{name: "Edge: SHA with a command substitution", command: "deploy $(id)"},
		{name: "Edge: SHA followed by a shell separator", command: "deploy " + sha + ";id"},
		{name: "Edge: embedded newline smuggles a second command", command: "deploy " + sha + "\nid"},
		{name: "Error: extra argument after the SHA", command: "deploy " + sha + " env extra"},
		{name: "Error: env flag in the wrong case", command: "deploy " + sha + " ENV"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry, _, stubOut := stageEntryTree(t)
			res := runScript(t, entry, entryEnv(tc.command, stubOut), "")

			assert.Equal(t, usageExit, res.code)
			assert.Contains(t, res.output, "usage:")
			assert.NoFileExists(t, stubOut, "deploy.sh must not run for a rejected request")
		})
	}
}

func TestSSHEntryAcceptsDeployRequests(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{name: "Happy path: deploy a valid SHA", command: "deploy " + validSHA},
		{name: "Edge: extra spaces between tokens are harmless", command: "deploy   " + validSHA},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry, deployDir, stubOut := stageEntryTree(t)
			res := runScript(t, entry, entryEnv(tc.command, stubOut), "")

			require.Equal(t, 0, res.code, res.output)
			raw, err := os.ReadFile(stubOut)
			require.NoError(t, err)
			assert.Equal(t, validSHA, strings.TrimSpace(string(raw)))
			assert.NoFileExists(t, filepath.Join(deployDir, ".env"),
				"a plain deploy must not touch the production environment file")
		})
	}
}

func TestSSHEntryWritesEnvironmentFromStdin(t *testing.T) {
	tests := []struct {
		name        string
		stdin       string
		preExisting string
		wantCode    int
		wantEnv     string
		wantDeploy  bool
	}{
		{
			name:       "Happy path: env request writes the file and deploys",
			stdin:      "MONGO_PASSWORD=secret\nAPI_TOKEN=abc=def\n",
			wantCode:   0,
			wantEnv:    "MONGO_PASSWORD=secret\nAPI_TOKEN=abc=def\n",
			wantDeploy: true,
		},
		{
			name:        "Happy path: env request replaces an existing file",
			stdin:       "NEW=1\n",
			preExisting: "OLD=1\n",
			wantCode:    0,
			wantEnv:     "NEW=1\n",
			wantDeploy:  true,
		},
		{
			name:        "Edge: empty stdin is refused and keeps the existing file",
			stdin:       "",
			preExisting: "OLD=1\n",
			wantCode:    65,
			wantEnv:     "OLD=1\n",
			wantDeploy:  false,
		},
		{
			name:       "Edge: empty stdin with no existing file writes nothing",
			stdin:      "",
			wantCode:   65,
			wantEnv:    "",
			wantDeploy: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry, deployDir, stubOut := stageEntryTree(t)
			envPath := filepath.Join(deployDir, ".env")
			if tc.preExisting != "" {
				require.NoError(t, os.WriteFile(envPath, []byte(tc.preExisting), 0o600))
			}

			res := runScript(t, entry, entryEnv("deploy "+validSHA+" env", stubOut), tc.stdin)

			assert.Equal(t, tc.wantCode, res.code, res.output)
			if tc.wantEnv == "" {
				assert.NoFileExists(t, envPath)
			} else {
				got, err := os.ReadFile(envPath)
				require.NoError(t, err)
				assert.Equal(t, tc.wantEnv, string(got))
			}
			assert.Equal(t, tc.wantDeploy, fileExists(stubOut))
			assertNoLeftoverTemp(t, deployDir)
		})
	}
}

func TestSSHEntryEnvFileIsOwnerOnly(t *testing.T) {
	entry, deployDir, stubOut := stageEntryTree(t)

	res := runScript(t, entry, entryEnv("deploy "+validSHA+" env", stubOut), "MONGO_PASSWORD=x\n")

	require.Equal(t, 0, res.code, res.output)
	info, err := os.Stat(filepath.Join(deployDir, ".env"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func assertNoLeftoverTemp(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".env.*"))
	require.NoError(t, err)
	assert.Empty(t, matches, "temporary env files must not survive a run")
}

// bootstrapFixture is a fresh host: no deploy user, no firewall rules, no clone,
// with every system path redirected into a temp directory.
type bootstrapFixture struct {
	state     string
	logPath   string
	env       []string
	appDir    string
	authKeys  string
	dropin    string
	autoUp    string
	bin       string
	sshdFails bool
}

const testPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKeyOnly ci@legnaris"

func newBootstrapFixture(t *testing.T) *bootstrapFixture {
	t.Helper()
	root := t.TempDir()
	f := &bootstrapFixture{
		state:    filepath.Join(root, "state"),
		logPath:  filepath.Join(root, "calls.log"),
		appDir:   filepath.Join(root, "opt", "legnaris"),
		authKeys: filepath.Join(root, "home", "deploy", ".ssh", "authorized_keys"),
		dropin:   filepath.Join(root, "sshd", "99-legnaris.conf"),
		autoUp:   filepath.Join(root, "apt", "20auto-upgrades"),
	}
	require.NoError(t, os.MkdirAll(f.state, 0o755))
	f.bin = fakeBin(t, map[string]string{
		"id": `
if [[ "$1" == "-nG" ]]; then
  echo "deploy"
  [[ -f "$FAKE_STATE/docker" ]] && echo "docker"
  exit 0
fi
[[ -f "$FAKE_STATE/user" ]]`,
		"useradd": `touch "$FAKE_STATE/user"`,
		"usermod": `touch "$FAKE_STATE/docker"`,
		"ufw": `
case "$1" in
  status)
    if [[ -f "$FAKE_STATE/ufw_active" ]]; then echo "Status: active"; else echo "Status: inactive"; fi
    [[ -f "$FAKE_STATE/ufw_rules" ]] && while read -r p; do echo "$p/tcp ALLOW Anywhere"; done <"$FAKE_STATE/ufw_rules"
    ;;
  allow)
    port="${2%/tcp}"
    echo "$port" >>"$FAKE_STATE/ufw_rules"
    ;;
  --force) touch "$FAKE_STATE/ufw_active" ;;
esac`,
		"sshd":      `[[ -z "${FAKE_SSHD_FAILS:-}" ]]`,
		"systemctl": `exit 0`,
		"dpkg":      `[[ -f "$FAKE_STATE/unattended" ]]`,
		"apt-get":   `touch "$FAKE_STATE/unattended"`,
		"git": `
if [[ "$1" == "clone" ]]; then mkdir -p "$3/.git"; fi`,
	})
	f.env = []string{
		fakePATH(f.bin),
		"FAKE_LOG=" + f.logPath,
		"FAKE_STATE=" + f.state,
		"LEGNARIS_APP_DIR=" + f.appDir,
		"LEGNARIS_REPO_URL=https://example.invalid/legnaris.git",
		"LEGNARIS_AUTHORIZED_KEYS=" + f.authKeys,
		"LEGNARIS_SSHD_DROPIN=" + f.dropin,
		"LEGNARIS_AUTO_UPGRADES=" + f.autoUp,
		"DEPLOY_PUBLIC_KEY=" + testPublicKey,
	}
	return f
}

func (f *bootstrapFixture) run(t *testing.T, args ...string) scriptResult {
	t.Helper()
	return runScript(t, bootstrapScript, f.env, "", args...)
}

// mutatingCalls lists the logged calls that change host state.
var mutatingCalls = []string{
	"useradd", "usermod", "ufw allow", "ufw --force enable", "apt-get", "git clone", "systemctl",
}

func assertNoMutations(t *testing.T, log string) {
	t.Helper()
	for _, call := range mutatingCalls {
		assert.NotContains(t, log, call, "dry run must not run %q", call)
	}
}

func TestBootstrapDryRunChangesNothing(t *testing.T) {
	f := newBootstrapFixture(t)

	res := f.run(t, "--dry-run")

	require.Equal(t, 0, res.code, res.output)
	assert.Contains(t, res.output, "[dry-run]")
	assertNoMutations(t, readLog(t, f.logPath))
	assert.NoFileExists(t, f.authKeys)
	assert.NoFileExists(t, f.dropin)
	assert.NoFileExists(t, f.autoUp)
	assert.NoDirExists(t, f.appDir)
}

func TestBootstrapFirstRunReachesBaseline(t *testing.T) {
	f := newBootstrapFixture(t)

	res := runScript(t, bootstrapScript, f.env, "", "--key-login-verified")

	require.Equal(t, 0, res.code, res.output)
	log := readLog(t, f.logPath)

	assert.Contains(t, log, "useradd --create-home --shell /bin/bash deploy")
	assert.Contains(t, log, "usermod -aG docker deploy")

	keys, err := os.ReadFile(f.authKeys)
	require.NoError(t, err)
	assert.Equal(t, restrictedKeyLine(f), strings.TrimSpace(string(keys)))

	dropin, err := os.ReadFile(f.dropin)
	require.NoError(t, err)
	assert.Contains(t, string(dropin), "PasswordAuthentication no")
	assert.Contains(t, string(dropin), "PermitRootLogin no")

	assert.Less(t, strings.Index(log, "ufw allow 22/tcp"), strings.Index(log, "ufw --force enable"),
		"SSH must be allowed in the firewall before the firewall is enabled")
	assert.Less(t, strings.Index(log, "sshd -t"), strings.Index(log, "systemctl reload ssh"),
		"the SSH daemon config must validate before it is reloaded")
	assert.Contains(t, log, "git clone https://example.invalid/legnaris.git")
	assert.DirExists(t, filepath.Join(f.appDir, ".git"))
}

func restrictedKeyLine(f *bootstrapFixture) string {
	return `restrict,command="` + f.appDir + `/deploy/vps/ssh-entry.sh" ` + testPublicKey
}

func TestBootstrapSecondRunIsNoOp(t *testing.T) {
	f := newBootstrapFixture(t)
	require.Equal(t, 0, runScript(t, bootstrapScript, f.env, "", "--key-login-verified").code)
	firstKeys := readFileOrFail(t, f.authKeys)
	firstDropin := readFileOrFail(t, f.dropin)

	require.NoError(t, os.Remove(f.logPath))
	res := runScript(t, bootstrapScript, f.env, "", "--key-login-verified")

	require.Equal(t, 0, res.code, res.output)
	assertNoMutations(t, readLog(t, f.logPath))
	assert.Equal(t, firstKeys, readFileOrFail(t, f.authKeys))
	assert.Equal(t, firstDropin, readFileOrFail(t, f.dropin))
}

func readFileOrFail(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

func TestBootstrapPasswordLoginRequiresVerificationFlag(t *testing.T) {
	f := newBootstrapFixture(t)

	res := f.run(t)

	require.Equal(t, 0, res.code, res.output)
	assert.NoFileExists(t, f.dropin, "password login must stay enabled without the verification flag")
	assert.Contains(t, res.output, "password login stays enabled")
	assert.NotContains(t, readLog(t, f.logPath), "systemctl reload ssh")
}

func TestBootstrapRollsBackWhenSSHDConfigIsInvalid(t *testing.T) {
	f := newBootstrapFixture(t)
	f.env = append(f.env, "FAKE_SSHD_FAILS=1")

	res := runScript(t, bootstrapScript, f.env, "", "--key-login-verified")

	assert.Equal(t, 1, res.code, res.output)
	assert.NoFileExists(t, f.dropin, "an invalid drop-in must be removed again")
	assert.NotContains(t, readLog(t, f.logPath), "systemctl reload ssh",
		"the daemon must not be reloaded with a config that failed validation")
}

func TestBootstrapRejectsBadInput(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		mutate func(f *bootstrapFixture)
	}{
		{name: "Error: unknown option", args: []string{"--force"}},
		{name: "Error: missing public key", args: nil, mutate: func(f *bootstrapFixture) {
			f.env = removeEnv(f.env, "DEPLOY_PUBLIC_KEY=")
		}},
		{name: "Edge: empty public key", args: nil, mutate: func(f *bootstrapFixture) {
			f.env = replaceEnv(f.env, "DEPLOY_PUBLIC_KEY=", "DEPLOY_PUBLIC_KEY=")
		}},
		{name: "Edge: key with an injected second line", args: nil, mutate: func(f *bootstrapFixture) {
			f.env = replaceEnv(f.env, "DEPLOY_PUBLIC_KEY=", "DEPLOY_PUBLIC_KEY="+testPublicKey+"\nssh-ed25519 AAAA attacker")
		}},
		{name: "Edge: key that is not an ssh public key", args: nil, mutate: func(f *bootstrapFixture) {
			f.env = replaceEnv(f.env, "DEPLOY_PUBLIC_KEY=", "DEPLOY_PUBLIC_KEY=/etc/shadow")
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newBootstrapFixture(t)
			if tc.mutate != nil {
				tc.mutate(f)
			}

			res := f.run(t, tc.args...)

			assert.Equal(t, usageExit, res.code, res.output)
			assertNoMutations(t, readLog(t, f.logPath))
			assert.NoFileExists(t, f.authKeys)
		})
	}
}

func TestBootstrapReplacesUnrestrictedKeyWithoutDuplicating(t *testing.T) {
	f := newBootstrapFixture(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(f.authKeys), 0o700))
	require.NoError(t, os.WriteFile(f.authKeys, []byte(testPublicKey+"\n"), 0o600))

	res := runScript(t, bootstrapScript, f.env, "", "--key-login-verified")

	require.Equal(t, 0, res.code, res.output)
	assert.Equal(t, restrictedKeyLine(f)+"\n", readFileOrFail(t, f.authKeys))
}

func replaceEnv(env []string, prefix, value string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			out = append(out, value)
			continue
		}
		out = append(out, kv)
	}
	return out
}

func removeEnv(env []string, prefix string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, prefix) {
			out = append(out, kv)
		}
	}
	return out
}
