package template

import (
	"fmt"
	"regexp"
	"slices"
)

const GameSystemFifth = "fifth"

const fifthEditionBrief = "The game system is fifth edition, as written in SRD 5.2.1."

type GameSystemNotice struct {
	Label string
	Path  string
}

type GameSystemNames struct {
	Classes     []string
	Subclasses  []string
	Species     []string
	Backgrounds []string
	Conditions  []string
}

type GameSystem struct {
	ID                 string
	Name               string
	CompatibilityLabel string
	Notice             GameSystemNotice
	Brief              string
	AllowedNames       GameSystemNames
	Denylist           []string
}

type GameSystems struct {
	systems map[string]GameSystem
}

var gameSystemIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var fifthEditionNames = GameSystemNames{
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
}

var fifthEditionDenylist = []string{
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
}

var defaultGameSystemList = []GameSystem{
	{
		ID:                 GameSystemFifth,
		Name:               "5E",
		CompatibilityLabel: "5E compatible",
		Notice:             GameSystemNotice{Label: "SRD 5.2.1", Path: "/srd"},
		Brief:              fifthEditionBrief,
		AllowedNames:       fifthEditionNames,
		Denylist:           fifthEditionDenylist,
	},
}

var defaultGameSystems = mustBuildDefaultGameSystems()

func DefaultGameSystems() GameSystems {
	return defaultGameSystems
}

func BuildGameSystems(systems []GameSystem) (GameSystems, error) {
	registry := GameSystems{
		systems: make(map[string]GameSystem, len(systems)),
	}

	for _, system := range systems {
		if err := validateGameSystem(system); err != nil {
			return GameSystems{}, err
		}
		if _, exists := registry.systems[system.ID]; exists {
			return GameSystems{}, fmt.Errorf("%w: %q", ErrGameSystemDuplicateID, system.ID)
		}
		registry.systems[system.ID] = system.clone()
	}

	return registry, nil
}

func (g GameSystems) Fetch(id string) (GameSystem, bool) {
	system, ok := g.systems[id]
	if !ok {
		return GameSystem{}, false
	}
	return system.clone(), true
}

func (g GameSystems) Brief(id string) (string, bool) {
	system, ok := g.systems[id]
	if !ok {
		return "", false
	}
	return system.Brief, true
}

func (g GameSystems) AllowedNames(id string) (GameSystemNames, bool) {
	system, ok := g.systems[id]
	if !ok {
		return GameSystemNames{}, false
	}
	return system.AllowedNames.clone(), true
}

func (g GameSystems) Denylist(id string) ([]string, bool) {
	system, ok := g.systems[id]
	if !ok {
		return nil, false
	}
	return slices.Clone(system.Denylist), true
}

func (s GameSystem) clone() GameSystem {
	s.AllowedNames = s.AllowedNames.clone()
	s.Denylist = slices.Clone(s.Denylist)
	return s
}

func (n GameSystemNames) clone() GameSystemNames {
	n.Classes = slices.Clone(n.Classes)
	n.Subclasses = slices.Clone(n.Subclasses)
	n.Species = slices.Clone(n.Species)
	n.Backgrounds = slices.Clone(n.Backgrounds)
	n.Conditions = slices.Clone(n.Conditions)
	return n
}

func validateGameSystem(system GameSystem) error {
	if !gameSystemIDPattern.MatchString(system.ID) {
		return fmt.Errorf("%w: %q", ErrGameSystemIDInvalid, system.ID)
	}
	if system.Name == "" || system.CompatibilityLabel == "" || system.Notice.Label == "" {
		return fmt.Errorf("%w: %q", ErrGameSystemFieldMissing, system.ID)
	}
	if !isSameOriginPath(system.Notice.Path) {
		return fmt.Errorf("%w: %q", ErrGameSystemNoticePathInvalid, system.Notice.Path)
	}
	return nil
}

func isSameOriginPath(path string) bool {
	if path == "" || path[0] != '/' {
		return false
	}
	if len(path) > 1 && path[1] == '/' {
		return false
	}
	for _, r := range path {
		if r == '\\' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func mustBuildDefaultGameSystems() GameSystems {
	systems, err := BuildGameSystems(defaultGameSystemList)
	if err != nil {
		panic(fmt.Sprintf("invalid default game systems: %v", err))
	}
	return systems
}

func validateTemplateGameSystem(tmpl Template) error {
	if tmpl.GameSystemID == "" {
		return nil
	}
	if _, ok := defaultGameSystems.Fetch(tmpl.GameSystemID); !ok {
		return fmt.Errorf("template %q: %w: %q", tmpl.ID, ErrGameSystemUnknown, tmpl.GameSystemID)
	}
	return nil
}

func gameSystemToDTO(id string) *GameSystemDTO {
	if id == "" {
		return nil
	}
	system, ok := defaultGameSystems.Fetch(id)
	if !ok {
		return nil
	}
	return &GameSystemDTO{
		ID:                 system.ID,
		Name:               system.Name,
		CompatibilityLabel: system.CompatibilityLabel,
		Notice: GameSystemNoticeDTO{
			Label: system.Notice.Label,
			Path:  system.Notice.Path,
		},
	}
}
