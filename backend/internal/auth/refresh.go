package auth

import (
	"time"
)

// RefreshToken is a persisted, single-use refresh credential. The raw token is
// never stored: only its SHA-256 hash. Tokens chained by rotation share a
// family_id so reuse of a rotated token can revoke the whole family at once.
type RefreshToken struct {
	ID         string    `bson:"_id,omitempty"`
	TokenHash  string    `bson:"token_hash"`
	UserID     string    `bson:"user_id"`
	FamilyID   string    `bson:"family_id"`
	ExpiresAt  time.Time `bson:"expires_at"`
	Revoked    bool      `bson:"revoked"`
	ReplacedBy string    `bson:"replaced_by,omitempty"`
	CreatedAt  time.Time `bson:"created_at"`
}
