package llmconsent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/user"
)

type AccountReader interface {
	FetchByID(ctx context.Context, id string) (*user.User, error)
}

type accountConsentStore struct {
	accounts AccountReader
	writes   ConsentStore
}

func NewAccountConsentStore(accounts AccountReader, writes ConsentStore) ConsentStore {
	return accountConsentStore{accounts: accounts, writes: writes}
}

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

func (s accountConsentStore) Grant(ctx context.Context, userID, version string, at time.Time) error {
	err := s.writes.Grant(ctx, userID, version, at)
	user.ForgetAccount(ctx, userID)
	return err
}

func (s accountConsentStore) Clear(ctx context.Context, userID string) error {
	err := s.writes.Clear(ctx, userID)
	user.ForgetAccount(ctx, userID)
	return err
}
