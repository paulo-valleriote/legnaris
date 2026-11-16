package auth

import (
	"time"
)

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
