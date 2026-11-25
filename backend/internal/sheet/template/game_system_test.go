package template

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gameSystemFixture(id string) GameSystem {
	return GameSystem{
		ID:                 id,
		Name:               "5E",
		CompatibilityLabel: "5E compatible",
		Notice:             GameSystemNotice{Label: "SRD 5.2.1", Path: "/srd"},
	}
}

func gameSystemWith(overrides func(*GameSystem)) GameSystem {
	system := gameSystemFixture(GameSystemFifth)
	overrides(&system)
	return system
}

func TestBuildGameSystems(t *testing.T) {
	tests := []struct {
		name    string
		systems []GameSystem
		wantErr error
	}{
		{
			name:    "Happy path: the fifth edition descriptor is registered by its id",
			systems: []GameSystem{gameSystemFixture(GameSystemFifth)},
		},
		{
			name:    "Edge: an empty catalogue builds an empty registry",
			systems: nil,
		},
		{
			name:    "Edge: a root-relative notice path is accepted",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) { system.Notice.Path = "/" })},
		},
		{
			name: "Edge: a notice path with a query and a fragment is accepted",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = "/srd/notice?lang=en#top"
			})},
		},
		{
			name:    "Error: an empty id is malformed",
			systems: []GameSystem{gameSystemFixture("")},
			wantErr: ErrGameSystemIDInvalid,
		},
		{
			name:    "Error: an upper-case id is malformed",
			systems: []GameSystem{gameSystemFixture("Fifth")},
			wantErr: ErrGameSystemIDInvalid,
		},
		{
			name:    "Error: an id starting with a digit is malformed",
			systems: []GameSystem{gameSystemFixture("5th")},
			wantErr: ErrGameSystemIDInvalid,
		},
		{
			name:    "Error: an id with a hyphen is malformed",
			systems: []GameSystem{gameSystemFixture("fifth-edition")},
			wantErr: ErrGameSystemIDInvalid,
		},
		{
			name:    "Error: an id with a space is malformed",
			systems: []GameSystem{gameSystemFixture("fifth edition")},
			wantErr: ErrGameSystemIDInvalid,
		},
		{
			name:    "Error: a duplicate id is refused",
			systems: []GameSystem{gameSystemFixture(GameSystemFifth), gameSystemFixture(GameSystemFifth)},
			wantErr: ErrGameSystemDuplicateID,
		},
		{
			name:    "Error: an empty name is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) { system.Name = "" })},
			wantErr: ErrGameSystemFieldMissing,
		},
		{
			name: "Error: an empty compatibility label is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.CompatibilityLabel = ""
			})},
			wantErr: ErrGameSystemFieldMissing,
		},
		{
			name: "Error: an empty notice label is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Label = ""
			})},
			wantErr: ErrGameSystemFieldMissing,
		},
		{
			name:    "Error: an empty notice path is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) { system.Notice.Path = "" })},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
		{
			name: "Error: a relative notice path is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = "srd"
			})},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
		{
			name: "Error: a protocol-relative notice path is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = "//example.com/srd"
			})},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
		{
			name: "Error: an absolute https notice path is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = "https://example.com/srd"
			})},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
		{
			name: "Error: an opaque protocol notice path is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = "http:"
			})},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
		{
			name: "Error: a javascript notice path is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = "javascript:alert(1)"
			})},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
		{
			name: "Error: a backslash notice path is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = `/\example.com/srd`
			})},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
		{
			name: "Error: a notice path with a control character is refused",
			systems: []GameSystem{gameSystemWith(func(system *GameSystem) {
				system.Notice.Path = "/srd\nx"
			})},
			wantErr: ErrGameSystemNoticePathInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := BuildGameSystems(tc.systems)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Len(t, registry.systems, len(tc.systems))
		})
	}
}

func TestGameSystems_Fetch(t *testing.T) {
	registry, err := BuildGameSystems([]GameSystem{gameSystemFixture(GameSystemFifth)})
	require.NoError(t, err)

	fetched, ok := registry.Fetch(GameSystemFifth)
	require.True(t, ok)
	assert.Equal(t, "fifth", fetched.ID)
	assert.Equal(t, "5E", fetched.Name)
	assert.Equal(t, "5E compatible", fetched.CompatibilityLabel)
	assert.Equal(t, GameSystemNotice{Label: "SRD 5.2.1", Path: "/srd"}, fetched.Notice)

	_, ok = registry.Fetch("sixth")
	assert.False(t, ok)

	_, ok = registry.Fetch("")
	assert.False(t, ok)
}

