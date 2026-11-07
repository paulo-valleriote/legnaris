package dice_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/dice"
)

func TestParse_BareTokenBoundary(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		class dice.Class
	}{
		{name: "Error: bare d", expr: "1d20 + d", class: dice.ClassUnreadableTerm},
		{name: "Error: bare D", expr: "1d20 + D", class: dice.ClassUnreadableTerm},
		{name: "Error: bare d with a leading minus", expr: "-d", class: dice.ClassUnreadableTerm},
		{name: "Error: bare kh", expr: "1d20 + kh", class: dice.ClassUnreadableTerm},
		{name: "Error: bare kl", expr: "1d20 + kl", class: dice.ClassUnreadableTerm},
		{name: "Error: bare kh1", expr: "1d20 + kh1", class: dice.ClassUnreadableTerm},
		{name: "Error: bare kl2", expr: "1d20 - kl2", class: dice.ClassUnreadableTerm},
		{name: "Error: bare KH with digits", expr: "KH3", class: dice.ClassUnreadableTerm},
		{name: "Error: bare Kl mixed case", expr: "1d20 + Kl", class: dice.ClassUnreadableTerm},
		{name: "Error: bare kh with many digits", expr: "1d20 + kh0001", class: dice.ClassUnreadableTerm},
		{name: "Edge: d followed by a letter is a Field", expr: "1d20 + da", class: ""},
		{name: "Edge: kh followed by a letter is a Field", expr: "1d20 + khz", class: ""},
		{name: "Edge: kl with digits and a trailing letter is a Field", expr: "1d20 + kl2x", class: ""},
		{name: "Edge: kh as the first segment of a path", expr: "kh.d", class: ""},
		{name: "Edge: d as the last segment of a path", expr: "stats.d", class: ""},
		{name: "Edge: kl as a dotted last segment", expr: "stats.kl1", class: ""},
		{name: "Edge: d followed by digits is a die", expr: "d1", class: ""},
		{name: "Edge: d followed by a die count is a die", expr: "d20", class: ""},
		{name: "Edge: keep token inside a dice term is unreadable", expr: "4d6kh3", class: dice.ClassUnreadableTerm},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dice.Parse(tc.expr)
			if tc.class == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			class, ok := dice.ClassOf(err)
			require.True(t, ok)
			assert.Equal(t, tc.class, class)
		})
	}
}
