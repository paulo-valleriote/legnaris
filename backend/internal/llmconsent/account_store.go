package llmconsent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/user"
)

// AccountReader reads an account document. The user repository, wrapped by
// user.NewCachedRepository, satisfies it, so the consent read shares the account
// read of the same request.
type AccountReader interface {
	FetchByID(ctx context.Context, id string) (*user.User, error)
}

// accountConsentStore reads the consent fields from the account document and
// writes them through the writes store. The write path also forgets the account
// in the request scope, so a read after a grant or a revoke sees the change.
type accountConsentStore struct {
	accounts AccountReader
	writes   ConsentStore
}

// NewAccountConsentStore returns the consent store that reads the account
// document through accounts and writes through writes.
func NewAccountConsentStore(accounts AccountReader, writes ConsentStore) ConsentStore {
	return accountConsentStore{accounts: accounts, writes: writes}
}

// Fetch returns the consent stored on the account, or ErrUserNotFound when the
// account is absent.
func (s accountConsentStore) Fetch(ctx context.Context, userID string) (Record, error) {
	account, err := s.accounts.FetchByID(ctx, userID)
	if errors.Is(err, user.ErrUserNotFound) {
		return Record{}, ErrUserNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("fetch llm consent: %w", err)
	}
	return Record{At: account.LLMConsentAt, Version: account.LLMConsentVersion}, nil
}

// Grant stores the consent through the write store and forgets the account.
func (s accountConsentStore) Grant(ctx context.Context, userID, version string, at time.Time) error {
	err := s.writes.Grant(ctx, userID, version, at)
	user.ForgetAccount(ctx, userID)
	return err
}

// Clear removes the consent through the write store and forgets the account.
func (s accountConsentStore) Clear(ctx context.Context, userID string) error {
	err := s.writes.Clear(ctx, userID)
	user.ForgetAccount(ctx, userID)
	return err
}
