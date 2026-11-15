package generation_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestFetchSuggestedName covers how GET /generations/{id} derives suggested_name:
// only a succeeded job with a usable naming leaf yields a name, and every other
// case answers null without failing the read.
func TestFetchSuggestedName(t *testing.T) {
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"
	namedTemplate := &template.Template{ID: "fantasy_v1", NameLeaf: "identity.characterName"}
	unnamedTemplate := &template.Template{ID: "fantasy_v1"}

	tests := []struct {
		name     string
		status   generation.Status
		values   map[string]any
		tmpl     *template.Template
		tmplErr  error
		wantName *string
	}{
		{name: "Edge: a queued job has no suggested name", status: generation.StatusQueued, values: nil, tmpl: namedTemplate},
		{name: "Edge: a running job has no suggested name", status: generation.StatusRunning, values: nil, tmpl: namedTemplate},
		{name: "Edge: a failed job has no suggested name", status: generation.StatusFailed, values: nil, tmpl: namedTemplate},
		{name: "Edge: an accepted job has no suggested name", status: generation.StatusAccepted, values: map[string]any{"identity": map[string]any{"characterName": "Aragorn"}}, tmpl: namedTemplate},
		{name: "Edge: a template without a naming leaf yields null", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"characterName": "Aragorn"}}, tmpl: unnamedTemplate},
		{name: "Edge: an absent leaf yields null", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"level": 3}}, tmpl: namedTemplate},
		{name: "Edge: a missing parent group yields null", status: generation.StatusSucceeded, values: map[string]any{"stats": map[string]any{}}, tmpl: namedTemplate},
		{name: "Edge: an empty string yields null", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"characterName": ""}}, tmpl: namedTemplate},
		{name: "Edge: a whitespace-only string yields null", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"characterName": " \t "}}, tmpl: namedTemplate},
		{name: "Edge: a non-string value yields null", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"characterName": 42}}, tmpl: namedTemplate},
		{name: "Edge: a nil value yields null", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"characterName": nil}}, tmpl: namedTemplate},
		{name: "Edge: an unreadable template yields null and the read still succeeds", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"characterName": "Aragorn"}}, tmplErr: errors.New("template gone")},
		{name: "Happy path: a succeeded job suggests the trimmed naming leaf", status: generation.StatusSucceeded, values: map[string]any{"identity": map[string]any{"characterName": "  Aragorn  "}}, tmpl: namedTemplate, wantName: ptr("Aragorn")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			f.store.EXPECT().FindOwned(mock.Anything, userID, jobID).
				Return(generation.Document{ID: jobID, TemplateID: "fantasy_v1", Status: tc.status, Values: tc.values}, nil)
			if tc.status == generation.StatusSucceeded {
				f.templates.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(tc.tmpl, tc.tmplErr)
			}

			view, err := f.svc.Fetch(context.Background(), userID, jobID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantName, view.SuggestedName)
		})
	}
}
