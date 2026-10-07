package auth

import "errors"

// User-facing message constants, exported so the controller boundary reuses the
// exact wording. They are plain strings, never error causes.
const (
	MsgInvalidCredentials   = "invalid credentials"
	MsgEmailAlreadyExists   = "email already registered"
	MsgInvalidEmail         = "email is invalid"
	MsgUsernameRequired     = "username is required"
	MsgUsernameTooLong      = "username must be at most 50 characters"
	MsgUsernameInvalid      = "username may only contain letters, numbers, underscore, and hyphen"
	MsgPasswordTooShort     = "password must be at least 8 bytes"
	MsgPasswordTooLong      = "password must be at most 72 bytes"
	MsgRefreshTokenRequired = "refresh token is required"
	MsgInvalidRefreshToken  = "invalid refresh token"
	MsgInternal             = "internal error"
)

// Lower-layer sentinels for the auth components (hasher, token manager,
// generator). The service translates these into apperr codes at the boundary.
var (
	ErrEmptyJWTSecret    = errors.New("JWT secret is not configured")
	ErrInvalidBcryptCost = errors.New("bcrypt cost is out of range")
	ErrPasswordMismatch  = errors.New("password does not match")
	ErrExpiredToken      = errors.New("access token expired")
	ErrInvalidToken      = errors.New("access token is invalid")
)
