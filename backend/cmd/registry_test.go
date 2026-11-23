package main

import (
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTemplateRegistry(t *testing.T) {
	tests := []struct {
		name       string
		appEnv     string
		wantDrafts bool
	}{
		{name: "Edge: an empty APP_ENV hides Drafts", appEnv: ""},
		{name: "Edge: production hides Drafts", appEnv: "production"},
		{name: "Edge: an unknown environment hides Drafts", appEnv: "staging"},
		{name: "Edge: a padded mixed-case development allows Drafts", appEnv: "  Development ", wantDrafts: true},
		{name: "Happy path: development allows Drafts", appEnv: "development", wantDrafts: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := buildTemplateRegistry(engine.NewFormulaEngine(), tc.appEnv)

			require.NoError(t, err)
			assert.Equal(t, tc.wantDrafts, registry.DraftsAllowed())
			_, ok := registry.FetchByID("fantasy_v1")
			assert.True(t, ok)
		})
	}
}
