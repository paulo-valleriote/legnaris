package user

import "time"

type User struct {
	ID           string    `bson:"_id,omitempty" json:"id"`
	Email        string    `bson:"email" json:"email"`
	Username     string    `bson:"username" json:"username"`
	PasswordHash string    `bson:"password_hash" json:"-"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}

// UserDTO is the wire shape of a user. It deliberately has no password field:
// the hash is never part of an API response.
type UserDTO struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) ToDTO() UserDTO {
	return UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		CreatedAt: u.CreatedAt,
	}
}
