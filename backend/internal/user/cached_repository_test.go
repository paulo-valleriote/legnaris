package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/user"
	"legnaris/backend/internal/user/mocks"
	"legnaris/backend/shared/requestcache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errDatabase = errors.New("database unavailable")

func TestCachedRepository_FetchByID(t *testing.T) {
	stored := &user.User{ID: "u1", Plan: "free"}

	tests := []struct {
		name      string
		scoped    bool
		reads     []string
		fetchErr  error
		wantLoads int
		wantErr   error
	}{
		{name: "Edge: without a request scope every read reaches the repository", scoped: false, reads: []string{"u1", "u1"}, wantLoads: 2},
		{name: "Happy path: a scoped account is read once per request", scoped: true, reads: []string{"u1", "u1", "u1"}, wantLoads: 1},
		{name: "Edge: distinct accounts are read separately", scoped: true, reads: []string{"u1", "u2", "u1"}, wantLoads: 2},
		{name: "Error: a missing account is not memoized", scoped: true, reads: []string{"u1", "u1"}, fetchErr: user.ErrUserNotFound, wantLoads: 2, wantErr: user.ErrUserNotFound},
		{name: "Error: a database failure is not memoized", scoped: true, reads: []string{"u1", "u1"}, fetchErr: errDatabase, wantLoads: 2, wantErr: errDatabase},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().FetchByID(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, id string) (*user.User, error) {
					if tc.fetchErr != nil {
						return nil, tc.fetchErr
					}
					return &user.User{ID: id, Plan: stored.Plan}, nil
				},
			).Times(tc.wantLoads)

			ctx := context.Background()
			if tc.scoped {
				ctx = requestcache.WithScope(ctx)
			}
			cached := user.NewCachedRepository(repo)
			for _, id := range tc.reads {
				got, err := cached.FetchByID(ctx, id)
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr)
					assert.Nil(t, got)
					continue
				}
				require.NoError(t, err)
				assert.Equal(t, id, got.ID)
			}
		})
	}
}

func TestCachedRepository_WritesForgetTheAccount(t *testing.T) {
	at := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		write func(repo user.Repository, ctx context.Context) error
	}{
		{name: "Happy path: MarkDeleting forgets the account", write: func(repo user.Repository, ctx context.Context) error {
			return repo.MarkDeleting(ctx, "u1", at)
		}},
		{name: "Happy path: MarkEmailVerified forgets the account", write: func(repo user.Repository, ctx context.Context) error {
			return repo.MarkEmailVerified(ctx, "u1", at)
		}},
		{name: "Happy path: Create forgets the account", write: func(repo user.Repository, ctx context.Context) error {
			return repo.Create(ctx, &user.User{ID: "u1"})
		}},
		{name: "Happy path: ReplaceUnverifiedCredentials forgets the account", write: func(repo user.Repository, ctx context.Context) error {
			_, err := repo.ReplaceUnverifiedCredentials(ctx, "u1", "name", "hash", at)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().FetchByID(mock.Anything, "u1").Return(&user.User{ID: "u1"}, nil).Twice()
			repo.EXPECT().MarkDeleting(mock.Anything, "u1", at).Return(nil).Maybe()
			repo.EXPECT().MarkEmailVerified(mock.Anything, "u1", at).Return(nil).Maybe()
			repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Maybe()
			repo.EXPECT().ReplaceUnverifiedCredentials(mock.Anything, "u1", "name", "hash", at).Return(true, nil).Maybe()

			ctx := requestcache.WithScope(context.Background())
			cached := user.NewCachedRepository(repo)
			_, err := cached.FetchByID(ctx, "u1")
			require.NoError(t, err)
			_ = tc.write(cached, ctx)
			_, err = cached.FetchByID(ctx, "u1")
			require.NoError(t, err)
		})
	}
}

func TestForgetAccount(t *testing.T) {
	tests := []struct {
		name      string
		forgetID  string
		wantLoads int
	}{
		{name: "Happy path: forgetting the read account loads it again", forgetID: "u1", wantLoads: 2},
		{name: "Edge: forgetting another account keeps the memo", forgetID: "u2", wantLoads: 1},
		{name: "Edge: forgetting an empty id keeps the memo", forgetID: "", wantLoads: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().FetchByID(mock.Anything, "u1").Return(&user.User{ID: "u1"}, nil).Times(tc.wantLoads)

			ctx := requestcache.WithScope(context.Background())
			cached := user.NewCachedRepository(repo)
			_, err := cached.FetchByID(ctx, "u1")
			require.NoError(t, err)
			user.ForgetAccount(ctx, tc.forgetID)
			_, err = cached.FetchByID(ctx, "u1")
			require.NoError(t, err)
		})
	}
}
