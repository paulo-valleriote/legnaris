package contextdocs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/contextdocs"
)

func checkPathMap() *contextdocs.PathMap {
	return &contextdocs.PathMap{
		Version: 1,
		ContextFiles: []string{
			"context/architecture.md",
			"context/auth.md",
			"context/admin.md",
			"context/platform.md",
			"context/web-client.md",
		},
		Paths: []contextdocs.PathEntry{
			{Target: "backend/internal/app", Kind: contextdocs.KindDirectory, Context: "architecture"},
			{Target: "backend/internal/auth", Kind: contextdocs.KindDirectory, Context: "auth"},
			{Target: "backend/internal/contextdocs", Kind: contextdocs.KindDirectory, Context: "architecture"},
			{Target: "web/src/sheet", Kind: contextdocs.KindDirectory, Context: "web-client"},
			{Target: "deploy", Kind: contextdocs.KindDirectory, Context: "admin"},
			{Target: ".github/workflows", Kind: contextdocs.KindDirectory, Context: "admin"},
			{Target: "go.mod", Kind: contextdocs.KindFile, Context: "platform"},
			{Target: "web/package.json", Kind: contextdocs.KindFile, Context: "platform"},
		},
		Env: []contextdocs.EnvEntry{
			{Key: "JWT_SECRET", Context: "auth"},
			{Key: "PORT", Context: "platform"},
		},
	}
}

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name         string
		changed      []string
		delta        contextdocs.EnvDelta
		exempt       bool
		wantTriggers int
		required     []string
		missing      []string
	}{
		{
			name:         "Happy path: mapped file with its context update",
			changed:      []string{"web/package.json", "context/platform.md"},
			wantTriggers: 1,
			required:     []string{"context/platform.md"},
		},
		{
			name:         "Error: dependency change without the owning context",
			changed:      []string{"web/package.json"},
			wantTriggers: 1,
			required:     []string{"context/platform.md"},
			missing:      []string{"context/platform.md"},
		},
		{
			name:         "Happy path: no-context-change label exempts the check",
			changed:      []string{"web/package.json"},
			exempt:       true,
			wantTriggers: 1,
			required:     []string{"context/platform.md"},
		},
		{
			name:         "Happy path: tests only need no context update",
			changed:      []string{"backend/internal/sheet/engine_test.go", "web/src/sheet/Sheet.test.tsx", "testdata/dice/vectors.json", "web/e2e/smoke.spec.ts"},
			wantTriggers: 0,
		},
		{
			name:         "Happy path: context files only need no context update",
			changed:      []string{"context/auth.md", contextdocs.IndexPath},
			wantTriggers: 0,
		},
		{
			name:         "Error: new backend package without architecture",
			changed:      []string{"backend/internal/newthing/thing.go"},
			wantTriggers: 1,
			required:     []string{"context/architecture.md"},
			missing:      []string{"context/architecture.md"},
		},
		{
			name:         "Happy path: new backend package with architecture",
			changed:      []string{"backend/internal/newthing/thing.go", "context/architecture.md"},
			wantTriggers: 1,
			required:     []string{"context/architecture.md"},
		},
		{
			name:         "Error: new web folder without architecture",
			changed:      []string{"web/src/newarea/View.tsx"},
			wantTriggers: 1,
			required:     []string{"context/architecture.md"},
			missing:      []string{"context/architecture.md"},
		},
		{
			name:         "Error: deploy change without admin",
			changed:      []string{"deploy/docker-compose.yml"},
			wantTriggers: 1,
			required:     []string{"context/admin.md"},
			missing:      []string{"context/admin.md"},
		},
		{
			name:         "Happy path: deploy change with admin",
			changed:      []string{"deploy/docker-compose.yml", "context/admin.md"},
			wantTriggers: 1,
			required:     []string{"context/admin.md"},
		},
		{
			name:         "Error: workflow change without admin",
			changed:      []string{".github/workflows/ci.yml"},
			wantTriggers: 1,
			required:     []string{"context/admin.md"},
			missing:      []string{"context/admin.md"},
		},
		{
			name:         "Error: go.mod change without platform",
			changed:      []string{"go.mod"},
			wantTriggers: 1,
			required:     []string{"context/platform.md"},
			missing:      []string{"context/platform.md"},
		},
		{
			name:         "Happy path: nested file stays owned by its mapped parent",
			changed:      []string{"backend/internal/auth/oauth/token.go", "context/auth.md"},
			wantTriggers: 1,
			required:     []string{"context/auth.md"},
		},
		{
			name:         "Error: route registration without architecture",
			changed:      []string{"backend/internal/app/router.go"},
			wantTriggers: 1,
			required:     []string{"context/architecture.md"},
			missing:      []string{"context/architecture.md"},
		},
		{
			name:         "Happy path: route registration with architecture",
			changed:      []string{"backend/internal/app/router.go", "context/architecture.md"},
			wantTriggers: 1,
			required:     []string{"context/architecture.md"},
		},
		{
			name:         "Error: added env key needs its mapped context",
			delta:        contextdocs.EnvDelta{Added: []string{"JWT_SECRET"}},
			wantTriggers: 1,
			required:     []string{"context/auth.md"},
			missing:      []string{"context/auth.md"},
		},
		{
			name:         "Edge: unmapped env key falls back to platform",
			delta:        contextdocs.EnvDelta{Added: []string{"NEW_FLAG"}},
			wantTriggers: 1,
			required:     []string{"context/platform.md"},
			missing:      []string{"context/platform.md"},
		},
		{
			name:         "Error: removed env key needs its mapped context",
			delta:        contextdocs.EnvDelta{Removed: []string{"PORT"}},
			wantTriggers: 1,
			required:     []string{"context/platform.md"},
			missing:      []string{"context/platform.md"},
		},
		{
			name:         "Happy path: env key change with its mapped context",
			changed:      []string{"context/auth.md"},
			delta:        contextdocs.EnvDelta{Added: []string{"JWT_SECRET"}},
			wantTriggers: 1,
			required:     []string{"context/auth.md"},
		},
		{
			name:         "Edge: empty diff needs nothing",
			changed:      nil,
			wantTriggers: 0,
		},
		{
			name:         "Edge: duplicated and relative paths collapse",
			changed:      []string{"./web/package.json", "web/package.json", " web/package.json "},
			wantTriggers: 1,
			required:     []string{"context/platform.md"},
			missing:      []string{"context/platform.md"},
		},
		{
			name:         "Edge: paths escaping the repository are ignored",
			changed:      []string{"../outside.go", "docs/legal/privacy-policy.en.md"},
			wantTriggers: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := contextdocs.Analyze(checkPathMap(), tc.changed, tc.delta, tc.exempt)
			assert.Len(t, report.Triggers, tc.wantTriggers)
			if tc.required == nil {
				assert.Empty(t, report.Required)
			} else {
				assert.Equal(t, tc.required, report.Required)
			}
			assert.Equal(t, tc.missing, report.Missing)
			assert.Equal(t, tc.exempt || tc.missing == nil, report.OK())
		})
	}
}

