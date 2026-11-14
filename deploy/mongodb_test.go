package deploy_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// replicaSetInitScript is the first-run script that initiates rs0; the Compose
// stacks mount it into /docker-entrypoint-initdb.d.
const replicaSetInitScript = "mongodb/init/00-replset.js"

// keyfileScript provisions the replica-set keyfile before mongod starts.
const keyfileScript = "mongodb/keyfile.sh"

// readComposeAt parses a Compose file in the deploy directory, including the
// top-level volumes that composeDoc does not model.
func readComposeAt(t *testing.T, path string) (composeDoc, map[string]any) {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read %s", path)

	var doc struct {
		composeDoc `yaml:",inline"`
		Volumes    map[string]any `yaml:"volumes"`
	}
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", path)
	return doc.composeDoc, doc.Volumes
}

// hasVolume reports whether a service mounts the given "source:target[:mode]"
// entry verbatim.
func hasVolume(volumes []string, entry string) bool {
	for _, v := range volumes {
		if v == entry {
			return true
		}
	}
	return false
}

// hasFlagPair reports whether args carries flag immediately followed by value.
func hasFlagPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// TestMongoServicesJoinTheReplicaSet guards ADR 0013: every MongoDB service in a
// Compose stack runs as replica set rs0, names its own member address, and runs
// the first-run init script that initiates the set.
func TestMongoServicesJoinTheReplicaSet(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		service string
		member  string
	}{
		{name: "Happy path: production stack joins rs0 at the internal address", file: "docker-compose.yml", service: "mongodb", member: "mongodb:27017"},
		{name: "Happy path: load-test stack joins rs0 at the internal address", file: "docker-compose.dev-observability.yml", service: "mongodb", member: "mongodb:27017"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := readComposeAt(t, tc.file)
			svc, ok := doc.Services[tc.service]
			require.Truef(t, ok, "%s must define the %s service", tc.file, tc.service)

			assert.Truef(t, hasFlagPair(svc.Command, "--replSet", "rs0"),
				"%s command must run --replSet rs0, got %v", tc.service, svc.Command)
			assert.Equalf(t, tc.member, svc.Environment["MONGO_RS_MEMBER"],
				"%s must name its member address; the replica-set config must match what clients connect to", tc.service)
			assert.Truef(t, mountsInitDir(svc.Volumes),
				"%s must mount the init scripts into /docker-entrypoint-initdb.d so rs.initiate runs on first start", tc.service)
		})
	}
}

// mountsInitDir reports whether any volume mounts a host path into the
// first-run directory of the official MongoDB image.
func mountsInitDir(volumes []string) bool {
	for _, v := range volumes {
		if strings.HasSuffix(v, ":/docker-entrypoint-initdb.d:ro") || strings.HasSuffix(v, ":/docker-entrypoint-initdb.d/00-replset.js:ro") {
			return true
		}
	}
	return false
}

// TestProductionMongoUsesAKeyfileProvisionedOnFirstRun guards the production
// keyfile path. A keyfile turns on authorization, so the set and the users are
// both established on the same first run.
func TestProductionMongoUsesAKeyfileProvisionedOnFirstRun(t *testing.T) {
	doc, topVolumes := readComposeAt(t, "docker-compose.yml")

	t.Run("Happy path: mongod reads the keyfile from the shared volume", func(t *testing.T) {
		mongo := doc.Services["mongodb"]
		assert.Truef(t, hasFlagPair(mongo.Command, "--keyFile", "/etc/mongo-keyfile/keyfile"),
			"mongodb command must pass --keyFile, got %v", mongo.Command)
		assert.True(t, hasVolume(mongo.Volumes, "mongo_keyfile:/etc/mongo-keyfile:ro"),
			"mongodb must mount the keyfile volume read-only")
	})

	t.Run("Happy path: mongod waits for the keyfile service to finish", func(t *testing.T) {
		dep, ok := doc.Services["mongodb"].DependsOn["mongodb-keyfile"].(map[string]any)
		require.True(t, ok, "mongodb must depend on mongodb-keyfile in the long form")
		assert.Equal(t, "service_completed_successfully", dep["condition"])
	})

	t.Run("Edge: the keyfile service writes the volume and reruns the idempotent script", func(t *testing.T) {
		keyfile, ok := doc.Services["mongodb-keyfile"]
		require.True(t, ok, "the stack must define the mongodb-keyfile service")
		assert.Equal(t, "mongo:7.0", keyfile.Image)
		assert.Equal(t, []string{"sh", "/provision/keyfile.sh", "/etc/mongo-keyfile"}, keyfile.Entrypoint)
		assert.True(t, hasVolume(keyfile.Volumes, "./mongodb/keyfile.sh:/provision/keyfile.sh:ro"),
			"mongodb-keyfile must mount the provisioning script")
		assert.True(t, hasVolume(keyfile.Volumes, "mongo_keyfile:/etc/mongo-keyfile"),
			"mongodb-keyfile must write the keyfile volume")
		assert.Equal(t, "no", keyfile.Restart)
	})

	t.Run("Error: the keyfile volume is declared at the top level", func(t *testing.T) {
		_, declared := topVolumes["mongo_keyfile"]
		assert.True(t, declared, "volumes must declare mongo_keyfile")
	})
}

