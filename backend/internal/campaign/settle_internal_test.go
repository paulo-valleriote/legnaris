package campaign

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"legnaris/backend/shared/apperr"
)

func TestTransferOwnership(t *testing.T) {
	tests := []struct {
		name      string
		members   []Member
		from      string
		to        string
		wantErr   bool
		wantOwner string
	}{
		{
			name: "Happy path: the source loses ownership and the target gains it",
			members: []Member{
				{UserID: "owner", Role: RoleGM, Owner: true},
				{UserID: "successor", Role: RoleGM},
			},
			from: "owner", to: "successor", wantOwner: "successor",
		},
		{
			name: "Edge: the target listed before the source still receives ownership",
			members: []Member{
				{UserID: "successor", Role: RoleGM},
				{UserID: "owner", Role: RoleGM, Owner: true},
			},
			from: "owner", to: "successor", wantOwner: "successor",
		},
		{
			name: "Error: a missing source member is refused",
			members: []Member{
				{UserID: "successor", Role: RoleGM},
			},
			from: "owner", to: "successor", wantErr: true,
		},
		{
			name: "Error: a missing target member is refused",
			members: []Member{
				{UserID: "owner", Role: RoleGM, Owner: true},
			},
			from: "owner", to: "successor", wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Campaign{Members: append([]Member(nil), tc.members...)}
			err := transferOwnership(c, tc.from, tc.to)
			if tc.wantErr {
				require.Error(t, err)
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperr.CodeInternal, appErr.Code)
				assert.Equal(t, MsgHandOffIncomplete, appErr.Message)
				return
			}
			require.NoError(t, err)
			var owners []string
			for _, m := range c.Members {
				if m.Owner {
					owners = append(owners, m.UserID)
				}
			}
			assert.Equal(t, []string{tc.wantOwner}, owners)
		})
	}
}