func TestAnalyzeNilPathMap(t *testing.T) {
	report := contextdocs.Analyze(nil, []string{"web/package.json"}, contextdocs.EnvDelta{}, false)
	assert.True(t, report.OK())
	assert.Empty(t, report.Triggers)
}

func TestReportSummary(t *testing.T) {
	tests := []struct {
		name   string
		report contextdocs.Report
		want   string
	}{
		{
			name:   "Happy path: no mapped change",
			report: contextdocs.Analyze(checkPathMap(), []string{"docs/legal/privacy-policy.en.md"}, contextdocs.EnvDelta{}, false),
			want:   "No mapped area changed",
		},
		{
			name:   "Happy path: every change documented",
			report: contextdocs.Analyze(checkPathMap(), []string{"web/package.json", "context/platform.md"}, contextdocs.EnvDelta{}, false),
			want:   "Every mapped change has a context update.",
		},
		{
			name:   "Error: missing context is listed",
			report: contextdocs.Analyze(checkPathMap(), []string{"deploy/docker-compose.yml"}, contextdocs.EnvDelta{}, false),
			want:   "Missing context updates",
		},
		{
			name:   "Happy path: exemption is reported",
			report: contextdocs.Analyze(checkPathMap(), []string{"deploy/docker-compose.yml"}, contextdocs.EnvDelta{}, true),
			want:   contextdocs.LabelNoContextChange,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, tc.report.Summary(), tc.want)
		})
	}
}

