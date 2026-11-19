package mongouri_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/scripts/ci/mongouri"
)

func TestFindings(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    []string
	}{
		{
			name:    "Happy path: a development URI in .env carries every parameter",
			path:    ".env.example",
			content: "DATABASE_URI=mongodb://root:legnaris-dev-only@localhost:27017/?replicaSet=rs0&retryWrites=true&authSource=admin\n",
			want:    nil,
		},
		{
			name:    "Happy path: a compose URI with an expanded password carries every parameter",
			path:    "deploy/docker-compose.yml",
			content: "      DATABASE_URI: mongodb://legnaris_app:${MONGO_APP_PASSWORD:?MONGO_APP_PASSWORD is required}@mongodb:27017/legnaris?replicaSet=rs0&retryWrites=true&authSource=legnaris\n",
			want:    nil,
		},
		{
			name:    "Happy path: a quoting shell export carries every parameter",
			path:    "scripts/dev.sh",
			content: "export DATABASE_URI=\"mongodb://$MONGO_ROOT_USER:$MONGO_ROOT_PASSWORD@localhost:$MONGO_PORT/?replicaSet=rs0&retryWrites=true&authSource=admin\"\n",
			want:    nil,
		},
		{
			name:    "Happy path: a mongodb+srv URI carrying every parameter passes",
			path:    "deploy/.env.example",
			content: "DATABASE_URI=mongodb+srv://app:pw@cluster.example.com/legnaris?retryWrites=true&replicaSet=rs0&authSource=legnaris\n",
			want:    nil,
		},
		{
			name:    "Edge: query separators in another order still pass",
			path:    ".env.example",
			content: "DATABASE_URI=mongodb://root:pw@localhost:27017/?authSource=admin;retryWrites=true;replicaSet=rs0\n",
			want:    nil,
		},
		{
			name:    "Edge: a file without a MongoDB URI has nothing to report",
			path:    ".env.example",
			content: "# MONGO_INITDB_ROOT_USERNAME=root\nREDIS_URL=redis://cache:6379\n",
			want:    nil,
		},
		{
			name:    "Edge: YAML keys mentioning mongodb are not URIs",
			path:    "deploy/docker-compose.yml",
			content: "  mongodb:\n    image: mongo:7.0\n",
			want:    nil,
		},
		{
			name:    "Error: a URI without replicaSet is reported",
			path:    ".env.example",
			content: "DATABASE_URI=mongodb://root:pw@localhost:27017/?retryWrites=true&authSource=admin\n",
			want:    []string{".env.example:1: missing replicaSet="},
		},
		{
			name:    "Error: retryWrites=false is reported",
			path:    "deploy/docker-compose.yml",
			content: "      DATABASE_URI: mongodb://root:pw@mongodb:27017/?replicaSet=rs0&retryWrites=false&authSource=admin\n",
			want:    []string{"deploy/docker-compose.yml:1: missing retryWrites=true"},
		},
		{
			name:    "Error: an empty authSource is reported",
			path:    "deploy/.env.example",
			content: "DATABASE_URI=mongodb://root:pw@mongodb:27017/?replicaSet=rs0&retryWrites=true&authSource=\n",
			want:    []string{"deploy/.env.example:1: missing authSource="},
		},
		{
			name:    "Error: a URI with no parameters reports all three",
			path:    "deploy/.env.example",
			content: "DATABASE_URI=\"mongodb://root:pw@localhost:27017/legnaris\"\n",
			want:    []string{"deploy/.env.example:1: missing replicaSet=, retryWrites=true, authSource="},
		},
		{
			name:    "Error: two URIs on one line are checked separately",
			path:    ".env.example",
			content: "DATABASE_URI=mongodb://root:pw@a:27017/?replicaSet=rs0&retryWrites=true&authSource=admin mirror=mongodb://root:pw@b:27017/\n",
			want:    []string{".env.example:1: missing replicaSet=, retryWrites=true, authSource="},
		},
		{
			name:    "Error: the failing line number is reported",
			path:    "scripts/dev.sh",
			content: "#!/bin/sh\n\nexport DATABASE_URI=\"mongodb://root:pw@localhost:27017/legnaris?replicaSet=rs0\"\n",
			want:    []string{"scripts/dev.sh:3: missing retryWrites=true, authSource="},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, finding := range mongouri.Findings(tc.path, []byte(tc.content)) {
				got = append(got, finding.String())
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsConfigPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "Happy path: the checked-in environment file", path: ".env.example", want: true},
		{name: "Happy path: the deploy environment file", path: "deploy/.env.example", want: true},
		{name: "Happy path: a compose file", path: "deploy/docker-compose.yml", want: true},
		{name: "Happy path: a compose override", path: "deploy/docker-compose.vps.yml", want: true},
		{name: "Happy path: a YAML workflow", path: ".github/workflows/ci.yml", want: true},
		{name: "Happy path: the dev stack script", path: "scripts/dev.sh", want: true},
		{name: "Happy path: a deploy script", path: "deploy/vps/deploy.sh", want: true},
		{name: "Happy path: a Dockerfile", path: "Dockerfile", want: true},
		{
			name: "Edge: markdown documentation is not shipped config",
			path: "docs/architecture.md",
			want: false,
		},
		{
			name: "Edge: the pre-init provisioning scripts cannot name the replica set",
			path: "deploy/mongodb/bootstrap.sh",
			want: false,
		},
		{name: "Edge: a Go source is not config", path: "backend/shared/db/database.go", want: false},
		{name: "Edge: a Grafana dashboard is not config", path: "deploy/grafana/dashboards/api.json", want: false},
		{name: "Edge: the Makefile is a build tool", path: "Makefile", want: false},
		{name: "Edge: a fixture capture script that only forwards DATABASE_URI is config too", path: "web/src/api/fixtures/capture.sh", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, mongouri.IsConfigPath(tc.path))
		})
	}
}

func TestTrackedConfig(t *testing.T) {
	t.Run("Happy path: every tracked MongoDB URI names replicaSet, retryWrites=true and authSource", func(t *testing.T) {
		root, files := trackedFiles(t)

		var findings []string
		for _, p := range files {
			if !mongouri.IsConfigPath(p) {
				continue
			}
			content, err := os.ReadFile(filepath.Join(root, p))
			require.NoError(t, err)
			for _, finding := range mongouri.Findings(p, content) {
				findings = append(findings, finding.String())
			}
		}

		assert.NotEmpty(t, files)
		assert.Empty(t, findings)
	})
}

func trackedFiles(t *testing.T) (string, []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed; the tracked configuration cannot be enumerated")
	}
	toplevel, err := exec.CommandContext(t.Context(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("the source tree is not a Git checkout; the tracked configuration cannot be enumerated")
	}
	root := strings.TrimSpace(string(toplevel))
	output, err := exec.CommandContext(t.Context(), "git", "-C", root, "ls-files", "-z").Output()
	require.NoError(t, err)

	var files []string
	for _, name := range strings.Split(string(output), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return root, files
}
