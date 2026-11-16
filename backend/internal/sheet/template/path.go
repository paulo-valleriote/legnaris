package template

import (
	"legnaris/backend/shared/maputils"
	"strings"
)

type Path []string

func (p Path) String() string {
	return strings.Join(p, ".")
}

func (p Path) EngineKey() string {
	return strings.Join(p, "_")
}

func (p Path) Get(nested map[string]any) (any, bool) {
	return maputils.Get(nested, p)
}

func (p Path) Set(nested map[string]any, value any) error {
	return maputils.Set(nested, p, value)
}
