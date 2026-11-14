package llmconsent_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/llmconsent/mocks"
	"legnaris/backend/internal/user"
	usermocks "legnaris/backend/internal/user/mocks"
	"legnaris/backend/shared/requestcache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAccountConsentStore_Fetch(t *testing.T) {
	at := fixedNow.Add(-time.Hour)

	tests := []struct {
		name     string
		account  *user.User
		fetchErr error
		want     llmconsent.Record
		wantErr  error
	}{
		{name: "Edge: an account without consent reads as an empty record", account: &user.User{ID: "user-1"}, want: llmconsent.Record{}},
		{name: "Happy path: the consent stored on the account is returned", account: &user.User{ID: "user-1", LLMConsentAt: &at, LLMConsentVersion: llmconsent.Version}, want: llmconsent.Record{At: &at, Version: llmconsent.Version}},
		{name: "Error: a missing account is ErrUserNotFound", fetchErr: user.ErrUserNotFound, wantErr: llmconsent.ErrUserNotFound},
		{name: "Error: a read failure is returned", fetchErr: errStore, wantErr: errStore},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			accounts := mocks.NewMockAccountReader(t)
			accounts.EXPECT().FetchByID(mock.Anything, "user-1").Return(tc.account, tc.fetchErr).Once()
			store := llmconsent.NewAccountConsentStore(accounts, mocks.NewMockConsentStore(t))

			got, err := store.Fetch(context.Background(), "user-1")

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAccountConsentStore_Writes(t *testing.T) {
	tests := []struct {
		name     string
		writeErr error
		wantErr  error
	}{
		{name: "Happy path: a successful write is passed through", writeErr: nil},
		{name: "Error: a write failure is returned", writeErr: errStore, wantErr: errStore},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writes := mocks.NewMockConsentStore(t)
			writes.EXPECT().Grant(mock.Anything, "user-1", llmconsent.Version, fixedNow).Return(tc.writeErr).Once()
			writes.EXPECT().Clear(mock.Anything, "user-1").Return(tc.writeErr).Once()
			store := llmconsent.NewAccountConsentStore(mocks.NewMockAccountReader(t), writes)

			grantErr := store.Grant(context.Background(), "user-1", llmconsent.Version, fixedNow)
			clearErr := store.Clear(context.Background(), "user-1")

			if tc.wantErr != nil {
				require.ErrorIs(t, grantErr, tc.wantErr)
				require.ErrorIs(t, clearErr, tc.wantErr)
				return
			}
			require.NoError(t, grantErr)
			require.NoError(t, clearErr)
		})
	}
}

// TestAccountConsentStore_ReadsAfterWriteSeeTheChange pins that a grant or a revoke
// forgets the account memo, so a read later in the same request sees the change.
func TestAccountConsentStore_ReadsAfterWriteSeeTheChange(t *testing.T) {
	at := fixedNow.Add(-time.Hour)

	tests := []struct {
		name      string
		write     func(store llmconsent.ConsentStore, ctx context.Context) error
		wantFirst llmconsent.Record
		wantAfter llmconsent.Record
	}{
		{
			name: "Happy path: a grant is visible to the next read of the request",
			write: func(store llmconsent.ConsentStore, ctx context.Context) error {
				return store.Grant(ctx, "user-1", llmconsent.Version, at)
			},
			wantFirst: llmconsent.Record{},
			wantAfter: llmconsent.Record{At: &at, Version: llmconsent.Version},
		},
		{
			name: "Happy path: a revoke is visible to the next read of the request",
			write: func(store llmconsent.ConsentStore, ctx context.Context) error {
				return store.Clear(ctx, "user-1")
			},
			wantFirst: llmconsent.Record{At: &at, Version: llmconsent.Version},
			wantAfter: llmconsent.Record{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := requestcache.WithScope(context.Background())
			repo := usermocks.NewMockRepository(t)
			first := &user.User{ID: "user-1"}
			if tc.wantFirst.At != nil {
				first = &user.User{ID: "user-1", LLMConsentAt: tc.wantFirst.At, LLMConsentVersion: tc.wantFirst.Version}
			}
			after := &user.User{ID: "user-1"}
			if tc.wantAfter.At != nil {
				after = &user.User{ID: "user-1", LLMConsentAt: tc.wantAfter.At, LLMConsentVersion: tc.wantAfter.Version}
			}
			repo.EXPECT().FetchByID(mock.Anything, "user-1").Return(first, nil).Once()
			repo.EXPECT().FetchByID(mock.Anything, "user-1").Return(after, nil).Once()
			writes := mocks.NewMockConsentStore(t)
			writes.EXPECT().Grant(mock.Anything, "user-1", llmconsent.Version, at).Return(nil).Maybe()
			writes.EXPECT().Clear(mock.Anything, "user-1").Return(nil).Maybe()
			store := llmconsent.NewAccountConsentStore(user.NewCachedRepository(repo), writes)

			got, err := store.Fetch(ctx, "user-1")
			require.NoError(t, err)
			assert.Equal(t, tc.wantFirst, got)

			require.NoError(t, tc.write(store, ctx))

			got, err = store.Fetch(ctx, "user-1")
			require.NoError(t, err)
			assert.Equal(t, tc.wantAfter, got)
		})
	}
}