func TestParseEnvKeyList(t *testing.T) {
	tests := []struct {
		name     string
		makefile string
		want     []string
	}{
		{
			name:     "Happy path: simple assignment",
			makefile: "APP_ENV_KEYS = PORT APP_NAME\n",
			want:     []string{"APP_NAME", "PORT"},
		},
		{
			name:     "Edge: expanded assignment",
			makefile: "APP_ENV_KEYS := PORT\n",
			want:     []string{"PORT"},
		},
		{
			name:     "Edge: duplicates collapse",
			makefile: "APP_ENV_KEYS = PORT PORT APP_NAME\n",
			want:     []string{"APP_NAME", "PORT"},
		},
		{
			name:     "Edge: continuation line",
			makefile: "APP_ENV_KEYS = PORT \\\n  APP_NAME\n",
			want:     []string{"APP_NAME", "PORT"},
		},
		{
			name:     "Edge: trailing comment",
			makefile: "APP_ENV_KEYS = PORT # APP_NAME\n",
			want:     []string{"PORT"},
		},
		{
			name:     "Edge: similar variable is ignored",
			makefile: "APP_ENV_KEYS_EXTRA = PORT\n",
			want:     []string{},
		},
		{
			name:     "Edge: missing variable",
			makefile: "OTHER = PORT\n",
			want:     []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, contextdocs.ParseEnvKeyList(tc.makefile))
		})
	}
}

func TestDiffEnvKeys(t *testing.T) {
	tests := []struct {
		name string
		base []string
		head []string
		want contextdocs.EnvDelta
	}{
		{
			name: "Happy path: added and removed keys",
			base: []string{"PORT", "JWT_SECRET"},
			head: []string{"PORT", "APP_NAME"},
			want: contextdocs.EnvDelta{Added: []string{"APP_NAME"}, Removed: []string{"JWT_SECRET"}},
		},
		{
			name: "Edge: unchanged keys",
			base: []string{"PORT"},
			head: []string{"PORT"},
			want: contextdocs.EnvDelta{},
		},
		{
			name: "Edge: no base list",
			base: nil,
			head: []string{"PORT"},
			want: contextdocs.EnvDelta{Added: []string{"PORT"}},
		},
		{
			name: "Edge: no head list",
			base: []string{"PORT"},
			head: nil,
			want: contextdocs.EnvDelta{Removed: []string{"PORT"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, contextdocs.DiffEnvKeys(tc.base, tc.head))
		})
	}
}

func TestAlwaysTriggersAreMapped(t *testing.T) {
	root := repoRoot(t)
	pathMap := loadPathMap(t, root)

	tests := []struct {
		name  string
		file  string
		owner string
	}{
		{name: "go.mod", file: "go.mod", owner: "platform"},
		{name: "web dependencies", file: "web/package.json", owner: "platform"},
		{name: "deploy stack", file: "deploy/docker-compose.yml", owner: "admin"},
		{name: "workflows", file: ".github/workflows/ci.yml", owner: "admin"},
		{name: "route registration", file: "backend/internal/app/router.go", owner: "architecture"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, ok := contextdocs.ContextOwner(pathMap, tc.file)
			require.True(t, ok, "%s must be mapped", tc.file)
			assert.Equal(t, tc.owner, owner)
		})
	}
}

func TestMakefileEnvKeysAreMapped(t *testing.T) {
	root := repoRoot(t)
	pathMap := loadPathMap(t, root)
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	require.NoError(t, err)

	keys := contextdocs.ParseEnvKeyList(string(data))
	require.NotEmpty(t, keys)

	mapped := make(map[string]struct{}, len(pathMap.Env))
	for _, entry := range pathMap.Env {
		mapped[entry.Key] = struct{}{}
	}
	for _, key := range keys {
		assert.Contains(t, mapped, key, "every APP_ENV_KEYS entry must be mapped in context/paths.yaml")
	}
}
