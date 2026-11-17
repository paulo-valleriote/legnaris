package campaign

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/shared/apperr"
)

type TransactionRunner interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type MutateFunc func(txCtx context.Context, c *Campaign) error

func Mutate(ctx context.Context, tx TransactionRunner, repo Repository, campaignID string, now time.Time, fn MutateFunc) (Campaign, error) {
	var out Campaign
	err := tx.WithTransaction(ctx, func(txCtx context.Context) error {
		loaded, err := repo.FetchByID(txCtx, campaignID)
		if errors.Is(err, ErrNotFound) {
			return notFound()
		}
		if err != nil {
			return internalError(MsgCampaignRead, err)
		}
		if !loaded.Active() {
			return notFound()
		}
		expected := loaded.Version
		if err := fn(txCtx, loaded); err != nil {
			return err
		}
		loaded.recount()
		loaded.Version = expected + 1
		loaded.UpdatedAt = now
		if err := repo.Save(txCtx, loaded, expected); err != nil {
			return saveError(err)
		}
		out = *loaded
		return nil
	})
	if err != nil {
		return Campaign{}, unwrapTxError(err)
	}
	return out, nil
}

func JoinPlayer(ctx context.Context, tx TransactionRunner, repo Repository, campaignID, userID string, now time.Time) (Campaign, error) {
	return Mutate(ctx, tx, repo, campaignID, now, func(_ context.Context, c *Campaign) error {
		if _, ok := c.MemberOf(userID); ok {
			return apperr.NewError(apperr.CodeConflict, MsgAlreadyMember, nil)
		}
		if c.PlayerCount >= MaxPlayers {
			return full(MsgPlayersFull)
		}
		c.Members = append(c.Members, Member{UserID: userID, Role: RolePlayer, JoinedAt: now})
		return nil
	})
}

func saveError(err error) error {
	if errors.Is(err, ErrVersionConflict) {
		return apperr.NewError(apperr.CodeConflict, MsgConcurrentChange, err)
	}
	return internalError(MsgCampaignWrite, err)
}

func internalError(message string, err error) error {
	return apperr.NewError(apperr.CodeInternal, message, err)
}

func full(message string) *apperr.Error {
	return apperr.NewError(apperr.CodeCampaignFull, message, nil)
}

func unwrapTxError(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return internalError(MsgTransactionFailed, err)
}
