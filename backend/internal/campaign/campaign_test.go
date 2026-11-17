package campaign

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Edge: empty name is invalid", "", false},
		{"Edge: single character is valid", "a", true},
		{"Happy path: typical name is valid", "Curse of Strahd", true},
		{"Edge: 80 runes is valid", strings.Repeat("a", 80), true},
		{"Edge: 81 runes is invalid", strings.Repeat("a", 81), false},
		{"Edge: multi-byte runes count as one character", strings.Repeat("á", 80), true},
		{"Edge: 81 multi-byte runes is invalid", strings.Repeat("á", 81), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ValidName(tc.input))
		})
	}
}

func TestValidTemplateID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Edge: empty id is invalid", "", false},
		{"Edge: uppercase is invalid", "Dnd5e", false},
		{"Edge: leading digit is invalid", "5e", false},
		{"Edge: hyphen is invalid", "dnd-5e", false},
		{"Edge: path traversal is invalid", "../dnd", false},
		{"Happy path: lowercase slug with digits and underscore", "dnd_5e2", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ValidTemplateID(tc.input))
		})
	}
}

func TestCampaign_MemberOf(t *testing.T) {
	c := Campaign{Members: []Member{{UserID: "owner", Role: RoleGM, Owner: true}, {UserID: "p1", Role: RolePlayer}}}
	tests := []struct {
		name   string
		userID string
		found  bool
	}{
		{"Edge: empty user id never matches", "", false},
		{"Edge: unknown user is not a member", "stranger", false},
		{"Happy path: player is found", "p1", true},
		{"Happy path: owner is found", "owner", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := c.MemberOf(tc.userID)
			assert.Equal(t, tc.found, ok)
		})
	}
}

func TestCampaign_MemberOfEmptyMembers(t *testing.T) {
	var c Campaign
	_, ok := c.MemberOf("anyone")
	assert.False(t, ok)
}

func TestCampaign_Recount(t *testing.T) {
	tests := []struct {
		name    string
		members []Member
		gms     int
		players int
	}{
		{"Edge: no members yields zero counters", nil, 0, 0},
		{"Happy path: owner plus players", []Member{{UserID: "o", Role: RoleGM}, {UserID: "p", Role: RolePlayer}, {UserID: "q", Role: RolePlayer}}, 1, 2},
		{"Happy path: mixed GMs and players", []Member{{UserID: "a", Role: RoleGM}, {UserID: "b", Role: RoleGM}, {UserID: "c", Role: RolePlayer}}, 2, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Campaign{Members: tc.members, GMCount: 99, PlayerCount: 99}
			c.recount()
			assert.Equal(t, tc.gms, c.GMCount)
			assert.Equal(t, tc.players, c.PlayerCount)
		})
	}
}
