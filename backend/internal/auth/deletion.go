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

// DeleteAccount records the account deletion request. It only marks the account
// as deleting; the sweeper removes the data later. Repeating the request is a
// no-op that keeps the first deletion instant.
func (s *ServiceImpl) DeleteAccount(ctx context.Context, userID string) error {
	if err := idgen.Validate(userID); err != nil {
		return apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
	}

	if err := s.users.MarkDeleting(ctx, userID, s.clock.Now()); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
		}
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	s.log.Info(ctx, "account deletion requested", logger.Attribute{Key: attrUserID, Value: userID})
	return nil
}

// EnsureActive refuses a request whose account is being deleted. It is the gate
// every authenticated route runs after the token check, so a deleting account
// loses access on its next request, not when its token expires.
func (s *ServiceImpl) EnsureActive(ctx context.Context, userID string) error {
	account, err := s.users.FetchByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
		}
		return apperr.NewError(apperr.CodeInternal, MsgInternal, fmt.Errorf("load account: %w", err))
	}
	if account.IsDeleting() {
		return apperr.NewError(apperr.CodeForbidden, MsgAccountDeleting, nil)
	}
	return nil
}
