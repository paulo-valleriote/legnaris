package campaign

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"legnaris/backend/shared/apperr"
)

var (
	ownerMember  = Member{UserID: "owner", Role: RoleGM, Owner: true}
	gmMember     = Member{UserID: "gm", Role: RoleGM}
	playerMember = Member{UserID: "player", Role: RolePlayer}
	otherGM      = Member{UserID: "other-gm", Role: RoleGM}
	otherPlayer  = Member{UserID: "other-player", Role: RolePlayer}
)

func TestAuthorize(t *testing.T) {
	tests := []struct {
		name   string
		act    action
		actor  Member
		target Member
		want   apperr.Code
	}{
		{"Happy path: owner leave is refused as owner cannot leave", actionLeave, ownerMember, ownerMember, apperr.CodeCampaignForbidden},
		{"Happy path: GM may leave", actionLeave, gmMember, gmMember, ""},
		{"Happy path: player may leave", actionLeave, playerMember, playerMember, ""},
		{"Happy path: owner promotes a player", actionPromote, ownerMember, playerMember, ""},
		{"Happy path: owner demotes a GM", actionDemote, ownerMember, gmMember, ""},
		{"Happy path: owner removes a GM", actionRemove, ownerMember, gmMember, ""},
		{"Happy path: owner removes a player", actionRemove, ownerMember, playerMember, ""},
		{"Happy path: GM removes a player", actionRemove, gmMember, playerMember, ""},
		{"Error: GM cannot promote", actionPromote, gmMember, playerMember, apperr.CodeCampaignForbidden},
		{"Error: player cannot promote", actionPromote, playerMember, otherPlayer, apperr.CodeCampaignForbidden},
		{"Error: GM cannot demote", actionDemote, gmMember, otherGM, apperr.CodeCampaignForbidden},
		{"Error: owner cannot promote self", actionPromote, ownerMember, ownerMember, apperr.CodeCampaignForbidden},
		{"Error: owner cannot demote self", actionDemote, ownerMember, ownerMember, apperr.CodeCampaignForbidden},
		{"Error: owner role cannot be changed by another actor", actionDemote, gmMember, ownerMember, apperr.CodeCampaignForbidden},
		{"Error: owner cannot remove self through remove", actionRemove, ownerMember, ownerMember, apperr.CodeCampaignForbidden},
		{"Error: GM cannot remove another GM", actionRemove, gmMember, otherGM, apperr.CodeCampaignForbidden},
		{"Error: GM cannot remove the owner", actionRemove, gmMember, ownerMember, apperr.CodeCampaignForbidden},
		{"Error: GM cannot remove self through remove", actionRemove, gmMember, gmMember, apperr.CodeCampaignForbidden},
		{"Error: player cannot remove a player", actionRemove, playerMember, otherPlayer, apperr.CodeCampaignForbidden},
		{"Error: player cannot remove a GM", actionRemove, playerMember, gmMember, apperr.CodeCampaignForbidden},
		{"Error: nobody removes the owner", actionRemove, ownerMember, ownerMember, apperr.CodeCampaignForbidden},
		{"Edge: unknown action is forbidden", action(0), ownerMember, playerMember, apperr.CodeCampaignForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := authorize(tc.act, tc.actor, tc.target)
			if tc.want == "" {
				assert.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			assert.Equal(t, tc.want, err.Code)
			assert.Equal(t, 403, err.HttpStatus())
		})
	}
}
