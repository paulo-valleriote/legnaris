package auth

import (
	"context"
	"errors"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

func (s *ServiceImpl) Profile(ctx context.Context, userID string) (*user.UserDTO, error) {
	if err := idgen.Validate(userID); err != nil {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
	}

	account, err := s.users.FetchByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil, apperr.NewError(apperr.CodeUnauthorized, MsgAccountUnavailable, err)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	profile := account.ToDTO()
	return &profile, nil
}
