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

// MsgAccountDeleting is the refusal sent to a signed-in request whose account is
// being deleted. It is the same for every route, so a client can drop its session.
const MsgAccountDeleting = "account deletion is in progress"

// DeleteAccountCommand carries the signed-in account and the deletion
// confirmation it presents: the challenge RequestAccountDeletion issued and the
// code that reached the account's address (LGN-190).
type DeleteAccountCommand struct {
	UserID      string
	ChallengeID string
	Code        string
}

// RequestAccountDeletion starts the deletion confirmation: it sends a one-time
// code to the account's verified address and returns the challenge the client
// must present to DeleteAccount. An account already deleting is refused like
// every other authenticated route, and an unknown one is unauthorized.
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

// DeleteAccount records the account deletion request once the confirmation code
// is accepted. The challenge must belong to the signed-in account and carry the
// deletion purpose; any other row is refused with the uniform code failure and
// is not consumed. The account is only marked deleting; the sweeper removes the
// data later. Repeating the request with a fresh code keeps the first instant.
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

// EnsureActive refuses a request whose account is being deleted. It is the gate
// every authenticated route runs after the token check, so a deleting account
// loses access on its next request, not when its token expires.
func (s *ServiceImpl) EnsureActive(ctx context.Context, userID string) error {
	_, err := s.activeAccount(ctx, userID)
	return err
}

// activeAccount loads the account and refuses one that is deleting or gone.
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
