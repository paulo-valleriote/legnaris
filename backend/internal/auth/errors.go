package auth

import "errors"

const (
	MsgInvalidCredentials   = "invalid credentials"
	MsgUsernameTaken        = "Username is already taken."
	MsgRegistrationConflict = "Registration failed. Check your details and try again, or sign in if you already have an account."
	MsgDisposableEmail      = "use a permanent email address; this one cannot receive the verification code"
	MsgInvalidEmail         = "email is invalid"
	MsgUsernameRequired     = "username is required"
	MsgUsernameTooLong      = "username must be at most 50 characters"
	MsgUsernameInvalid      = "username may only contain letters, numbers, underscore, and hyphen"
	MsgPasswordTooShort     = "password must be at least 8 bytes"
	MsgPasswordTooLong      = "password must be at most 72 bytes"
	MsgRefreshTokenRequired = "refresh token is required"
	MsgInvalidRefreshToken  = "invalid refresh token"
	MsgCSRFInvalid          = "csrf token is invalid"
	MsgInternal             = "internal error"
	MsgAccountUnavailable   = "account is unavailable"

	MsgOTPInvalid          = "invalid or expired code"
	MsgEmailUnavailable    = "email delivery is temporarily unavailable; retry later"
	MsgChallengeIDRequired = "challenge_id is required"
	MsgCodeRequired        = "code is required"
	MsgRateLimited         = "too many attempts; retry later"
)

var (
	ErrEmptyJWTSecret    = errors.New("JWT secret is not configured")
	ErrInvalidBcryptCost = errors.New("bcrypt cost is out of range")
	ErrPasswordMismatch  = errors.New("password does not match")
	ErrExpiredToken      = errors.New("access token expired")
	ErrInvalidToken      = errors.New("access token is invalid")
	ErrCSRFTokenInvalid  = errors.New("csrf token is invalid")
)
