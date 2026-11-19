package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyze(t *testing.T, source string) []violation {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(source), 0o600))
	violations, err := checkTree(dir)
	require.NoError(t, err)
	return violations
}

func summarize(violations []violation) []string {
	if len(violations) == 0 {
		return nil
	}
	lines := make([]string, 0, len(violations))
	for _, v := range violations {
		lines = append(lines, v.message)
	}
	return lines
}

func TestCheckTreeViolations(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name: "Happy path: every case carries an allowed prefix",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "Happy path: a valid input succeeds"},
		{name: "Edge: an empty input is accepted"},
		{name: "Error: an invalid input fails"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: nil,
		},
		{
			name: "Error: an unprefixed slice table is reported",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "a valid input succeeds"},
		{name: "Error: an invalid input fails"},
		{name: "an empty input is accepted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "a valid input succeeds" must start with "Happy path:", "Edge:" or "Error:"`,
				`test case name "an empty input is accepted" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Error: an unprefixed map keyed table is reported",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"first":  {name: "happy"},
		"second": {name: "Edge: an empty value is accepted"},
	}
	for key, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_ = key
		})
	}
}
`,
			want: []string{
				`test case name "happy" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Edge: a fixture struct whose name never reaches t.Run is ignored",
			source: `package sample

import "testing"

type fixture struct {
	name  string
	value int
}

func TestThing(t *testing.T) {
	fixtures := []fixture{
		{name: "plain fixture", value: 1},
		{name: "another fixture", value: 2},
	}
	for _, tc := range fixtures {
		_ = tc.name
	}
}
`,
			want: nil,
		},
		{
			name: "Edge: a field that never reaches t.Run is ignored",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name    string
		display string
	}{
		{name: "unprefixed name", display: "unprefixed display"},
	}
	for _, tc := range tests {
		t.Run(tc.display, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "unprefixed display" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Edge: non-literal names are ignored",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	label := "dynamic"
	tests := []struct {
		name string
	}{
		{name: label},
		{name: "dynamic " + label},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: nil,
		},
		{
			name: "Edge: a nested t.Run over a subtable is ignored",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name  string
		steps []string
	}{
		{name: "Edge: a table with steps", steps: []string{"first", "second"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, step := range tc.steps {
				t.Run(step, func(t *testing.T) {})
			}
		})
	}
}
`,
			want: nil,
		},
		{
			name: "Edge: a range over an inline table literal is checked",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	for _, tc := range []struct {
		name string
	}{
		{name: "inline case"},
	} {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "inline case" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Edge: a package level table is checked",
			source: `package sample

import "testing"

var sharedCases = []struct {
	name string
}{
	{name: "shared case"},
}

func TestThing(t *testing.T) {
	for _, tc := range sharedCases {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "shared case" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Edge: a table returned by a helper is checked",
			source: `package sample

import "testing"

type testCase struct {
	name string
}

func cases(t *testing.T) []testCase {
	t.Helper()
	return []testCase{
		{name: "helper case"},
	}
}

func TestThing(t *testing.T) {
	for _, tc := range cases(t) {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "helper case" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Edge: an appended table is checked",
			source: `package sample

import "testing"

type testCase struct {
	name string
}

func TestThing(t *testing.T) {
	tests := make([]testCase, 0, 1)
	tests = append(tests, testCase{name: "appended case"})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "appended case" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Edge: a table that is never ranged is ignored",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	unused := []struct {
		name string
	}{
		{name: "never used"},
	}
	_ = unused
}
`,
			want: nil,
		},
		{
			name: "Edge: a t.Run over a variable that is not the range value is ignored",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	other := struct{ name string }{name: "other"}
	tests := []struct {
		name string
	}{
		{name: "Edge: prefixed", },
	}
	for _, tc := range tests {
		_ = tc
		t.Run(other.name, func(t *testing.T) {})
	}
}
`,
			want: nil,
		},
		{
			name: "Edge: an unkeyed struct literal is ignored",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name  string
		value int
	}{
		{"positional name", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: nil,
		},
		{
			name: "Edge: a map of pointer values is checked",
			source: `package sample

import "testing"

type testCase struct {
	name string
}

func TestThing(t *testing.T) {
	tests := map[string]*testCase{
		"first": {name: "pointer case"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "pointer case" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
		{
			name: "Edge: a shadowing local table is checked instead of the package table",
			source: `package sample

import "testing"

var sharedCases = []struct {
	name string
}{
	{name: "Happy path: a package case"},
}

func TestThing(t *testing.T) {
	sharedCases := []struct {
		name string
	}{
		{name: "local case"},
	}
	for _, tc := range sharedCases {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			want: []string{
				`test case name "local case" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, summarize(analyze(t, tc.source)))
		})
	}
}

func TestCheckTreePositions(t *testing.T) {
	source := `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "first case"},
		{name: "second case"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`
	violations := analyze(t, source)
	require.Len(t, violations, 2)
	assert.Equal(t, 9, violations[0].position.Line)
	assert.Equal(t, 10, violations[1].position.Line)
	assert.True(t, strings.HasSuffix(violations[0].position.Filename, "sample_test.go"))
}

func TestCheckTreeSkipsNonTestFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample.go"), []byte(`package sample

func Thing() {}
`), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "testdata"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "testdata", "fixture_test.go"), []byte(`package fixture

import "testing"

func TestFixture(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "fixture case"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`), 0o600))

	violations, err := checkTree(dir)
	require.NoError(t, err)
	assert.Empty(t, violations)
}

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantCode int
		wantOut  []string
	}{
		{
			name: "Happy path: a clean tree exits zero",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "Happy path: a valid input succeeds"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			wantCode: 0,
			wantOut:  nil,
		},
		{
			name: "Error: a violation exits one and is printed as file:line: message",
			source: `package sample

import "testing"

func TestThing(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "unprefixed case"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {})
	}
}
`,
			wantCode: 1,
			wantOut: []string{
				`sample_test.go:9: test case name "unprefixed case" must start with "Happy path:", "Edge:" or "Error:"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(tc.source), 0o600))
			var out bytes.Buffer
			code, err := run(dir, &out)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCode, code)
			if tc.wantOut == nil {
				assert.Empty(t, out.String())
				return
			}
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			require.Len(t, lines, len(tc.wantOut))
			for i, line := range lines {
				assert.True(t, strings.HasSuffix(line, tc.wantOut[i]), "line %d: %q", i, line)
			}
		})
	}
}

func TestRunReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken_test.go"), []byte("package sample\n\nfunc Broken( {\n"), 0o600))
	var out bytes.Buffer
	code, err := run(dir, &out)
	assert.Equal(t, 2, code)
	require.Error(t, err)
	assert.Empty(t, out.String())
}
