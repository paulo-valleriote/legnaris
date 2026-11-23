package template_test

import (
	"testing"

	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/assert"
)

func TestDraftsAllowed(t *testing.T) {
	tests := []struct {
		name   string
		appEnv string
		want   bool
	}{
		{name: "Edge: an empty APP_ENV fails closed", appEnv: "", want: false},
		{name: "Edge: production fails closed", appEnv: "production", want: false},
		{name: "Edge: staging fails closed", appEnv: "staging", want: false},
		{name: "Happy path: Development matches case-insensitively", appEnv: "Development", want: true},
		{name: "Happy path: surrounding spaces are trimmed", appEnv: " development ", want: true},
		{name: "Happy path: upper case matches", appEnv: "DEVELOPMENT", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, template.DraftsAllowed(tc.appEnv))
		})
	}
}

func TestState_VisibilityMatrix(t *testing.T) {
	states := []template.State{template.StateDraft, template.StateActive, template.StateRetired}
	envs := []struct {
		appEnv string
		allow  bool
	}{
		{appEnv: "", allow: false},
		{appEnv: "production", allow: false},
		{appEnv: "staging", allow: false},
		{appEnv: "Development", allow: true},
		{appEnv: " development ", allow: true},
	}

	type want struct {
		listed    bool
		available bool
		creatable bool
	}
	expected := map[template.State]func(bool) want{
		template.StateDraft: func(allow bool) want {
			return want{listed: allow, available: allow, creatable: allow}
		},
		template.StateActive: func(bool) want {
			return want{listed: true, available: true, creatable: true}
		},
		template.StateRetired: func(bool) want {
			return want{listed: false, available: true, creatable: false}
		},
	}

	for _, state := range states {
		for _, env := range envs {
			name := string(state) + "/APP_ENV=" + env.appEnv
			t.Run(name, func(t *testing.T) {
				allow := template.DraftsAllowed(env.appEnv)
				assert.Equal(t, env.allow, allow)
				want := expected[state](allow)
				assert.Equal(t, want.listed, state.Listed(allow))
				assert.Equal(t, want.available, state.Available(allow))
				assert.Equal(t, want.creatable, state.Creatable(allow))
			})
		}
	}
}

func TestState_Valid(t *testing.T) {
	tests := []struct {
		name  string
		state template.State
		want  bool
	}{
		{name: "Happy path: draft", state: template.StateDraft, want: true},
		{name: "Happy path: active", state: template.StateActive, want: true},
		{name: "Happy path: retired", state: template.StateRetired, want: true},
		{name: "Edge: an empty state is invalid", state: "", want: false},
		{name: "Edge: an unknown state is invalid", state: "preview", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.state.Valid())
		})
	}
}