func TestDefaultGameSystems_PinnedEntry(t *testing.T) {
	system, ok := DefaultGameSystems().Fetch(GameSystemFifth)
	require.True(t, ok)

	assert.Equal(t, GameSystemFifth, system.ID)
	assert.Equal(t, "5E", system.Name)
	assert.Equal(t, "5E compatible", system.CompatibilityLabel)
	assert.Equal(t, GameSystemNotice{Label: "SRD 5.2.1", Path: "/srd"}, system.Notice)
	assert.Equal(t, "The game system is fifth edition, as written in SRD 5.2.1.", system.Brief)
}

func TestDefaultGameSystems_FifthEditionNames(t *testing.T) {
	system, ok := DefaultGameSystems().Fetch(GameSystemFifth)
	require.True(t, ok)

	assert.Equal(t, GameSystemNames{
		Classes: []string{
			"Barbarian", "Bard", "Cleric", "Druid", "Fighter", "Monk",
			"Paladin", "Ranger", "Rogue", "Sorcerer", "Warlock", "Wizard",
		},
		Subclasses: []string{
			"Path of the Berserker", "College of Lore", "Life Domain", "Circle of the Land",
			"Champion", "Warrior of the Open Hand", "Oath of Devotion", "Hunter", "Thief",
			"Draconic Sorcery", "Fiend Patron", "Evoker",
		},
		Species: []string{
			"Dragonborn", "Dwarf", "Elf", "Gnome", "Goliath", "Halfling", "Human", "Orc", "Tiefling",
		},
		Backgrounds: []string{"Acolyte", "Criminal", "Sage", "Soldier"},
		Conditions: []string{
			"Blinded", "Charmed", "Deafened", "Exhaustion", "Frightened", "Grappled",
			"Incapacitated", "Invisible", "Paralyzed", "Petrified", "Poisoned", "Prone",
			"Restrained", "Stunned", "Unconscious",
		},
	}, system.AllowedNames)
}

func TestDefaultGameSystems_FifthEditionDenylist(t *testing.T) {
	system, ok := DefaultGameSystems().Fetch(GameSystemFifth)
	require.True(t, ok)

	assert.Equal(t, []string{
		"Dungeons & Dragons",
		"D&D",
		"D&D Beyond",
		"Wizards of the Coast",
		"Player's Handbook",
		"official",
		"licensed by",
		"powered by",
		"Forgotten Realms",
		"Strahd",
		"Orcus",
		"Tiamat",
		"beholder",
		"mind flayer",
		"race",
	}, system.Denylist)
}

func TestGameSystems_DataAccessors(t *testing.T) {
	registry, err := BuildGameSystems([]GameSystem{gameSystemWith(func(system *GameSystem) {
		system.Brief = "Custom brief."
		system.AllowedNames = GameSystemNames{
			Classes:     []string{"Fighter"},
			Subclasses:  []string{"Champion"},
			Species:     []string{"Human"},
			Backgrounds: []string{"Soldier"},
			Conditions:  []string{"Prone"},
		}
		system.Denylist = []string{"beholder"}
	})})
	require.NoError(t, err)

	brief, ok := registry.Brief(GameSystemFifth)
	require.True(t, ok)
	assert.Equal(t, "Custom brief.", brief)

	names, ok := registry.AllowedNames(GameSystemFifth)
	require.True(t, ok)
	assert.Equal(t, []string{"Fighter"}, names.Classes)
	assert.Equal(t, []string{"Champion"}, names.Subclasses)
	assert.Equal(t, []string{"Human"}, names.Species)
	assert.Equal(t, []string{"Soldier"}, names.Backgrounds)
	assert.Equal(t, []string{"Prone"}, names.Conditions)

	denylist, ok := registry.Denylist(GameSystemFifth)
	require.True(t, ok)
	assert.Equal(t, []string{"beholder"}, denylist)

	_, ok = registry.Brief("sixth")
	assert.False(t, ok)
	_, ok = registry.AllowedNames("sixth")
	assert.False(t, ok)
	_, ok = registry.Denylist("sixth")
	assert.False(t, ok)
}

func TestGameSystems_DataAccessorsReturnCopies(t *testing.T) {
	registry, err := BuildGameSystems([]GameSystem{gameSystemWith(func(system *GameSystem) {
		system.AllowedNames = GameSystemNames{Classes: []string{"Fighter"}}
		system.Denylist = []string{"beholder"}
	})})
	require.NoError(t, err)

	names, ok := registry.AllowedNames(GameSystemFifth)
	require.True(t, ok)
	names.Classes[0] = "Mutated"

	denylist, ok := registry.Denylist(GameSystemFifth)
	require.True(t, ok)
	denylist[0] = "mutated"

	fetched, ok := registry.Fetch(GameSystemFifth)
	require.True(t, ok)
	assert.Equal(t, []string{"Fighter"}, fetched.AllowedNames.Classes)
	assert.Equal(t, []string{"beholder"}, fetched.Denylist)
}

