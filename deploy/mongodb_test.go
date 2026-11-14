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

// replicaSetInitScript initiates rs0; bootstrap.sh runs it first, unauthenticated.
const replicaSetInitScript = "mongodb/init/00-replset.js"

// usersInitScript provisions the application users; bootstrap.sh runs it as root.
const usersInitScript = "mongodb/init/01-create-users.js"

// mongoBootstrapScript brings a started member to its first-run state; every stack runs it.
const mongoBootstrapScript = "mongodb/bootstrap.sh"

// keyfileScript provisions the replica-set keyfile before mongod starts.
const keyfileScript = "mongodb/keyfile.sh"

// mongoKeyfileArg and mongoKeyfileVolume are the keyfile path inside mongod and the
// read-only mount that carries it, shared by every stack.
const (
	mongoKeyfileArg    = "/etc/mongo-keyfile/keyfile"
	mongoKeyfileVolume = "mongo_keyfile:/etc/mongo-keyfile:ro"
)

// credentialedDatabaseURI matches a MongoDB URI on the internal network that carries
// a username and password, the only form a keyfile-enabled member accepts.
var credentialedDatabaseURI = `^mongodb://[^:/@\s]+:[^@]+@mongodb:27017/`

// mongoStack is a Compose stack that runs MongoDB as replica set rs0 with a keyfile.
type mongoStack struct {
	name string
	file string
	// member is the address the member is reached at, which the replica-set config must name.
	member string
	// usersScript reports whether bootstrap.sh provisions the application users.
	usersScript bool
	// dependents are the services that read the database and wait for the bootstrap.
	dependents []string
}

// mongoStacks lists every Compose stack that runs MongoDB (ADR 0013). Each one gets
// the same keyfile and bootstrap rules; the dev scripts are checked separately.
var mongoStacks = []mongoStack{
	{
		name:        "production",
		file:        "docker-compose.yml",
		member:      "mongodb:27017",
		usersScript: true,
		dependents:  []string{"app", "relay", "mongodb-exporter", "mongo-express"},
	},
	{
		name:       "dev observability and load test",
		file:       "docker-compose.dev-observability.yml",
		member:     "mongodb:27017",
		dependents: []string{"app"},
	},
}

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

