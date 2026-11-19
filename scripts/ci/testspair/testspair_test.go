package testspair_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/scripts/ci/testspair"
)

func TestViolations(t *testing.T) {
	tests := []struct {
		name      string
		changed   []string
		generated func(string) bool
		want      []string
	}{
		{
			name:    "Happy path: a Go package that ships a test change passes",
			changed: []string{"backend/internal/sheet/service.go", "backend/internal/sheet/service_test.go"},
			want:    nil,
		},
		{
			name:    "Happy path: a web source that ships a test change passes",
			changed: []string{"web/src/sheet/grid.tsx", "web/src/test/sheet_grid.test.tsx"},
			want:    nil,
		},
		{
			name:    "Happy path: several packages each with their own test change pass",
			changed: []string{"backend/a/a.go", "backend/a/a_test.go", "backend/b/b.go", "backend/b/b_test.go"},
			want:    nil,
		},
		{
			name:    "Edge: an empty diff needs nothing",
			changed: nil,
			want:    nil,
		},
		{
			name:    "Edge: test-only changes need no further test",
			changed: []string{"backend/internal/user/repository_test.go"},
			want:    nil,
		},
		{
			name:    "Edge: docs, CI config and testdata are ignored",
			changed: []string{"AGENTS.md", "docs/architecture.md", ".github/workflows/ci.yml", "backend/internal/sheet/testdata/case.json"},
			want:    nil,
		},
		{
			name:    "Edge: mocks and generated files are ignored",
			changed: []string{"backend/internal/sheet/mocks/mock_Service.go", "backend/internal/sheet/catalog_gen.go"},
			want:    nil,
		},
		{
			name:    "Edge: a generated file recognised by content is ignored",
			changed: []string{"backend/internal/sheet/generated_helpers.go", "backend/internal/sheet/service_test.go"},
			generated: func(p string) bool {
				return p == "backend/internal/sheet/generated_helpers.go"
			},
			want: nil,
		},
		{
			name:    "Edge: web tests outside web/src/test count as test files, not sources",
			changed: []string{"web/src/pwa/sw.test.ts"},
			want:    nil,
		},
		{
			name:    "Edge: type declarations under web/src are not sources",
			changed: []string{"web/src/vite-env.d.ts"},
			want:    nil,
		},
		{
			name:    "Edge: a root-level Go file needs a root-level test file",
			changed: []string{"main.go", "main_test.go"},
			want:    nil,
		},
		{
			name:    "Edge: path separators are normalised",
			changed: []string{"./backend/a/a.go", "./backend/a/a_test.go"},
			want:    nil,
		},
		{
			name:    "Error: a Go package without a test change is reported",
			changed: []string{"backend/internal/sheet/service.go"},
			want:    []string{"go backend/internal/sheet changed without a *_test.go change"},
		},
		{
			name:    "Error: a test in another package does not count",
			changed: []string{"backend/internal/sheet/service.go", "backend/internal/sheet/template/template_test.go"},
			want: []string{
				"go backend/internal/sheet changed without a *_test.go change",
			},
		},
		{
			name:    "Error: a web source without a web/src/test change is reported",
			changed: []string{"web/src/sheet/grid.tsx"},
			want:    []string{"web/src/ changed without a change under web/src/test/"},
		},
		{
			name:    "Error: a Go test change does not cover a web source",
			changed: []string{"web/src/sheet/grid.tsx", "backend/internal/sheet/service_test.go"},
			want:    []string{"web/src/ changed without a change under web/src/test/"},
		},
		{
			name:    "Error: a web test change outside web/src/test does not cover a source",
			changed: []string{"web/src/sheet/grid.tsx", "web/src/sheet/grid.test.tsx"},
			want:    []string{"web/src/ changed without a change under web/src/test/"},
		},
		{
			name:    "Error: both areas are reported in a stable order",
			changed: []string{"web/src/sheet/grid.tsx", "backend/internal/sheet/service.go"},
			want: []string{
				"go backend/internal/sheet changed without a *_test.go change",
				"web/src/ changed without a change under web/src/test/",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, violation := range testspair.Violations(tc.changed, tc.generated) {
				got = append(got, violation.String())
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHasLabel(t *testing.T) {
	tests := []struct {
		name       string
		labelsJSON string
		label      string
		want       bool
	}{
		{
			name:       "Happy path: the exception label is present",
			labelsJSON: `["bug","no-tests:refactor"]`,
			label:      testspair.ExceptionLabel,
			want:       true,
		},
		{
			name:       "Edge: an empty label list has no exception",
			labelsJSON: `[]`,
			label:      testspair.ExceptionLabel,
			want:       false,
		},
		{
			name:       "Edge: an empty payload has no exception",
			labelsJSON: "",
			label:      testspair.ExceptionLabel,
			want:       false,
		},
		{
			name:       "Edge: malformed JSON is not an exception",
			labelsJSON: `["no-tests:refactor"`,
			label:      testspair.ExceptionLabel,
			want:       false,
		},
		{
			name:       "Edge: a different case is not the exception",
			labelsJSON: `["No-Tests:Refactor"]`,
			label:      testspair.ExceptionLabel,
			want:       false,
		},
		{
			name:       "Edge: an empty label never matches",
			labelsJSON: `[""]`,
			label:      "",
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, testspair.HasLabel(tc.labelsJSON, tc.label))
		})
	}
}

func TestViolationsNilPredicate(t *testing.T) {
	t.Run("Edge: a nil generated predicate still classifies by path", func(t *testing.T) {
		violations := testspair.Violations([]string{"backend/a/a.go", "backend/a/mocks/mock_A.go"}, nil)
		require.Len(t, violations, 1)
		assert.Equal(t, "go backend/a changed without a *_test.go change", violations[0].String())
	})
}
