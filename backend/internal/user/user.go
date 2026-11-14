package user

import "time"

type User struct {
	ID           string `bson:"_id,omitempty" json:"id"`
	Email        string `bson:"email" json:"email"`
	Username     string `bson:"username" json:"username"`
	PasswordHash string `bson:"password_hash" json:"-"`
	// EmailVerifiedAt is absent until verification happens; it is never
	// backfilled and never exposed on the wire DTO (PRQ-12, PRQ-14, PRQ-15).
	// The derived boolean on the DTO is the only representation of the state
	// (TFA-08).
	EmailVerifiedAt *time.Time `bson:"email_verified_at,omitempty" json:"email_verified_at,omitempty"`
	// Plan is the stored subscription plan identifier. A missing value reads as
	// the Free plan (see plan.Lookup) and is never written by F1a.
	Plan      string    `bson:"plan,omitempty" json:"-"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
	// Status is empty for an active account. The deletion fields below are
	// written only by the account deletion flow (LGN-190).
	Status              string     `bson:"status,omitempty" json:"-"`
	DeletingAt          *time.Time `bson:"deleting_at,omitempty" json:"-"`
	DeletionAttempts    int        `bson:"deletion_attempts,omitempty" json:"-"`
	DeletionNextAttempt *time.Time `bson:"deletion_next_attempt_at,omitempty" json:"-"`
	DeletionLockedUntil *time.Time `bson:"deletion_locked_until,omitempty" json:"-"`
	DeletionLastError   string     `bson:"deletion_last_error,omitempty" json:"-"`
}

// IsDeleting reports whether the account is being deleted or its deletion gave
// up. Either state blocks every authenticated operation and every sign-in.
func (u *User) IsDeleting() bool {
	return u.Status == StatusDeleting || u.Status == StatusDeletionDead
}

// UserDTO is the wire shape of a user. It deliberately has no password field:
// the hash is never part of an API response.
type UserDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	// EmailVerified reports whether the account's address was ever verified.
	// The stored instant stays internal: only this derived state reaches the
	// wire (TFA-08, TFA-09).
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

func (u *User) ToDTO() UserDTO {
	return UserDTO{
		ID:            u.ID,
		Email:         u.Email,
		Username:      u.Username,
		EmailVerified: u.EmailVerifiedAt != nil,
		CreatedAt:     u.CreatedAt,
	}
}