// TestEveryMongoStackRunsAReplicaSetWithAKeyfile guards ADR 0013 for every stack: mongod
// runs rs0 with a keyfile, starts without root credentials, and waits for a keyfile
// one-shot. Authorization is on everywhere, so no stack may run unauthenticated.
func TestEveryMongoStackRunsAReplicaSetWithAKeyfile(t *testing.T) {
	for _, stack := range mongoStacks {
		t.Run(stack.name, func(t *testing.T) {
			doc, topVolumes := readComposeAt(t, stack.file)
			mongo, ok := doc.Services["mongodb"]
			require.Truef(t, ok, "%s must define the mongodb service", stack.file)

			t.Run("Happy path: mongod runs rs0 with the keyfile from the shared volume", func(t *testing.T) {
				assert.Truef(t, hasFlagPair(mongo.Command, "--replSet", "rs0"),
					"command must run --replSet rs0, got %v", mongo.Command)
				assert.Truef(t, hasFlagPair(mongo.Command, "--keyFile", mongoKeyfileArg),
					"command must pass --keyFile %s, got %v", mongoKeyfileArg, mongo.Command)
				assert.Truef(t, hasVolume(mongo.Volumes, mongoKeyfileVolume),
					"mongodb must mount %s", mongoKeyfileVolume)
			})

			t.Run("Edge: mongod starts without root credentials, so the image's init server never drops --replSet or --keyFile", func(t *testing.T) {
				for key := range mongo.Environment {
					assert.Falsef(t, strings.HasPrefix(key, "MONGO_INITDB_ROOT_"),
						"mongodb must not set %s: the image would then run its temporary init server without the set", key)
				}
				for _, v := range mongo.Volumes {
					assert.NotContainsf(t, v, "/docker-entrypoint-initdb.d",
						"init scripts run through bootstrap.sh, never the image entrypoint: %s", v)
				}
			})

			t.Run("Edge: the healthcheck pings without credentials, so it passes before the root user exists", func(t *testing.T) {
				require.NotNil(t, mongo.Healthcheck)
				assert.NotContains(t, strings.Join(mongo.Healthcheck.Test, " "), " -u ")
			})

			t.Run("Happy path: mongod waits for the keyfile one-shot to finish", func(t *testing.T) {
				dep, ok := mongo.DependsOn["mongodb-keyfile"].(map[string]any)
				require.True(t, ok, "mongodb must depend on mongodb-keyfile in the long form")
				assert.Equal(t, "service_completed_successfully", dep["condition"])
			})

			t.Run("Edge: the keyfile one-shot provisions the volume and never restarts", func(t *testing.T) {
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
		})
	}
}

// TestEveryMongoStackBootstrapsAfterTheMemberStarts guards the first-run order: the
// one-shot runs in the member's network namespace (a localhost connection), initiates
// rs0 before anything else, and gates every service that reads the database.
func TestEveryMongoStackBootstrapsAfterTheMemberStarts(t *testing.T) {
	for _, stack := range mongoStacks {
		t.Run(stack.name, func(t *testing.T) {
			doc, _ := readComposeAt(t, stack.file)
			boot, ok := doc.Services["mongodb-bootstrap"]
			require.Truef(t, ok, "%s must define the mongodb-bootstrap service", stack.file)

			t.Run("Happy path: the one-shot runs bootstrap.sh with rs0 first", func(t *testing.T) {
				require.GreaterOrEqual(t, len(boot.Command), 3, "command must name the script and the replica-set file")
				assert.Equal(t, []string{"sh", "/provision/bootstrap.sh", "/provision/init/00-replset.js"}, boot.Command[:3])
				assert.Equal(t, stack.usersScript, commandRunsScript(boot.Command, "/provision/init/01-create-users.js"),
					"the user script runs only in stacks that provision application users")
			})

			t.Run("Edge: the one-shot shares the member's network namespace so its connection is a localhost connection", func(t *testing.T) {
				assert.Equal(t, "service:mongodb", boot.NetworkMode)
				assert.Empty(t, boot.Networks, "a service with network_mode cannot also join networks")
			})

			t.Run("Edge: the member address the set names is the address clients use", func(t *testing.T) {
				assert.Equal(t, stack.member, boot.Environment["MONGO_RS_MEMBER"])
			})

			t.Run("Happy path: the root user name is root and its password is set", func(t *testing.T) {
				assert.Equal(t, "root", boot.Environment["MONGO_INITDB_ROOT_USERNAME"])
				assert.NotEmpty(t, boot.Environment["MONGO_INITDB_ROOT_PASSWORD"])
			})

			t.Run("Happy path: the bootstrap scripts are mounted read-only", func(t *testing.T) {
				assert.True(t, hasVolume(boot.Volumes, "./mongodb:/provision:ro"))
			})

			t.Run("Edge: the one-shot waits for a healthy member and runs once per start", func(t *testing.T) {
				assert.Equal(t, "no", boot.Restart)
				dep, ok := boot.DependsOn["mongodb"].(map[string]any)
				require.True(t, ok, "mongodb-bootstrap must depend on mongodb in the long form")
				assert.Equal(t, "service_healthy", dep["condition"])
			})

			for _, name := range stack.dependents {
				t.Run("Error: "+name+" starts only after the bootstrap completes", func(t *testing.T) {
					svc, ok := doc.Services[name]
					require.Truef(t, ok, "%s must define the %s service", stack.file, name)
					dep, ok := svc.DependsOn["mongodb-bootstrap"].(map[string]any)
					require.Truef(t, ok, "%s must depend on mongodb-bootstrap", name)
					assert.Equal(t, "service_completed_successfully", dep["condition"])
				})
			}

			t.Run("Error: the app authenticates to the member with a credentialed URI", func(t *testing.T) {
				assert.Regexp(t, credentialedDatabaseURI, doc.Services["app"].Environment["DATABASE_URI"])
			})
		})
	}
}

// commandRunsScript reports whether a one-shot's command passes the given script path.
func commandRunsScript(command []string, script string) bool {
	for _, arg := range command {
		if arg == script {
			return true
		}
	}
	return false
}

// TestReplicaSetInitScriptIsIdempotent guards the replica-set script: it reads the set
// state before it initiates, and it fails loudly when no primary emerges.
func TestReplicaSetInitScriptIsIdempotent(t *testing.T) {
	script := readFile(t, replicaSetInitScript)

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

// TestUsersInitScriptIsIdempotent guards the user script, which bootstrap.sh runs on
// every start: a user that exists must be left untouched, never recreated or rotated.
func TestUsersInitScriptIsIdempotent(t *testing.T) {
	script := readFile(t, usersInitScript)

	t.Run("Edge: every user is created only when it does not already exist", func(t *testing.T) {
		assert.Equal(t, 1, strings.Count(script, "createUser("), "createUser must appear once, inside ensureUser")
		assert.Contains(t, script, "database.getUser(spec.user) !== null")
	})

	t.Run("Edge: the users are not rotated on restart", func(t *testing.T) {
		assert.NotContains(t, script, "updateUser(")
	})
}

// bootstrapEnv returns the environment bootstrap.sh runs with. The fake mongosh reads
// the FAKE_* switches: FAKE_ROOT=exists makes the root login succeed, FAKE_CREATE=fail
// makes createUser fail, FAKE_REPLSET=fail makes the replica-set script fail and
// FAKE_USERS=fail makes the authenticated user script fail. Overrides replace keys.
func bootstrapEnv(overrides ...string) []string {
	defaults := map[string]string{
		"MONGO_INITDB_ROOT_USERNAME": "root",
		"MONGO_INITDB_ROOT_PASSWORD": "dev-only-pw",
		"FAKE_ROOT":                  "missing",
		"FAKE_CREATE":                "ok",
		"FAKE_REPLSET":               "ok",
		"FAKE_USERS":                 "ok",
	}
	for _, kv := range overrides {
		key, value, _ := strings.Cut(kv, "=")
		defaults[key] = value
	}
	env := make([]string, 0, len(defaults))
	for key, value := range defaults {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

// fakeMongoshBody routes each mongosh call by its arguments, so the test sees the call
// order without a server. A probe is any call authenticated as root without a file.
const fakeMongoshBody = `case "$*" in
  *"-u root"*"--file"*) [ "$FAKE_USERS" != fail ] ;;
  *"-u root"*) [ "$FAKE_ROOT" = exists ] ;;
  *createUser*) [ "$FAKE_CREATE" != fail ] ;;
  *00-replset*) [ "$FAKE_REPLSET" != fail ] ;;
esac`

// runBootstrap runs bootstrap.sh with the fake mongosh and returns the result and the
// recorded mongosh invocations, one per entry.
func runBootstrap(t *testing.T, env []string, args ...string) (scriptResult, []string) {
	t.Helper()
	scriptEnvironment, logPath := scriptEnv(t, map[string]string{"mongosh": fakeMongoshBody})
	res := runScript(t, mongoBootstrapScript, append(scriptEnvironment, env...), "", args...)
	return res, logLines(readLog(t, logPath))
}

// logLines splits the fake command log into one entry per invocation.
func logLines(log string) []string {
	trimmed := strings.TrimSpace(log)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// TestBootstrapScriptOrdersReplicaSetRootAndUsers runs the real bootstrap.sh against a
// fake mongosh. It covers the first run, the restart and the failure paths, asserting
// that the replica set comes first, the root user is created only when the root login
// fails, and the user scripts run only after the root login works.
func TestBootstrapScriptOrdersReplicaSetRootAndUsers(t *testing.T) {
	const replset = "/provision/init/00-replset.js"
	const users = "/provision/init/01-create-users.js"

	tests := []struct {
		name      string
		env       []string
		args      []string
		wantCode  int
		wantCalls []string
	}{
		{
			name:     "Happy path: first run initiates rs0, creates root, then runs the user scripts as root",
			args:     []string{replset, users},
			wantCode: 0,
			wantCalls: []string{
				"replset:" + replset,
				"probe",
				"create-root",
				"users:" + users,
			},
		},
		{
			name:     "Edge: a restart with root already present creates no root user",
			env:      bootstrapEnv("FAKE_ROOT=exists"),
			args:     []string{replset, users},
			wantCode: 0,
			wantCalls: []string{
				"replset:" + replset,
				"probe",
				"users:" + users,
			},
		},
		{
			name:     "Edge: with no user script only the replica set and the root user are set up",
			args:     []string{replset},
			wantCode: 0,
			wantCalls: []string{
				"replset:" + replset,
				"probe",
				"create-root",
			},
		},
		{
			name:     "Error: a missing root username stops before any mongosh call",
			env:      bootstrapEnv("MONGO_INITDB_ROOT_USERNAME="),
			args:     []string{replset, users},
			wantCode: 1,
		},
		{
			name:     "Error: a missing replica-set script argument fails with the usage message",
			args:     nil,
			wantCode: 1,
		},
		{
			name:     "Error: a failing replica-set initiation never creates the root user",
			env:      bootstrapEnv("FAKE_REPLSET=fail"),
			args:     []string{replset, users},
			wantCode: 1,
			wantCalls: []string{
				"replset:" + replset,
			},
		},
		{
			name:     "Error: a root user that cannot be created fails loudly and runs no user script",
			env:      bootstrapEnv("FAKE_CREATE=fail"),
			args:     []string{replset, users},
			wantCode: 1,
			wantCalls: []string{
				"replset:" + replset,
				"probe",
				"create-root",
			},
		},
		{
			name:     "Error: a failing user script fails the run",
			env:      bootstrapEnv("FAKE_USERS=fail"),
			args:     []string{replset, users},
			wantCode: 1,
			wantCalls: []string{
				"replset:" + replset,
				"probe",
				"create-root",
				"users:" + users,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := tc.env
			if env == nil {
				env = bootstrapEnv()
			}
			res, calls := runBootstrap(t, env, tc.args...)
			require.Equal(t, tc.wantCode, res.code, res.output)
			if tc.wantCode == 1 && tc.wantCalls == nil {
				assert.Empty(t, calls, "a failing precondition must not reach mongosh")
				return
			}
			assert.Equal(t, normalizeCalls(tc.wantCalls), normalizeCalls(callKinds(calls, replset, users)))
		})
	}
}

// callKinds reduces each recorded mongosh invocation to the step it performed, so the
// assertions read the order rather than the full connection arguments.
func callKinds(calls []string, replset, users string) []string {
	kinds := make([]string, 0, len(calls))
	for _, call := range calls {
		switch {
		case strings.Contains(call, replset) && !strings.Contains(call, "-u root"):
			kinds = append(kinds, "replset:"+replset)
		case strings.Contains(call, "-u root") && strings.Contains(call, "--file "+users):
			kinds = append(kinds, "users:"+users)
		case strings.Contains(call, "-u root"):
			kinds = append(kinds, "probe")
		case strings.Contains(call, "createUser"):
			kinds = append(kinds, "create-root")
		}
	}
	return kinds
}

// normalizeCalls returns a nil slice for an empty list so expected and actual compare equal.
func normalizeCalls(calls []string) []string {
	if len(calls) == 0 {
		return nil
	}
	return calls
}

// TestDevEntrypointsUseTheKeyfileAndCredentials guards the scripts that start MongoDB
// outside Compose: dev.sh and `make dev-db` must run rs0 with the keyfile, bootstrap
// it, and never accept a URI without credentials.
func TestDevEntrypointsUseTheKeyfileAndCredentials(t *testing.T) {
	tests := []struct {
		name string
		file string
		want []string
	}{
		{
			name: "Happy path: dev.sh runs rs0 with the keyfile, bootstraps it and exports credentials",
			file: "../scripts/dev.sh",
			want: []string{
				"--replSet rs0 --keyFile /etc/mongo-keyfile/keyfile",
				"sh /provision/bootstrap.sh /provision/init/00-replset.js",
				`DATABASE_URI="mongodb://$MONGO_ROOT_USER:$MONGO_ROOT_PASSWORD@localhost:$MONGO_PORT/"`,
			},
		},
		{
			name: "Happy path: make dev-db runs rs0 with the keyfile and bootstraps it",
			file: "../Makefile",
			want: []string{
				"--replSet rs0 --keyFile /etc/mongo-keyfile/keyfile",
				"sh /provision/bootstrap.sh /provision/init/00-replset.js",
			},
		},
		{
			name: "Edge: make dev-db refuses a DATABASE_URI without credentials",
			file: "../Makefile",
			want: []string{"DATABASE_URI must carry the root credentials"},
		},
		{
			name: "Edge: the example DATABASE_URI carries credentials, so make dev-db and the app agree",
			file: "../.env.example",
			want: []string{"DATABASE_URI=mongodb://root:legnaris-dev-only@localhost:27017/"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := readFile(t, tc.file)
			for _, fragment := range tc.want {
				assert.Containsf(t, content, fragment, "%s must contain %q", tc.file, fragment)
			}
		})
	}
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
