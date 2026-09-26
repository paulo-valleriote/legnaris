package user

import "time"

type User struct {
	ID        string    `bson:"_id,omitempty" json:"id"`
	Email     string    `bson:"email" json:"email"`
	Username  string    `bson:"username" json:"username"`
	Password  string    `bson:"password" json:"-"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
