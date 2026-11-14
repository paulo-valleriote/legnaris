package deploy_test

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bootstrapTestImage    = "mongo:7.0"
	bootstrapTestUser     = "root"
	bootstrapTestPassword = "bootstrap-it-pw"
	bootstrapReadyTimeout = 60 * time.Second
)

// dockerRun runs the docker CLI with extra environment entries and returns its
// combined output. Secrets travel through env, never through argv.
func dockerRun(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// requireDocker skips the test when no Docker daemon or the Mongo image is available.
func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not installed — skipping bootstrap integration test")
	}
	if out, err := dockerRun(t, nil, "image", "inspect", bootstrapTestImage); err != nil {
		t.Skipf("docker or %s unavailable — skipping bootstrap integration test: %s", bootstrapTestImage, out)
	}
}

// startBootstrapMember starts a keyfile-protected rs0 member and returns its name.
func startBootstrapMember(t *testing.T) string {
	t.Helper()
	suffix := make([]byte, 4)
	_, err := rand.Read(suffix)
	require.NoError(t, err)
	name := "legnaris-bootstrap-it-" + hex.EncodeToString(suffix)
	volume := name + "-keyfile"
	provision, err := filepath.Abs("mongodb")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = dockerRun(t, nil, "rm", "-fv", name)
		_, _ = dockerRun(t, nil, "volume", "rm", "-f", volume)
	})

	out, err := dockerRun(t, nil, "volume", "create", volume)
	require.NoError(t, err, out)
	out, err = dockerRun(t, nil, "run", "--rm", "--entrypoint", "sh",
		"-v", volume+":/etc/mongo-keyfile", "-v", provision+":/provision:ro",
		bootstrapTestImage, "/provision/keyfile.sh", "/etc/mongo-keyfile")
	require.NoError(t, err, out)
	out, err = dockerRun(t, nil, "run", "-d", "--name", name,
		"-v", volume+":/etc/mongo-keyfile:ro", "-v", provision+":/provision:ro",
		bootstrapTestImage, "--replSet", "rs0", "--keyFile", "/etc/mongo-keyfile/keyfile")
	require.NoError(t, err, out)
	waitForMongo(t, name)
	return name
}

// waitForMongo blocks until mongod answers a ping, which works before any user exists.
func waitForMongo(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(bootstrapReadyTimeout)
	for time.Now().Before(deadline) {
		if _, err := dockerRun(t, nil, "exec", name, "mongosh", "--quiet", "--eval", "db.adminCommand({ping: 1}).ok"); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("mongod in %s did not become ready within %s", name, bootstrapReadyTimeout)
}

// bootstrapMember runs bootstrap.sh inside the member with the root credentials taken
// from the environment, as every real stack does.
func bootstrapMember(t *testing.T, name string) (string, error) {
	t.Helper()
	return dockerRun(t, []string{
		"MONGO_INITDB_ROOT_USERNAME=" + bootstrapTestUser,
		"MONGO_INITDB_ROOT_PASSWORD=" + bootstrapTestPassword,
		"MONGO_APP_PASSWORD=app-it-pw",
		"MONGO_EXPORTER_PASSWORD=exporter-it-pw",
		"MONGO_VIEW_PASSWORD=view-it-pw",
	}, "exec",
		"-e", "MONGO_INITDB_ROOT_USERNAME", "-e", "MONGO_INITDB_ROOT_PASSWORD",
		"-e", "MONGO_APP_PASSWORD", "-e", "MONGO_EXPORTER_PASSWORD", "-e", "MONGO_VIEW_PASSWORD",
		"-e", "MONGO_RS_MEMBER=localhost:27017",
		name, "sh", "/provision/bootstrap.sh",
		"/provision/init/00-replset.js", "/provision/init/01-create-users.js")
}

// memberState reports the replica-set health, the root login and the users that exist.
func memberState(t *testing.T, name string) string {
	t.Helper()
	const probe = `db.getSiblingDB("admin").auth(process.env.MONGO_INITDB_ROOT_USERNAME, process.env.MONGO_INITDB_ROOT_PASSWORD);` +
		`const users = db.getSiblingDB("legnaris").getUsers().users.map(u => u.user).sort().join(",");` +
		`print("set=" + rs.status().set + " ok=" + rs.status().ok + " users=" + users)`
	out, err := dockerRun(t, []string{
		"MONGO_INITDB_ROOT_USERNAME=" + bootstrapTestUser,
		"MONGO_INITDB_ROOT_PASSWORD=" + bootstrapTestPassword,
	}, "exec", "-e", "MONGO_INITDB_ROOT_USERNAME", "-e", "MONGO_INITDB_ROOT_PASSWORD",
		name, "mongosh", "--quiet", "--eval", probe)
	require.NoError(t, err, out)
	return strings.TrimSpace(out)
}

// TestBootstrapAgainstRealReplicaSet runs bootstrap.sh against a real keyfile-protected
// member: first run, a rerun on the initialized set, and a rerun after a container
// restart. The LGN-207 acceptance requires none of them to fail or change the state.
func TestBootstrapAgainstRealReplicaSet(t *testing.T) {
	requireDocker(t)
	name := startBootstrapMember(t)
	const want = "set=rs0 ok=1 users=legnaris_app,legnaris_view"

	t.Run("Happy path: the first run initiates rs0, creates root and the application users", func(t *testing.T) {
		out, err := bootstrapMember(t, name)
		require.NoError(t, err, out)
		assert.Equal(t, want, memberState(t, name))
	})

	t.Run("Edge: running the bootstrap again on an initialized set succeeds and changes nothing", func(t *testing.T) {
		out, err := bootstrapMember(t, name)
		require.NoError(t, err, out)
		assert.Equal(t, want, memberState(t, name))
	})

	t.Run("Edge: restarting the member and bootstrapping again succeeds and changes nothing", func(t *testing.T) {
		out, err := dockerRun(t, nil, "restart", name)
		require.NoError(t, err, out)
		waitForMongo(t, name)

		out, err = bootstrapMember(t, name)
		require.NoError(t, err, out)
		assert.Equal(t, want, memberState(t, name))
	})

	t.Run("Error: a different root password fails loudly instead of overwriting root", func(t *testing.T) {
		out, err := dockerRun(t, []string{
			"MONGO_INITDB_ROOT_USERNAME=" + bootstrapTestUser,
			"MONGO_INITDB_ROOT_PASSWORD=another-pw",
		}, "exec", "-e", "MONGO_INITDB_ROOT_USERNAME", "-e", "MONGO_INITDB_ROOT_PASSWORD",
			"-e", "MONGO_RS_MEMBER=localhost:27017",
			name, "sh", "/provision/bootstrap.sh", "/provision/init/00-replset.js")
		require.Error(t, err, out)
		assert.Equal(t, want, memberState(t, name))
	})
}
