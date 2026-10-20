package user_test

import (
	"strings"
	"testing"

	"legnaris/backend/internal/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanonicalEmail is the normative rule table: parse, drop the display name,
// trim, lowercase and cap the canonical value, preserving dots and +tags
// (PRQ-08, SC-004).
func TestCanonicalEmail(t *testing.T) {
	valid := []struct {
		name string
		in   string
		want string
	}{
		{name: "Happy path: capitals and surrounding whitespace are removed", in: "  Player@Example.COM  ", want: "player@example.com"},
		{name: "Happy path: a display name is dropped", in: "Name <Player@Example.com>", want: "player@example.com"},
		{name: "Edge: a quoted display name is dropped", in: `"Player One" <player@example.com>`, want: "player@example.com"},
		{name: "Edge: a +tag is preserved", in: "player+news@example.com", want: "player+news@example.com"},
		{name: "Edge: dots in the local part are preserved", in: "p.l.a.y.e.r@example.com", want: "p.l.a.y.e.r@example.com"},
		{name: "Edge: an underscore is preserved", in: "player_one@example.com", want: "player_one@example.com"},
		{name: "Edge: a quoted local part keeps its characters", in: `"player name"@example.com`, want: "player name@example.com"},
		{name: "Edge: a subdomain address is preserved", in: "Player@Mail.Example.COM", want: "player@mail.example.com"},
		{name: "Edge: the 254-byte canonical address is accepted", in: strings.Repeat("a", user.MaxEmailLength-len("@example.com")) + "@example.com", want: strings.Repeat("a", user.MaxEmailLength-len("@example.com")) + "@example.com"},
	}

	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := user.CanonicalEmail(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.LessOrEqual(t, len(got), user.MaxEmailLength)
		})
	}

	invalid := []struct {
		name string
		in   string
	}{
		{name: "Error: empty input", in: ""},
		{name: "Error: whitespace-only input", in: "   "},
		{name: "Error: no at sign", in: "player"},
		{name: "Error: no domain", in: "player@"},
		{name: "Error: no local part", in: "@example.com"},
		{name: "Error: two addresses", in: "one@example.com, two@example.com"},
		{name: "Error: a display name without an address", in: "Player One"},
		{name: "Error: a 255-byte canonical address", in: strings.Repeat("a", user.MaxEmailLength-len("@example.com")+1) + "@example.com"},
		{name: "Error: a display name cannot make an over-long address acceptable", in: "Name <" + strings.Repeat("a", user.MaxEmailLength-len("@example.com")+1) + "@example.com>"},
	}

	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := user.CanonicalEmail(tc.in)
			assert.ErrorIs(t, err, user.ErrInvalidEmail)
			assert.Empty(t, got)
		})
	}
}

// TestCanonicalEmailIsPure proves the rule has no hidden state: the same input
// always yields the same output.
func TestCanonicalEmailIsPure(t *testing.T) {
	const input = "  Player@Example.COM  "

	first, err := user.CanonicalEmail(input)
	require.NoError(t, err)
	second, err := user.CanonicalEmail(input)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}
