package auth

import "time"

// Purpose is the server-side reason a challenge was issued. It is resolved from
// the stored record and never accepted from the client (OTP-01, OTP-09).
type Purpose string

const (
	// PurposeVerifyEmail marks a challenge issued for a freshly registered
	// account; PurposeLoginOTP marks one issued for a sign-in.
	PurposeVerifyEmail Purpose = "verify_email"
	PurposeLoginOTP    Purpose = "login_otp"
)

// Challenge is the durable record of one pending code step (data-model.md §1):
// the account it belongs to, the code's keyed digest, the movable deadline, the
// absolute ceiling, the consumed marker, the attempt count and the identity of
// the send intention that carries the code. At most one record is active per
// account and purpose.
type Challenge struct {
	ID           string     `bson:"_id"`
	UserID       string     `bson:"user_id"`
	Purpose      Purpose    `bson:"purpose"`
	CodeHash     string     `bson:"code_hash"`
	ExpiresAt    time.Time  `bson:"expires_at"`
	CeilingAt    time.Time  `bson:"ceiling_at"`
	ConsumedAt   *time.Time `bson:"consumed_at"`
	Attempts     int        `bson:"attempts"`
	CreatedAt    time.Time  `bson:"created_at"`
	LastSentAt   time.Time  `bson:"last_sent_at"`
	LastIntentID string     `bson:"last_intent_id"`
}

// Usable reports whether the challenge can still be consumed at now: it must be
// unconsumed, its deadline must be in the future (exactly at the expiry instant
// it is already expired), and its attempt count must be below maxAttempts
// (OTP-04, OTP-05, OTP-06). Expiry is decided against the injected clock, never
// by the record's absence.
func (c Challenge) Usable(now time.Time, maxAttempts int) bool {
	if c.ConsumedAt != nil {
		return false
	}
	if !now.Before(c.ExpiresAt) {
		return false
	}
	return c.Attempts < maxAttempts
}

// FailureResult classifies why a stored challenge is not usable, for metrics and
// logs only: it never changes the HTTP response, which is uniform for every
// failure (OTP-08, OTP-16). A usable challenge answers ResultInvalid rather than
// being reported as one of the diagnostic states.
func (c Challenge) FailureResult(now time.Time, maxAttempts int) string {
	switch {
	case c.ConsumedAt != nil:
		return ResultConsumed
	case !now.Before(c.ExpiresAt):
		return ResultExpired
	case c.Attempts >= maxAttempts:
		return ResultExhausted
	default:
		return ResultInvalid
	}
}
