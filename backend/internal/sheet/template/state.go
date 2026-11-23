package template

import "strings"

type State string

const (
	StateDraft   State = "draft"
	StateActive  State = "active"
	StateRetired State = "retired"

	developmentEnv = "development"
)

func DraftsAllowed(appEnv string) bool {
	return strings.EqualFold(strings.TrimSpace(appEnv), developmentEnv)
}

func (s State) Valid() bool {
	switch s {
	case StateDraft, StateActive, StateRetired:
		return true
	default:
		return false
	}
}

func (s State) Listed(draftsAllowed bool) bool {
	switch s {
	case StateActive:
		return true
	case StateDraft:
		return draftsAllowed
	default:
		return false
	}
}

func (s State) Available(draftsAllowed bool) bool {
	return s != StateDraft || draftsAllowed
}

func (s State) Creatable(draftsAllowed bool) bool {
	return s == StateActive || (s == StateDraft && draftsAllowed)
}

func (t *Template) Creatable(draftsAllowed bool) bool {
	return t.State.Creatable(draftsAllowed)
}

func (t *Template) Available(draftsAllowed bool) bool {
	return t.State.Available(draftsAllowed)
}
