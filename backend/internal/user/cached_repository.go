package user

import (
	"context"
	"time"

	"legnaris/backend/shared/requestcache"
)

// accountKey is the request memo key of one account document.
type accountKey struct {
	id string
}

// NewCachedRepository wraps repo so that, inside a request scope, each account
// is read from the database once. Every write through the wrapper forgets the
// account, so a request never reads a value it changed earlier. Outside a scope
// it behaves exactly like repo.
func NewCachedRepository(repo Repository) Repository {
	return cachedRepository{Repository: repo}
}

// ForgetAccount drops the memoized account id from the request scope, for a
// writer that changes the account document outside this repository.
func ForgetAccount(ctx context.Context, id string) {
	requestcache.Forget(ctx, accountKey{id: id})
}

type cachedRepository struct {
	Repository
}

// FetchByID returns the account, reading it once per request scope. A missing
// account is not memoized, so a later read in the same request sees it.
func (r cachedRepository) FetchByID(ctx context.Context, id string) (*User, error) {
	return requestcache.Memo(ctx, accountKey{id: id}, func(ctx context.Context) (*User, error) {
		return r.Repository.FetchByID(ctx, id)
	})
}

// Create stores the account and forgets any memoized read of its id.
func (r cachedRepository) Create(ctx context.Context, user *User) error {
	err := r.Repository.Create(ctx, user)
	ForgetAccount(ctx, user.ID)
	return err
}

// MarkEmailVerified stores the verification instant and forgets the account.
func (r cachedRepository) MarkEmailVerified(ctx context.Context, id string, at time.Time) error {
	err := r.Repository.MarkEmailVerified(ctx, id, at)
	ForgetAccount(ctx, id)
	return err
}

// MarkDeleting moves the account to deleting and forgets it, so the gate of the
// same request sees the new status.
func (r cachedRepository) MarkDeleting(ctx context.Context, id string, at time.Time) error {
	err := r.Repository.MarkDeleting(ctx, id, at)
	ForgetAccount(ctx, id)
	return err
}

// ReplaceUnverifiedCredentials swaps the credentials and forgets the account.
func (r cachedRepository) ReplaceUnverifiedCredentials(ctx context.Context, id, username, passwordHash string, at time.Time) (bool, error) {
	replaced, err := r.Repository.ReplaceUnverifiedCredentials(ctx, id, username, passwordHash, at)
	ForgetAccount(ctx, id)
	return replaced, err
}
