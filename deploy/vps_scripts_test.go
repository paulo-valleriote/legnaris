package deploy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	entryScript     = "vps/ssh-entry.sh"
	bootstrapScript = "vps/bootstrap.sh"
	deployScript    = "vps/deploy.sh"
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

func TestSSHEntryRejectsMalformedSHA(t *testing.T) {
	for _, tc := range malformedSHACases() {
		t.Run(tc.name, func(t *testing.T) {
			entry, _, stubOut := stageEntryTree(t)
			res := runScript(t, entry, entryEnv("deploy "+tc.sha, stubOut), "")

			assert.Equal(t, usageExit, res.code, res.output)
			assert.NoFileExists(t, stubOut, "deploy.sh must not run for a malformed SHA")
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
		{
			name:        "Edge: a lone newline on stdin is refused and keeps the existing file",
			stdin:       "\n",
			preExisting: "OLD=1\n",
			wantCode:    65,
			wantEnv:     "OLD=1\n",
			wantDeploy:  false,
		},
		{
			name:       "Edge: whitespace-only stdin writes nothing",
			stdin:      " \t\n\n  \n",
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
	state    string
	logPath  string
	env      []string
	appDir   string
	authKeys string
	dropin   string
	autoUp   string
	bin      string
}

// bootstrapHardenedSSHD is the drop-in content the bootstrap must produce.
const bootstrapHardenedSSHD = "PasswordAuthentication no\nPermitRootLogin no"

const testPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKeyOnly ci@legnaris"

func newBootstrapFixture(t *testing.T) *bootstrapFixture {
	t.Helper()
	root := t.TempDir()
	f := &bootstrapFixture{
		state:    filepath.Join(root, "state"),
		logPath:  filepath.Join(root, "calls.log"),
		appDir:   filepath.Join(root, "opt", "legnaris"),
		authKeys: filepath.Join(root, "home", "deploy", ".ssh", "authorized_keys"),
		dropin:   filepath.Join(root, "sshd", "00-legnaris.conf"),
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
		"sshd": `
case "$1" in
  -t) [[ -z "${FAKE_SSHD_FAILS:-}" ]] ;;
  -T) printf '%b\n' "${FAKE_SSHD_EFFECTIVE-passwordauthentication no\npermitrootlogin no}" ;;
esac`,
		"chown":     `exit 0`,
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
	"useradd", "usermod", "ufw allow", "ufw --force enable", "apt-get", "git clone", "chown", "systemctl",
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

func TestBootstrapFreshCloneIsOwnedByDeployUser(t *testing.T) {
	f := newBootstrapFixture(t)

	res := runScript(t, bootstrapScript, f.env, "", "--key-login-verified")

	require.Equal(t, 0, res.code, res.output)
	log := readLog(t, f.logPath)
	assert.Contains(t, log, "chown -R deploy:deploy "+f.appDir)
	assert.Greater(t, strings.Index(log, "chown -R deploy:deploy "+f.appDir), strings.Index(log, "git clone"),
		"ownership must be handed over after the clone exists")
}

func TestBootstrapFailsWithoutUfw(t *testing.T) {
	if _, err := exec.LookPath("ufw"); err == nil {
		t.Skip("host has ufw on PATH; the missing-binary case cannot be simulated")
	}
	f := newBootstrapFixture(t)
	require.NoError(t, os.Remove(filepath.Join(f.bin, "ufw")))

	res := runScript(t, bootstrapScript, f.env, "", "--key-login-verified")

	assert.Equal(t, 1, res.code, res.output)
	assert.Contains(t, res.output, "ufw is not installed")
	assertNoMutations(t, readLog(t, f.logPath))
	assert.NoFileExists(t, f.dropin)
}

func TestBootstrapVerifiesEffectiveSSHDSettings(t *testing.T) {
	tests := []struct {
		name           string
		effective      string
		preExisting    bool
		wantCode       int
		wantReload     bool
		wantDropinGone bool
	}{
		{
			name:       "Happy path: effective settings disable password and root login",
			effective:  "passwordauthentication no\npermitrootlogin no",
			wantCode:   0,
			wantReload: true,
		},
		{
			name:           "Error: an earlier file re-enables password login, drop-in is reverted",
			effective:      "passwordauthentication yes\npermitrootlogin no",
			wantCode:       1,
			wantDropinGone: true,
		},
		{
			name:           "Error: an earlier file re-enables root login, drop-in is reverted",
			effective:      "passwordauthentication no\npermitrootlogin yes",
			wantCode:       1,
			wantDropinGone: true,
		},
		{
			name:           "Edge: effective output that omits the settings fails closed",
			effective:      "",
			wantCode:       1,
			wantDropinGone: true,
		},
		{
			name:        "Error: drop-in already present but shadowed fails loudly on rerun",
			effective:   "passwordauthentication yes\npermitrootlogin no",
			preExisting: true,
			wantCode:    1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newBootstrapFixture(t)
			f.env = append(f.env, "FAKE_SSHD_EFFECTIVE="+tc.effective)
			if tc.preExisting {
				require.NoError(t, os.MkdirAll(filepath.Dir(f.dropin), 0o755))
				require.NoError(t, os.WriteFile(f.dropin, []byte(bootstrapHardenedSSHD+"\n"), 0o644))
			}

			res := runScript(t, bootstrapScript, f.env, "", "--key-login-verified")

			assert.Equal(t, tc.wantCode, res.code, res.output)
			log := readLog(t, f.logPath)
			assert.Equal(t, tc.wantReload, strings.Contains(log, "systemctl reload ssh"))
			if tc.wantCode != 0 {
				assert.Contains(t, res.output, "effective sshd settings")
			}
			if tc.wantDropinGone {
				assert.NoFileExists(t, f.dropin)
			}
		})
	}
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

const (
	newSHA = "1111111111111111111111111111111111111111"
	oldSHA = "2222222222222222222222222222222222222222"
)

// deployFakes records git and docker calls. Each FAKE_FAIL_* variable selects
// one failure: a git command containing FAKE_FAIL_GIT, or a compose subcommand
// FAKE_FAIL_SUB run for the image tag FAKE_FAIL_TAG. FAKE_ANCESTOR is the exit
// status of merge-base, and FAKE_FETCH_MAIN_FAIL fails fetching origin main.
var deployFakes = map[string]string{
	"git": `
if [[ -n "${FAKE_FAIL_GIT:-}" && "$*" == *"$FAKE_FAIL_GIT"* ]]; then exit 1; fi
if [[ "$1" == fetch && " $* " == *" origin main "* && -n "${FAKE_FETCH_MAIN_FAIL:-}" ]]; then exit 1; fi
if [[ "$1" == merge-base ]]; then exit "${FAKE_ANCESTOR:-0}"; fi`,
	"docker": `
for sub in pull up; do
  [[ " $* " == *" $sub "* ]] || continue
  printf '%s %s\n' "${IMAGE_TAG:-none}" "$sub" >>"$FAKE_LOG"
  if [[ -n "${FAKE_FAIL_TAG:-}" && "${IMAGE_TAG:-}" == "$FAKE_FAIL_TAG" && "$sub" == "${FAKE_FAIL_SUB:-}" ]]; then exit 1; fi
done
exit 0`,
}

// stageDeployTree copies deploy.sh into a throwaway checkout layout. withGit
// creates an empty .git directory so the ancestry check on main is active.
func stageDeployTree(t *testing.T, withGit bool) (script, root string) {
	t.Helper()
	root = t.TempDir()
	vps := filepath.Join(root, "deploy", "vps")
	require.NoError(t, os.MkdirAll(vps, 0o755))
	src, err := os.ReadFile(deployScript)
	require.NoError(t, err)
	script = filepath.Join(vps, "deploy.sh")
	require.NoError(t, os.WriteFile(script, src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "deploy", "docker-compose.yml"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "deploy", "docker-compose.vps.yml"), nil, 0o644))
	if withGit {
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	}
	return script, root
}

// composeActions returns the logged compose calls as "<tag> <subcommand>".
func composeActions(log string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
		if strings.HasPrefix(line, "sha-") || strings.HasPrefix(line, "none ") {
			out = append(out, strings.TrimPrefix(line, "none "))
		}
	}
	return out
}

func readFileOrEmpty(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return strings.TrimSpace(string(raw))
}

func TestDeployReleaseFailuresStopBeforeUp(t *testing.T) {
	tests := []struct {
		name       string
		prev       string
		env        []string
		wantCode   int
		wantCalls  []string
		wantState  string
		wantOutput string
	}{
		{
			name:      "Happy path: new commit is pulled, started and recorded",
			prev:      oldSHA,
			wantCode:  0,
			wantCalls: []string{"sha-" + newSHA + " pull", "sha-" + newSHA + " up"},
			wantState: newSHA,
		},
		{
			name:       "Error: fetch fails, up is not run and the previous commit is restored",
			prev:       oldSHA,
			env:        []string{"FAKE_FAIL_GIT=fetch --depth 1 origin " + newSHA},
			wantCode:   1,
			wantCalls:  []string{"sha-" + oldSHA + " pull", "sha-" + oldSHA + " up"},
			wantState:  oldSHA,
			wantOutput: "rolling back",
		},
		{
			name:       "Error: checkout fails, up is not run and the previous commit is restored",
			prev:       oldSHA,
			env:        []string{"FAKE_FAIL_GIT=checkout --detach " + newSHA},
			wantCode:   1,
			wantCalls:  []string{"sha-" + oldSHA + " pull", "sha-" + oldSHA + " up"},
			wantState:  oldSHA,
			wantOutput: "rolling back",
		},
		{
			name:      "Error: image pull fails, up is not run for the new commit",
			prev:      oldSHA,
			env:       []string{"FAKE_FAIL_TAG=sha-" + newSHA, "FAKE_FAIL_SUB=pull"},
			wantCode:  1,
			wantCalls: []string{"sha-" + newSHA + " pull", "sha-" + oldSHA + " pull", "sha-" + oldSHA + " up"},
			wantState: oldSHA,
		},
		{
			name:      "Error: new stack never becomes healthy, previous commit is restored",
			prev:      oldSHA,
			env:       []string{"FAKE_FAIL_TAG=sha-" + newSHA, "FAKE_FAIL_SUB=up"},
			wantCode:  1,
			wantCalls: []string{"sha-" + newSHA + " pull", "sha-" + newSHA + " up", "sha-" + oldSHA + " pull", "sha-" + oldSHA + " up"},
			wantState: oldSHA,
		},
		{
			name:      "Edge: failure with no previous deploy recorded has nothing to roll back to",
			prev:      "",
			env:       []string{"FAKE_FAIL_GIT=fetch --depth 1 origin " + newSHA},
			wantCode:  1,
			wantCalls: nil,
			wantState: "",
		},
		{
			name:       "Edge: rollback target fails too and the run reports it",
			prev:       oldSHA,
			env:        []string{"FAKE_FAIL_GIT=fetch --depth 1 origin " + newSHA, "FAKE_FAIL_TAG=sha-" + oldSHA, "FAKE_FAIL_SUB=pull"},
			wantCode:   1,
			wantCalls:  []string{"sha-" + oldSHA + " pull"},
			wantState:  oldSHA,
			wantOutput: "rollback also failed",
		},
		{
			name:      "Edge: redeploying the recorded commit does not roll back to itself",
			prev:      newSHA,
			env:       []string{"FAKE_FAIL_GIT=fetch --depth 1 origin " + newSHA},
			wantCode:  1,
			wantCalls: nil,
			wantState: newSHA,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			script, root := stageDeployTree(t, false)
			stateFile := filepath.Join(root, "deploy", ".deployed_sha")
			if tc.prev != "" {
				require.NoError(t, os.WriteFile(stateFile, []byte(tc.prev+"\n"), 0o644))
			}
			env, logPath := scriptEnv(t, deployFakes)
			env = append(env, tc.env...)

			res := runScript(t, script, env, "", newSHA)

			assert.Equal(t, tc.wantCode, res.code, res.output)
			assert.Equal(t, tc.wantCalls, composeActions(readLog(t, logPath)))
			if tc.wantOutput != "" {
				assert.Contains(t, res.output, tc.wantOutput)
			}
			assert.Equal(t, tc.wantState, readFileOrEmpty(t, stateFile))
		})
	}
}

func TestDeployRefusesCommitOutsideMain(t *testing.T) {
	tests := []struct {
		name       string
		env        []string
		wantCode   int
		wantOutput string
	}{
		{
			name:       "Error: commit is not an ancestor of origin/main",
			env:        []string{"FAKE_ANCESTOR=1"},
			wantCode:   66,
			wantOutput: "is not an ancestor of origin/main",
		},
		{
			name:       "Error: origin/main cannot be fetched, so the check fails closed",
			env:        []string{"FAKE_FETCH_MAIN_FAIL=1"},
			wantCode:   1,
			wantOutput: "could not fetch origin/main",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			script, root := stageDeployTree(t, true)
			env, logPath := scriptEnv(t, deployFakes)
			env = append(env, tc.env...)

			res := runScript(t, script, env, "", newSHA)

			assert.Equal(t, tc.wantCode, res.code, res.output)
			assert.Contains(t, res.output, tc.wantOutput)
			assert.Empty(t, composeActions(readLog(t, logPath)), "nothing may be pulled or started")
			assert.NoFileExists(t, filepath.Join(root, "deploy", ".deployed_sha"))
		})
	}
}

func TestDeployChecksAncestryBeforeRelease(t *testing.T) {
	script, _ := stageDeployTree(t, true)
	env, logPath := scriptEnv(t, deployFakes)

	res := runScript(t, script, env, "", newSHA)

	require.Equal(t, 0, res.code, res.output)
	log := readLog(t, logPath)
	assert.Contains(t, log, "git fetch --quiet origin main")
	assert.Contains(t, log, "git merge-base --is-ancestor "+newSHA+" origin/main")
	assert.Equal(t, []string{"sha-" + newSHA + " pull", "sha-" + newSHA + " up"}, composeActions(log))
}
