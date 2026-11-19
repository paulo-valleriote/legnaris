package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "Happy path: a title with the Jira key passes",
			title:    "LGN-287: require a Jira key in the PR title",
			wantCode: 0,
			wantOut:  "LGN-287",
		},
		{
			name:     "Error: a title without the key fails",
			title:    "require a Jira key in the PR title",
			wantCode: 1,
			wantErr:  "pull request title has no LGN-<number> Jira key",
		},
		{
			name:     "Error: an empty title fails",
			title:    "",
			wantCode: 1,
			wantErr:  "pull request title has no LGN-<number> Jira key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.title, &stdout, &stderr)
			require.Equal(t, tc.wantCode, code)
			assert.Contains(t, stdout.String(), tc.wantOut)
			assert.Contains(t, stderr.String(), tc.wantErr)
		})
	}
}
