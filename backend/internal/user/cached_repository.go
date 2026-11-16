package user

import (
	"context"
	"time"

	"legnaris/backend/shared/requestcache"
)

type accountKey struct {
	id string
}

func NewCachedRepository(repo Repository) Repository {
	return cachedRepository{Repository: repo}
}

func ForgetAccount(ctx context.Context, id string) {
	requestcache.Forget(ctx, accountKey{id: id})
}

type cachedRepository struct {
	Repository
}

func (r cachedRepository) FetchByID(ctx context.Context, id string) (*User, error) {
	return requestcache.Memo(ctx, accountKey{id: id}, func(ctx context.Context) (*User, error) {
		return r.Repository.FetchByID(ctx, id)
	})
}

func (r cachedRepository) Create(ctx context.Context, user *User) error {
	err := r.Repository.Create(ctx, user)
	ForgetAccount(ctx, user.ID)
	return err
}

func (r cachedRepository) MarkEmailVerified(ctx context.Context, id string, at time.Time) error {
	err := r.Repository.MarkEmailVerified(ctx, id, at)
	ForgetAccount(ctx, id)
	return err
}

func (r cachedRepository) MarkDeleting(ctx context.Context, id string, at time.Time) error {
	err := r.Repository.MarkDeleting(ctx, id, at)
	ForgetAccount(ctx, id)
	return err
}

func (r cachedRepository) ReplaceUnverifiedCredentials(ctx context.Context, id, username, passwordHash string, at time.Time) (bool, error) {
	replaced, err := r.Repository.ReplaceUnverifiedCredentials(ctx, id, username, passwordHash, at)
	ForgetAccount(ctx, id)
	return replaced, err
}
