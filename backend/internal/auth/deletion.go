package auth

import (
	"context"
	"errors"
	"fmt"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

const MsgAccountDeleting = "account deletion is in progress"

type DeleteAccountCommand struct {
	UserID      string
	ChallengeID string
	Code        string
}

func (s *ServiceImpl) RequestAccountDeletion(ctx context.Context, userID string) (*ChallengeIssued, error) {
	if err := idgen.Validate(userID); err != nil {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
	}

	account, err := s.activeAccount(ctx, userID)
	if err != nil {
		return nil, err
	}

	issued, err := s.startChallenge(ctx, account.ID, account.Email, PurposeDeleteAccount)
	if err != nil {
		return nil, err
	}

	s.log.Info(ctx, "account deletion challenge started", logger.Attribute{Key: attrUserID, Value: userID})
	return issued, nil
}

func (s *ServiceImpl) DeleteAccount(ctx context.Context, cmd DeleteAccountCommand) error {
	if err := idgen.Validate(cmd.UserID); err != nil {
		return apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
	}

	if _, err := s.consumeChallenge(ctx, cmd.ChallengeID, cmd.Code, func(row *Challenge) bool {
		return row.Purpose == PurposeDeleteAccount && row.UserID == cmd.UserID
	}); err != nil {
		return err
	}

	if err := s.users.MarkDeleting(ctx, cmd.UserID, s.clock.Now()); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
		}
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	s.challenge.Observer.ChallengeVerified(ResultVerified)
	s.log.Info(ctx, "account deletion requested", logger.Attribute{Key: attrUserID, Value: cmd.UserID})
	return nil
}

func (s *ServiceImpl) EnsureActive(ctx context.Context, userID string) error {
	_, err := s.activeAccount(ctx, userID)
	return err
}

func (s *ServiceImpl) activeAccount(ctx context.Context, userID string) (*user.User, error) {
	account, err := s.users.FetchByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil, apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, fmt.Errorf("load account: %w", err))
	}
	if account.IsDeleting() {
		return nil, apperr.NewError(apperr.CodeForbidden, MsgAccountDeleting, nil)
	}
	return account, nil
}
