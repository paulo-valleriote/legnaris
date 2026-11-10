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
	ancestryScript = "cd/check-ancestry.sh"
	imagesScript   = "cd/check-images.sh"
	probeScript    = "cd/probe.sh"
	envKeysScript  = "cd/check-env-keys.sh"
)

func TestCheckAncestryRejectsMalformedSHA(t *testing.T) {
	for _, tc := range malformedSHACases() {
		t.Run(tc.name, func(t *testing.T) {
			env, logPath := scriptEnv(t, map[string]string{"git": `exit 0`})

			res := runScript(t, ancestryScript, env, "", tc.sha)

			assert.Equal(t, usageExit, res.code, res.output)
			assert.Empty(t, readLog(t, logPath), "git must not run for a malformed SHA")
		})
	}
}

func TestCheckAncestryAgainstMain(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantCode int
		wantOut  string
	}{
		{name: "Happy path: commit is reachable from main", mode: "ancestor", wantCode: 0, wantOut: "is on origin/main"},
		{name: "Error: commit is not on main", mode: "diverged", wantCode: 1, wantOut: "is not an ancestor of origin/main"},
		{name: "Error: git fails (missing object) fails closed", mode: "error", wantCode: 1, wantOut: "is not an ancestor"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "calls.log")
			bin := fakeBin(t, map[string]string{
				"git": `
case "$FAKE_MODE" in
  ancestor) exit 0 ;;
  diverged) exit 1 ;;
  *) exit 128 ;;
esac`,
			})
			env := []string{fakePATH(bin), "FAKE_LOG=" + logPath, "FAKE_MODE=" + tc.mode}

			res := runScript(t, ancestryScript, env, "", validSHA)

			assert.Equal(t, tc.wantCode, res.code, res.output)
			assert.Contains(t, res.output, tc.wantOut)
			assert.Contains(t, readLog(t, logPath), "merge-base --is-ancestor "+validSHA+" origin/main")
		})
	}
}

func TestCheckAncestryRejectsBadBaseRef(t *testing.T) {
	tests := []struct {
		name    string
		baseRef string
	}{
		{name: "Edge: empty base ref", baseRef: ""},
		{name: "Edge: base ref that looks like a git option", baseRef: "--output=/tmp/x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, logPath := scriptEnv(t, map[string]string{"git": `exit 0`})
			env = append(env, "BASE_REF="+tc.baseRef)

			res := runScript(t, ancestryScript, env, "", validSHA)

			assert.Equal(t, usageExit, res.code, res.output)
			assert.Empty(t, readLog(t, logPath), "git must not run with a bad base ref")
		})
	}
}

func TestCheckAncestryBaseRefOverride(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "calls.log")
	bin := fakeBin(t, map[string]string{"git": `exit 0`})
	env := []string{fakePATH(bin), "FAKE_LOG=" + logPath, "BASE_REF=refs/remotes/origin/trunk"}

	res := runScript(t, ancestryScript, env, "", validSHA)

	require.Equal(t, 0, res.code, res.output)
	assert.Contains(t, readLog(t, logPath), "--is-ancestor "+validSHA+" refs/remotes/origin/trunk")
}

func TestCheckImagesRejectsMalformedSHA(t *testing.T) {
	for _, tc := range malformedSHACases() {
		t.Run(tc.name, func(t *testing.T) {
			env, logPath := scriptEnv(t, map[string]string{"docker": `exit 0`})

			res := runScript(t, imagesScript, env, "", tc.sha)

			assert.Equal(t, usageExit, res.code, res.output)
			assert.Empty(t, readLog(t, logPath), "docker must not run for a malformed SHA")
		})
	}
}

func TestCheckImagesRejectsMalformedOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner string
	}{
		{name: "Edge: empty owner", owner: ""},
		{name: "Edge: owner with a path separator", owner: "acme/evil"},
		{name: "Edge: owner with a leading hyphen", owner: "-acme"},
		{name: "Edge: owner with whitespace", owner: "ac me"},
		{name: "Edge: owner with a dot", owner: "ac.me"},
		{name: "Edge: owner that climbs the path", owner: "../other"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, logPath := scriptEnv(t, map[string]string{"docker": `exit 0`})
			env = append(env, "IMAGE_OWNER="+tc.owner)

			res := runScript(t, imagesScript, env, "", validSHA)

			assert.Equal(t, usageExit, res.code, res.output)
			assert.Empty(t, readLog(t, logPath), "docker must not run for a malformed owner")
		})
	}
}

