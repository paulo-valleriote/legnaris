package auth

import "errors"

// User-facing message constants, exported so the controller boundary reuses the
// exact wording. They are plain strings, never error causes.
const (
	MsgInvalidCredentials   = "invalid credentials"
	MsgRegistrationConflict = "these details cannot be used to register"
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
	// MsgAccountUnavailable is the identity read's answer for a credential that
	// names no usable account: the account was removed, or never existed. It is
	// deliberately the same unauthorized outcome as an invalid credential, so
	// the two are indistinguishable (TFA-09, TFA-10).
	MsgAccountUnavailable = "account is unavailable"

	// Challenge copy. MsgOTPInvalid is the one uniform message every
	// verification failure carries, so unknown, expired, consumed, wrong and
	// exhausted are indistinguishable (OTP-08).
	MsgOTPInvalid          = "invalid or expired code"
	MsgEmailUnavailable    = "email delivery is temporarily unavailable; retry later"
	MsgChallengeIDRequired = "challenge_id is required"
	MsgCodeRequired        = "code is required"
	// MsgRateLimited mirrors the rate-limit guard's copy for the durable
	// per-account bound the service enforces itself (OTP-14).
	MsgRateLimited = "too many attempts; retry later"
)

// Lower-layer sentinels for the auth components (hasher, token manager,
// generator). The service translates these into apperr codes at the boundary.
var (
	ErrEmptyJWTSecret    = errors.New("JWT secret is not configured")
	ErrInvalidBcryptCost = errors.New("bcrypt cost is out of range")
	ErrPasswordMismatch  = errors.New("password does not match")
	ErrExpiredToken      = errors.New("access token expired")
	ErrInvalidToken      = errors.New("access token is invalid")
	ErrCSRFTokenInvalid  = errors.New("csrf token is invalid")
)
