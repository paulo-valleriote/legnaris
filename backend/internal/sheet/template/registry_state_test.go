package template

import (
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stateFixture(id string, state State) Template {
	return Template{
		ID:    id,
		Name:  id,
		State: state,
		Fields: FieldGroup{
			"stats": {Children: FieldGroup{
				"strength": layoutLeaf(""),
			}},
		},
	}
}

func stateFixtures() []Template {
	return []Template{
		stateFixture("draft_v1", StateDraft),
		stateFixture("active_v1", StateActive),
		stateFixture("retired_v1", StateRetired),
	}
}

func catalogIDs(templates []Template) []string {
	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		ids = append(ids, tmpl.ID)
	}
	return ids
}

func TestRegistry_LifecycleVisibility(t *testing.T) {
	tests := []struct {
		name       string
		appEnv     string
		wantListed []string
	}{
		{
			name:       "Edge: an empty APP_ENV hides drafts and retired templates",
			appEnv:     "",
			wantListed: []string{"active_v1"},
		},
		{
			name:       "Edge: production hides drafts and retired templates",
			appEnv:     "production",
			wantListed: []string{"active_v1"},
		},
		{
			name:       "Edge: staging hides drafts and retired templates",
			appEnv:     "staging",
			wantListed: []string{"active_v1"},
		},
		{
			name:       "Happy path: development lists drafts but not retired templates",
			appEnv:     "Development",
			wantListed: []string{"draft_v1", "active_v1"},
		},
		{
			name:       "Happy path: a padded development value lists drafts",
			appEnv:     " development ",
			wantListed: []string{"draft_v1", "active_v1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := buildRegistry(engine.NewFormulaEngine(), stateFixtures(),
				WithDraftsAllowed(DraftsAllowed(tc.appEnv)))
			require.NoError(t, err)

			assert.Equal(t, tc.wantListed, catalogIDs(registry.List()))
		})
	}
}

func TestRegistry_TemplateStatesByGameSystemIgnoresState(t *testing.T) {
	draft := stateFixture("draft_5e", StateDraft)
	draft.GameSystemID = GameSystemFifth
	active := stateFixture("active_5e", StateActive)
	active.GameSystemID = GameSystemFifth
	retired := stateFixture("retired_5e", StateRetired)
	retired.GameSystemID = GameSystemFifth

	registry, err := buildRegistry(engine.NewFormulaEngine(),
		[]Template{draft, active, retired, stateFixture("standalone_v1", StateActive)},
		WithDraftsAllowed(false),
	)
	require.NoError(t, err)

	t.Run("Happy path: every state of the game system is returned with its state", func(t *testing.T) {
		assert.Equal(t, []TemplateStateRef{
			{ID: "draft_5e", State: StateDraft},
			{ID: "active_5e", State: StateActive},
			{ID: "retired_5e", State: StateRetired},
		}, registry.TemplateStatesByGameSystem(GameSystemFifth))
	})

	t.Run("Edge: an unknown game system returns no templates", func(t *testing.T) {
		assert.Empty(t, registry.TemplateStatesByGameSystem("unknown"))
	})

	t.Run("Edge: an empty game system returns no templates", func(t *testing.T) {
		assert.Empty(t, registry.TemplateStatesByGameSystem(""))
	})
}

func TestRegistry_InternalFetchAndDAGKeepEveryState(t *testing.T) {
	for _, allowDrafts := range []bool{false, true} {
		registry, err := buildRegistry(engine.NewFormulaEngine(), stateFixtures(), WithDraftsAllowed(allowDrafts))
		require.NoError(t, err)

		for _, tmpl := range stateFixtures() {
			fetched, ok := registry.FetchByID(tmpl.ID)
			require.True(t, ok, "internal fetch of %s", tmpl.ID)
			assert.Equal(t, tmpl.State, fetched.State)

			dag, ok := registry.DAG(tmpl.ID)
			require.True(t, ok, "internal DAG of %s", tmpl.ID)
			assert.NotNil(t, dag)
		}
	}
}

func TestBuildRegistry_RejectsInvalidTemplateState(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  string
	}{
		{name: "Error: a missing state fails boot", state: "", want: "has no lifecycle state"},
		{name: "Error: an unknown state fails boot", state: "preview", want: "has unknown lifecycle state 'preview'"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildRegistry(engine.NewFormulaEngine(), []Template{stateFixture("bad_v1", tc.state)})

			require.Error(t, err)
			var invalid *InvalidTemplateStateError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, "bad_v1", invalid.TemplateID)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestFantasyV1_IsActive(t *testing.T) {
	var fantasy Template
	for _, tmpl := range DefaultTemplates() {
		if tmpl.ID == "fantasy_v1" {
			fantasy = tmpl
		}
	}
	require.Equal(t, "fantasy_v1", fantasy.ID)
	assert.Equal(t, StateActive, fantasy.State)
}

func TestTemplate_StatePredicates(t *testing.T) {
	tests := []struct {
		name      string
		state     State
		allow     bool
		listed    bool
		available bool
		creatable bool
	}{
		{name: "Edge: a draft is hidden outside development", state: StateDraft, allow: false},
		{name: "Happy path: a draft is visible in development", state: StateDraft, allow: true, listed: true, available: true, creatable: true},
		{name: "Happy path: an active template is always visible", state: StateActive, allow: false, listed: true, available: true, creatable: true},
		{name: "Edge: a retired template is fetchable but neither listed nor creatable", state: StateRetired, allow: true, available: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := &Template{State: tc.state}
			assert.Equal(t, tc.listed, tmpl.State.Listed(tc.allow))
			assert.Equal(t, tc.available, tmpl.Available(tc.allow))
			assert.Equal(t, tc.creatable, tmpl.Creatable(tc.allow))
		})
	}
}