func TestCheckImagesManifestsForSHA(t *testing.T) {
	dockerFake := `
for missing in ${FAKE_MISSING:-}; do
  case "$*" in *"legnaris-$missing:"*) exit 1 ;; esac
done
exit 0`

	tests := []struct {
		name         string
		missing      string
		owner        string
		wantCode     int
		wantOutputs  []string
		wantManifest []string
	}{
		{
			name:         "Happy path: all three images exist for the SHA",
			wantCode:     0,
			wantManifest: []string{"ghcr.io/paulo-valleriote/legnaris-app:sha-" + validSHA, "ghcr.io/paulo-valleriote/legnaris-web:sha-" + validSHA, "ghcr.io/paulo-valleriote/legnaris-rabbitmq:sha-" + validSHA},
		},
		{
			name:        "Error: rabbitmq image missing is reported by name",
			missing:     "rabbitmq",
			wantCode:    1,
			wantOutputs: []string{"legnaris-rabbitmq:sha-" + validSHA},
		},
		{
			name:        "Edge: every image missing lists all three",
			missing:     "app web rabbitmq",
			wantCode:    1,
			wantOutputs: []string{"legnaris-app:", "legnaris-web:", "legnaris-rabbitmq:"},
		},
		{
			name:         "Edge: owner is taken from IMAGE_OWNER",
			owner:        "acme",
			wantCode:     0,
			wantManifest: []string{"ghcr.io/acme/legnaris-app:sha-" + validSHA},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "calls.log")
			bin := fakeBin(t, map[string]string{"docker": dockerFake})
			env := []string{fakePATH(bin), "FAKE_LOG=" + logPath, "FAKE_MISSING=" + tc.missing}
			if tc.owner != "" {
				env = append(env, "IMAGE_OWNER="+tc.owner)
			}

			res := runScript(t, imagesScript, env, "", validSHA)

			assert.Equal(t, tc.wantCode, res.code, res.output)
			for _, want := range tc.wantOutputs {
				assert.Contains(t, res.output, want)
			}
			log := readLog(t, logPath)
			for _, want := range tc.wantManifest {
				assert.Contains(t, log, "manifest inspect "+want)
			}
		})
	}
}

func TestProbeRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		url  string
		env  []string
	}{
		{name: "Edge: empty URL", url: ""},
		{name: "Edge: non-HTTP scheme", url: "ftp://example.invalid/"},
		{name: "Edge: URL with whitespace", url: "https://example.invalid/a b"},
		{name: "Error: zero attempts", url: "https://example.invalid/", env: []string{"PROBE_ATTEMPTS=0"}},
		{name: "Error: non-numeric attempts", url: "https://example.invalid/", env: []string{"PROBE_ATTEMPTS=many"}},
		{name: "Error: empty timeout", url: "https://example.invalid/", env: []string{"PROBE_TIMEOUT="}},
		{name: "Error: negative interval", url: "https://example.invalid/", env: []string{"PROBE_INTERVAL=-1"}},
		{name: "Error: non-numeric interval", url: "https://example.invalid/", env: []string{"PROBE_INTERVAL=soon"}},
		{name: "Error: empty interval", url: "https://example.invalid/", env: []string{"PROBE_INTERVAL="}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "calls.log")
			bin := fakeBin(t, map[string]string{"curl": `exit 0`})
			env := append([]string{fakePATH(bin), "FAKE_LOG=" + logPath}, tc.env...)

			res := runScript(t, probeScript, env, "", tc.url)

			assert.Equal(t, usageExit, res.code, res.output)
			assert.Empty(t, readLog(t, logPath), "curl must not run for invalid input")
		})
	}
}

// curlFake fails its first FAKE_FAILS calls and succeeds afterwards, counting
// calls in FAKE_COUNT.
const curlFake = `
n=$(cat "$FAKE_COUNT" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" >"$FAKE_COUNT"
(( n > FAKE_FAILS ))`

