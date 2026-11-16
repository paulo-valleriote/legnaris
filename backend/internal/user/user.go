package user

import "time"

type User struct {
	ID                  string     `bson:"_id,omitempty" json:"id"`
	Email               string     `bson:"email" json:"email"`
	Username            string     `bson:"username" json:"username"`
	PasswordHash        string     `bson:"password_hash" json:"-"`
	EmailVerifiedAt     *time.Time `bson:"email_verified_at,omitempty" json:"email_verified_at,omitempty"`
	Plan                string     `bson:"plan,omitempty" json:"-"`
	PlanVersion         int64      `bson:"plan_version,omitempty" json:"-"`
	LLMConsentAt        *time.Time `bson:"llm_consent_at,omitempty" json:"-"`
	LLMConsentVersion   string     `bson:"llm_consent_version,omitempty" json:"-"`
	CreatedAt           time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `bson:"updated_at" json:"updated_at"`
	Status              string     `bson:"status,omitempty" json:"-"`
	DeletingAt          *time.Time `bson:"deleting_at,omitempty" json:"-"`
	DeletionAttempts    int        `bson:"deletion_attempts,omitempty" json:"-"`
	DeletionNextAttempt *time.Time `bson:"deletion_next_attempt_at,omitempty" json:"-"`
	DeletionLockedUntil *time.Time `bson:"deletion_locked_until,omitempty" json:"-"`
	DeletionLastError   string     `bson:"deletion_last_error,omitempty" json:"-"`
}

func (u *User) IsDeleting() bool {
	return u.Status == StatusDeleting || u.Status == StatusDeletionDead
}

type UserDTO struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Username      string    `json:"username"`
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
