package app

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/user"
	usermocks "legnaris/backend/internal/user/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCampaignNameLookup_Username(t *testing.T) {
	repoDown := errors.New("db down")
	tests := []struct {
		name    string
		found   *user.User
		err     error
		want    string
		wantErr error
	}{
		{name: "Happy path: returns the stored username", found: &user.User{Username: "alice"}, want: "alice"},
		{name: "Edge: a deleted user degrades to an empty name", err: user.ErrUserNotFound, want: ""},
		{name: "Error: other repository failures are returned", err: repoDown, wantErr: repoDown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := usermocks.NewMockRepository(t)
			repo.EXPECT().FetchByID(mock.Anything, "user-1").Return(tc.found, tc.err).Once()
			lookup := campaignNameLookup{users: repo}

			got, err := lookup.Username(context.Background(), "user-1")
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