func TestProbeRetriesBoundedTimes(t *testing.T) {
	tests := []struct {
		name      string
		fails     string
		attempts  string
		wantCode  int
		wantCalls int
		wantOut   string
	}{
		{name: "Happy path: healthy on first try", fails: "0", attempts: "3", wantCode: 0, wantCalls: 1, wantOut: "attempt 1"},
		{name: "Happy path: retry then pass", fails: "2", attempts: "3", wantCode: 0, wantCalls: 3, wantOut: "attempt 3"},
		{name: "Edge: failing on the last allowed attempt still fails", fails: "3", attempts: "3", wantCode: 1, wantCalls: 3, wantOut: "probe failed"},
		{name: "Edge: single attempt does not retry", fails: "1", attempts: "1", wantCode: 1, wantCalls: 1, wantOut: "probe failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "calls.log")
			countPath := filepath.Join(dir, "count")
			bin := fakeBin(t, map[string]string{"curl": curlFake, "sleep": `exit 0`})
			env := []string{
				fakePATH(bin), "FAKE_LOG=" + logPath, "FAKE_COUNT=" + countPath, "FAKE_FAILS=" + tc.fails,
				"PROBE_ATTEMPTS=" + tc.attempts, "PROBE_INTERVAL=0", "PROBE_TIMEOUT=7",
			}

			res := runScript(t, probeScript, env, "", "https://example.invalid/health")

			assert.Equal(t, tc.wantCode, res.code, res.output)
			assert.Contains(t, res.output, tc.wantOut)
			assert.Equal(t, tc.wantCalls, strings.Count(readLog(t, logPath), "curl "), "number of attempts")
			assert.Contains(t, readLog(t, logPath), "--max-time 7")
		})
	}
}

func TestCheckEnvKeysMatchesExample(t *testing.T) {
	const example = `# documented
MONGO_PASSWORD=change-me
export-free=ignored
API_URL=https://example.invalid
`
	tests := []struct {
		name     string
		stdin    string
		wantCode int
		wantOut  string
	}{
		{
			name:     "Happy path: same keys with different values",
			stdin:    "MONGO_PASSWORD=real\nAPI_URL=https://legnaris.example.invalid\n",
			wantCode: 0,
		},
		{
			name:     "Edge: values may contain equals signs and comments are ignored",
			stdin:    "# note\nMONGO_PASSWORD=a=b=c\n\nAPI_URL=https://x?y=1\n",
			wantCode: 0,
		},
		{
			name:     "Error: a key is missing",
			stdin:    "MONGO_PASSWORD=real\n",
			wantCode: 1,
			wantOut:  "missing in production env: API_URL",
		},
		{
			name:     "Error: an unknown key is present",
			stdin:    "MONGO_PASSWORD=real\nAPI_URL=x\nSTRAY=1\n",
			wantCode: 1,
			wantOut:  "unknown in production env: STRAY",
		},
		{
			name:     "Edge: a commented-out key counts as missing",
			stdin:    "MONGO_PASSWORD=real\n# API_URL=x\n",
			wantCode: 1,
			wantOut:  "missing in production env: API_URL",
		},
		{
			name:     "Edge: empty production file reports every key missing",
			stdin:    "",
			wantCode: 1,
			wantOut:  "missing in production env: MONGO_PASSWORD",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			examplePath := filepath.Join(t.TempDir(), "env.example")
			require.NoError(t, os.WriteFile(examplePath, []byte(example), 0o644))
			env := []string{"PATH=" + os.Getenv("PATH")}

			res := runScript(t, envKeysScript, env, tc.stdin, examplePath)

			assert.Equal(t, tc.wantCode, res.code, res.output)
			if tc.wantOut != "" {
				assert.Contains(t, res.output, tc.wantOut)
			}
		})
	}
}

func TestCheckEnvKeysRejectsBadUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "Edge: no example file argument", args: nil},
		{name: "Edge: example file does not exist", args: []string{filepath.Join(t.TempDir(), "absent.example")}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := []string{"PATH=" + os.Getenv("PATH")}

			res := runScript(t, envKeysScript, env, "MONGO_PASSWORD=x\n", tc.args...)

			assert.Equal(t, usageExit, res.code, res.output)
		})
	}
}
