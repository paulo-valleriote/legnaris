package auth

const (
	ResultVerified  = "verified"
	ResultInvalid   = "invalid"
	ResultExpired   = "expired"
	ResultConsumed  = "consumed"
	ResultExhausted = "exhausted"
	ResultUnknown   = "unknown"

	ResendSent       = "sent"
	ResendSuppressed = "suppressed"
	ResendCapped     = "capped"
	ResendReserved   = "reserved"
)

type ChallengeObserver interface {
	ChallengeIssued(purpose Purpose)
	ChallengeVerified(result string)
	ResendRequested(result string)
	SendCapBlocked()
}

type nopChallengeObserver struct{}

func (nopChallengeObserver) ChallengeIssued(Purpose) {}

func (nopChallengeObserver) ChallengeVerified(string) {}

func (nopChallengeObserver) ResendRequested(string) {}

func (nopChallengeObserver) SendCapBlocked() {}

func NewNopChallengeObserver() ChallengeObserver {
	return nopChallengeObserver{}
}
