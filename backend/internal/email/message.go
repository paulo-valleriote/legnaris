package email

import "time"

type Purpose string

const (
	PurposeVerifyEmail   Purpose = "verify_email"
	PurposeLoginOTP      Purpose = "login_otp"
	PurposeAccountExists Purpose = "account_exists"
	PurposeDeleteAccount Purpose = "delete_account"
)

func (p Purpose) CarriesCode() bool {
	return p != PurposeAccountExists
}

const TypeOTP = "email.otp"

type Message struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	To        string    `json:"to"`
	Template  string    `json:"template"`
	Data      Data      `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	Attempt   int       `json:"attempt"`
}

type Data struct {
	Code             string  `json:"code"`
	ExpiresInMinutes int     `json:"expires_in_minutes"`
	Purpose          Purpose `json:"purpose"`
}
