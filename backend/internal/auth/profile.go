package auth

import (
	"context"
	"errors"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

// Profile reads the signed-in account and reports its identity, its canonical
// address, its display name and whether its address was ever verified. The
// verification state is a derived boolean: the stored instant never reaches
// the caller (TFA-08, TFA-09).
//
// A credential that names an account which no longer exists answers the same
// unauthorized outcome as an invalid credential, never a 404, so the two are
// indistinguishable (TFA-10).
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