// TestReplicaSetInitScriptIsIdempotent guards the init script contract: it reads
// the set state before it initiates, and it fails loudly when no primary emerges.
func TestReplicaSetInitScriptIsIdempotent(t *testing.T) {
	raw, err := os.ReadFile(replicaSetInitScript)
	require.NoError(t, err)
	script := string(raw)

	t.Run("Edge: rs.initiate runs only when the member does not report rs0", func(t *testing.T) {
		guard := strings.Index(script, "db.hello().setName !== RS_NAME")
		initiate := strings.Index(script, "rs.initiate(")
		require.NotEqual(t, -1, guard, "the init script must check the set name")
		require.NotEqual(t, -1, initiate, "the init script must initiate the set")
		assert.Less(t, guard, initiate, "the set-name check must come before rs.initiate")
	})

	t.Run("Error: the script fails when no primary is elected", func(t *testing.T) {
		assert.Contains(t, script, "throw new Error(", "the script must throw when rs0 has no primary")
	})
}

// TestKeyfileScriptProvisionsOnce runs the real provisioning script. The owner is
// the current user so the test runs without root; the production default is 999.
func TestKeyfileScriptProvisionsOnce(t *testing.T) {
	owner := currentOwner(t)

	t.Run("Happy path: writes a base64 keyfile readable only by its owner", func(t *testing.T) {
		dir := t.TempDir()
		res := runScript(t, keyfileScript, keyfileEnv(owner), "", dir)
		require.Equal(t, 0, res.code, res.output)

		info, err := os.Stat(filepath.Join(dir, "keyfile"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o400), info.Mode().Perm())

		content := readFile(t, filepath.Join(dir, "keyfile"))
		assert.GreaterOrEqual(t, len(content), 6, "mongod rejects keyfiles shorter than 6 characters")
		assert.LessOrEqual(t, len(content), 1024, "mongod rejects keyfiles longer than 1024 characters")
		assert.Regexp(t, `^[A-Za-z0-9+/=]+$`, content)
	})

	t.Run("Edge: a second run keeps the existing keyfile", func(t *testing.T) {
		dir := t.TempDir()
		require.Equal(t, 0, runScript(t, keyfileScript, keyfileEnv(owner), "", dir).code)
		first := readFile(t, filepath.Join(dir, "keyfile"))

		res := runScript(t, keyfileScript, keyfileEnv(owner), "", dir)
		require.Equal(t, 0, res.code, res.output)
		assert.Equal(t, first, readFile(t, filepath.Join(dir, "keyfile")), "rerunning must not rotate the keyfile")
	})

	t.Run("Edge: an empty leftover keyfile is replaced", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "keyfile"), nil, 0o600))

		res := runScript(t, keyfileScript, keyfileEnv(owner), "", dir)
		require.Equal(t, 0, res.code, res.output)
		assert.NotEmpty(t, readFile(t, filepath.Join(dir, "keyfile")))
	})

	t.Run("Error: a missing directory argument fails without writing", func(t *testing.T) {
		res := runScript(t, keyfileScript, keyfileEnv(owner), "")
		assert.NotEqual(t, 0, res.code, "the script must reject a missing directory")
	})
}

// currentOwner returns the "uid:gid" of the test process, the owner the script
// assigns when KEYFILE_OWNER is set.
func currentOwner(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}

// keyfileEnv runs the script with an explicit owner and the host PATH.
func keyfileEnv(owner string) []string {
	return []string{"PATH=" + os.Getenv("PATH"), "KEYFILE_OWNER=" + owner}
}

// readFile returns a file's contents as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}
