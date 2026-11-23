package template

import (
	"fmt"
	"regexp"
)

const GameSystemFifth = "fifth"

type GameSystemNotice struct {
	Label string `bson:"label"`
	Path  string `bson:"path"`
}

type GameSystem struct {
	ID                 string           `bson:"id"`
	Name               string           `bson:"name"`
	CompatibilityLabel string           `bson:"compatibility_label"`
	Notice             GameSystemNotice `bson:"notice"`
}

type GameSystems struct {
	systems map[string]GameSystem
	order   []string
}

var gameSystemIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var defaultGameSystemList = []GameSystem{
	{
		ID:                 GameSystemFifth,
		Name:               "5E",
		CompatibilityLabel: "5E compatible",
		Notice:             GameSystemNotice{Label: "SRD 5.2.1", Path: "/srd"},
	},
}

var defaultGameSystems = mustBuildDefaultGameSystems()

func DefaultGameSystems() GameSystems {
	return defaultGameSystems
}

func BuildGameSystems(systems []GameSystem) (GameSystems, error) {
	registry := GameSystems{
		systems: make(map[string]GameSystem, len(systems)),
		order:   make([]string, 0, len(systems)),
	}

	for _, system := range systems {
		if err := validateGameSystem(system); err != nil {
			return GameSystems{}, err
		}
		if _, exists := registry.systems[system.ID]; exists {
			return GameSystems{}, fmt.Errorf("%w: %q", ErrGameSystemDuplicateID, system.ID)
		}
		registry.systems[system.ID] = system
		registry.order = append(registry.order, system.ID)
	}

	return registry, nil
}

func (g GameSystems) List() []GameSystem {
	systems := make([]GameSystem, 0, len(g.order))
	for _, id := range g.order {
		systems = append(systems, g.systems[id])
	}
	return systems
}

func (g GameSystems) Fetch(id string) (GameSystem, bool) {
	system, ok := g.systems[id]
	return system, ok
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