func TestGameSystems_DataEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		system GameSystem
	}{
		{
			name: "Edge: a game system without a brief or names builds with empty data",
			system: GameSystem{
				ID:                 "custom",
				Name:               "Custom",
				CompatibilityLabel: "Custom compatible",
				Notice:             GameSystemNotice{Label: "Custom", Path: "/custom"},
			},
		},
		{
			name: "Edge: a game system with an empty denylist builds",
			system: GameSystem{
				ID:                 "custom",
				Name:               "Custom",
				CompatibilityLabel: "Custom compatible",
				Notice:             GameSystemNotice{Label: "Custom", Path: "/custom"},
				Brief:              "Custom brief.",
				Denylist:           []string{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := BuildGameSystems([]GameSystem{tc.system})
			require.NoError(t, err)

			fetched, ok := registry.Fetch(tc.system.ID)
			require.True(t, ok)
			assert.Equal(t, tc.system.Brief, fetched.Brief)
			assert.Equal(t, tc.system.AllowedNames, fetched.AllowedNames)
			assert.Empty(t, fetched.Denylist)
		})
	}
}

func TestBuildRegistry_GameSystem(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    Template
		wantErr error
	}{
		{
			name: "Happy path: a template resolves a registered game system",
			tmpl: Template{
				ID:           "fifth_character_v1",
				State:        StateActive,
				Default:      true,
				GameSystemID: GameSystemFifth,
				Fields:       FieldGroup{},
			},
		},
		{
			name: "Edge: a template without a game system builds",
			tmpl: Template{ID: "fantasy_v1", State: StateActive, Default: true, Fields: FieldGroup{}},
		},
		{
			name:    "Error: an unknown game system id fails boot",
			tmpl:    Template{ID: "unknown_v1", State: StateActive, GameSystemID: "sixth", Fields: FieldGroup{}},
			wantErr: ErrGameSystemUnknown,
		},
		{
			name:    "Error: a malformed game system id fails boot",
			tmpl:    Template{ID: "malformed_v1", State: StateActive, GameSystemID: "Fifth", Fields: FieldGroup{}},
			wantErr: ErrGameSystemUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildRegistry(engine.NewFormulaEngine(), []Template{tc.tmpl})

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestTemplateToDTO_GameSystem(t *testing.T) {
	tests := []struct {
		name         string
		gameSystemID string
		wantKey      bool
		want         string
	}{
		{
			name:         "Happy path: a template with a game system carries its descriptor",
			gameSystemID: GameSystemFifth,
			wantKey:      true,
			want: `{"id":"fifth","name":"5E","compatibility_label":"5E compatible",` +
				`"notice":{"label":"SRD 5.2.1","path":"/srd"}}`,
		},
		{
			name:    "Edge: a template without a game system omits the key",
			wantKey: false,
		},
		{
			name:         "Edge: an unresolvable id is omitted rather than emitted as null",
			gameSystemID: "sixth",
			wantKey:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := Template{ID: "test", GameSystemID: tc.gameSystemID}
			raw, err := json.Marshal(tmpl.ToDTO())
			require.NoError(t, err)

			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &payload))

			if !tc.wantKey {
				assert.NotContains(t, payload, "game_system")
				assert.Nil(t, tmpl.ToDTO().GameSystem)
				return
			}

			require.Contains(t, payload, "game_system")
			assert.JSONEq(t, tc.want, string(payload["game_system"]))
			require.NotNil(t, tmpl.ToDTO().GameSystem)
			assert.Equal(t, GameSystemFifth, tmpl.ToDTO().GameSystem.ID)
		})
	}
}

func TestFantasyV1_HasNoGameSystem(t *testing.T) {
	assert.Empty(t, fantasyV1Template.GameSystemID)

	raw, err := json.Marshal(fantasyV1Template.ToDTO())
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.NotContains(t, payload, "game_system",
		"fantasy_v1 must keep its existing payload")

	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	fetched, ok := registry.FetchByID(fantasyV1Template.ID)
	require.True(t, ok)
	assert.Empty(t, fetched.GameSystemID)
}

func TestGameSystemErrors_AreDistinct(t *testing.T) {
	sentinels := []error{
		ErrGameSystemIDInvalid,
		ErrGameSystemDuplicateID,
		ErrGameSystemFieldMissing,
		ErrGameSystemNoticePathInvalid,
		ErrGameSystemUnknown,
	}

	for i, first := range sentinels {
		for j, second := range sentinels {
			if i == j {
				continue
			}
			assert.NotErrorIs(t, first, second, "%v must not match %v", first, second)
		}
	}
}
