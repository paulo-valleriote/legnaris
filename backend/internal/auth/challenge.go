package auth

import "time"

type Purpose string

const (
	PurposeVerifyEmail   Purpose = "verify_email"
	PurposeAccountExists Purpose = "account_exists"
	PurposeDeleteAccount Purpose = "delete_account"
)

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

func (c Challenge) Usable(now time.Time, maxAttempts int) bool {
	if c.ConsumedAt != nil {
		return false
	}
	if !now.Before(c.ExpiresAt) {
		return false
	}
	return c.Attempts < maxAttempts
}

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
