package prtitle_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"legnaris/scripts/ci/prtitle"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{name: "Happy path: the key leads the title", title: "LGN-287: require a Jira key in the PR title"},
		{name: "Happy path: the key trails the title", title: "Require a Jira key in the PR title (LGN-287)"},
		{name: "Happy path: the key sits mid-title", title: "ci(LGN-287): require a Jira key"},
		{name: "Edge: a key with extra trailing characters still matches", title: "LGN-287abc"},
		{name: "Error: a lowercase key is rejected", title: "lgn-287: require a Jira key", wantErr: true},
		{name: "Error: a key without a number is rejected", title: "LGN-: require a Jira key", wantErr: true},
		{name: "Error: a key with a non-numeric body is rejected", title: "LGN-abc: require a Jira key", wantErr: true},
		{name: "Error: another project key is rejected", title: "ABC-287: require a Jira key", wantErr: true},
		{name: "Error: an empty title is rejected", title: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := prtitle.Validate(tc.title)
			if tc.wantErr {
				assert.ErrorIs(t, err, prtitle.ErrMissingKey)
				return
			}
			assert.NoError(t, err)
		})
	}
}
