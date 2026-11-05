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
	// ExistsNoticeAt is when the owner was last told that someone tried to
	// register with the address; it paces those notices so a stranger cannot
	// flood a mailbox.
	ExistsNoticeAt *time.Time `bson:"exists_notice_at,omitempty" json:"-"`
	CreatedAt      time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `bson:"updated_at" json:"updated_at"`
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
