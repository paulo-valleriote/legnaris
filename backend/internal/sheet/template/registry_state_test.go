package template

import (
	"slices"
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
	active := stateFixture("active_v1", StateActive)
	active.Default = true
	return []Template{
		stateFixture("draft_v1", StateDraft),
		active,
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

func defaultStateFixture(id string, state State) Template {
	tmpl := stateFixture(id, state)
	tmpl.Default = true
	return tmpl
}

func TestBuildRegistry_RequiresExactlyOneActiveDefault(t *testing.T) {
	tests := []struct {
		name       string
		templates  []Template
		wantErr    bool
		wantPhrase string
	}{
		{
			name:      "Happy path: one active default builds",
			templates: []Template{defaultStateFixture("default_v1", StateActive), stateFixture("other_v1", StateActive)},
		},
		{
			name:       "Error: no default fails boot",
			templates:  []Template{stateFixture("active_v1", StateActive), stateFixture("other_v1", StateActive)},
			wantErr:    true,
			wantPhrase: "no template is marked",
		},
		{
			name:       "Error: two defaults fail boot",
			templates:  []Template{defaultStateFixture("first_v1", StateActive), defaultStateFixture("second_v1", StateActive)},
			wantErr:    true,
			wantPhrase: "exactly one is required",
		},
		{
			name:       "Error: a draft default fails boot",
			templates:  []Template{defaultStateFixture("draft_v1", StateDraft), stateFixture("active_v1", StateActive)},
			wantErr:    true,
			wantPhrase: "is not active",
		},
		{
			name:       "Error: a retired default fails boot",
			templates:  []Template{defaultStateFixture("retired_v1", StateRetired), stateFixture("active_v1", StateActive)},
			wantErr:    true,
			wantPhrase: "is not active",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := buildRegistry(engine.NewFormulaEngine(), tc.templates)

			if !tc.wantErr {
				require.NoError(t, err)
				assert.NotNil(t, registry)
				return
			}
			require.Error(t, err)
			var invalid *InvalidDefaultTemplateError
			require.ErrorAs(t, err, &invalid)
			assert.Contains(t, err.Error(), tc.wantPhrase)
		})
	}
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
	active.Default = true
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

func TestDefaultCatalog_StatesAndDefault(t *testing.T) {
	templates := DefaultTemplates()
	byID := make(map[string]Template, len(templates))
	for _, tmpl := range templates {
		byID[tmpl.ID] = tmpl
	}

	assert.Equal(t, StateRetired, byID["fantasy_v1"].State)
	assert.False(t, byID["fantasy_v1"].Default)
	assert.Equal(t, StateActive, byID[fifthCharacterID].State)
	assert.True(t, byID[fifthCharacterID].Default)
	assert.Equal(t, StateActive, byID[fifthStatBlockID].State)
	assert.False(t, byID[fifthStatBlockID].Default)
}

func TestDefaultCatalog_ListsOnlyTheFifthEditionTemplates(t *testing.T) {
	for _, draftsAllowed := range []bool{false, true} {
		registry, err := BuildRegistry(engine.NewFormulaEngine(), WithDraftsAllowed(draftsAllowed))
		require.NoError(t, err)

		listed := registry.List()
		assert.Equal(t, []string{fifthCharacterID, fifthStatBlockID}, catalogIDs(listed))

		defaults := make([]string, 0, 1)
		systems := make([]string, 0, len(listed))
		for _, tmpl := range listed {
			if tmpl.Default {
				defaults = append(defaults, tmpl.ID)
			}
			if !slices.Contains(systems, tmpl.GameSystemID) {
				systems = append(systems, tmpl.GameSystemID)
			}
		}
		assert.Equal(t, []string{fifthCharacterID}, defaults)
		assert.Equal(t, []string{GameSystemFifth}, systems)
	}
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
