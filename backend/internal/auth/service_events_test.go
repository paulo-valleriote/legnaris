package auth_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_Register_EmitsUserRegistered(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		email      string
		createErr  error
		wantEvents int
	}{
		{name: "Happy path: a created account emits one user.registered", email: "Player@Example.com", wantEvents: 1},
		{name: "Error: an invalid address is refused before the account is created", email: "not-an-email", wantEvents: 0},
		{name: "Error: an internal insert failure emits nothing", email: "Player@Example.com", createErr: errors.New("write failed"), wantEvents: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pub := eventsmocks.NewMockPublisher(t)
			var got []events.Event
			pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
				got = append(got, ev)
			}).Maybe()

			service, m := newAuthServiceWithPublisher(t, nil, nil, pub)

			var createdID string
			if tc.wantEvents > 0 || tc.createErr != nil {
				m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Maybe()
			}
			if tc.wantEvents > 0 || tc.createErr != nil {
				m.users.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, u *user.User) error {
					createdID = u.ID
					return tc.createErr
				}).Once()
			}
			if tc.wantEvents > 0 {
				m.expectRegistrationChallenge(testEmail)
			}

			_, err := service.Register(ctx, auth.RegisterCommand{Email: tc.email, Username: "player", Password: testPassword})
			if tc.wantEvents == 0 {
				require.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)

			require.Len(t, got, tc.wantEvents)
			ev := got[0]
			assert.Equal(t, "user.registered", ev.Type)
			require.NotNil(t, ev.ActorID)
			assert.Equal(t, createdID, *ev.ActorID)
			assert.Equal(t, "user", ev.AggregateType)
			assert.Equal(t, createdID, ev.AggregateID)
			require.NoError(t, idgen.Validate(ev.AggregateID))
			assert.Equal(t, int64(0), ev.AggregateVersion)
			assert.Equal(t, map[string]any{}, ev.Data)
		})
	}
}
